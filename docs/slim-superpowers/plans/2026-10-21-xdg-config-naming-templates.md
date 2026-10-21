# XDG Configuration & Remote Naming Templates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable XDG-compliant configuration (`config.yaml`) and template-based remote naming (`{user:-root}_{alias}`) for `ssh2rclone`.

**Architecture:** 
Introduce `internal/model/config.go` and `internal/config/app_config.go` to parse `$XDG_CONFIG_HOME/ssh2rclone/config.yaml` using `gopkg.in/yaml.v3`. Build a token-based template formatter in `internal/services/naming.go` supporting inline defaults (`{var:-default}`) and sanitization. Update `BuildRemotes` to deduplicate generated names, and expose `-config` and `-name-template` in `main.go`.

**Tech Stack:** Go 1.23+, `gopkg.in/yaml.v3` for YAML decoding, Go standard library (`os`, `path/filepath`, `regexp`, `strings`).

## Global Constraints
- Preserve strict isolation: do not touch anything outside `# --- BEGIN ssh2rclone ...` and `# --- END ssh2rclone ...`.
- Backward compatibility: when no config file and no `--name-template` flag exist, the remote name MUST default to `{alias}`.
- Skip internal duplicate remote names, logging a warning to `os.Stderr`.
- Skip external remote conflicts with existing logic.

---

### Task 1: Configuration Models & XDG Loader

**Files:**
- Modify: `go.mod` (add `gopkg.in/yaml.v3`)
- Create: `internal/model/config.go`
- Modify: `internal/model/models.go:27-34`
- Create: `internal/config/app_config.go`
- Test: `internal/config/app_config_test.go`

**Interfaces:**
- Produces:
  - `model.AppConfig` struct with `Version int` and `Naming model.NamingConfig`
  - `model.DefaultAppConfig() model.AppConfig`
  - `config.ResolveAppConfigPath(cliPath string) (string, error)`
  - `config.LoadAppConfig(resolvedPath string, isCustom bool) (model.AppConfig, error)`

- [x] **Step 1: Add `gopkg.in/yaml.v3` dependency**

Run: `go get gopkg.in/yaml.v3`

- [x] **Step 2: Update `internal/model/models.go` with CLI flag fields**

In `internal/model/models.go`, add `ConfigPath` and `NameTemplate` to `SyncOptions`:
```go
type SyncOptions struct {
	SSHConfigPath    string
	RcloneConfigPath string
	ConfigPath       string
	NameTemplate     string
	DryRun           bool
	AllSSH           bool
	NoBackup         bool
	Verbose          bool
}
```

- [x] **Step 3: Create `internal/model/config.go`**

```go
package model

// AppConfig represents the root configuration file for ssh2rclone.
type AppConfig struct {
	Version int          `yaml:"version"`
	Naming  NamingConfig `yaml:"naming"`
}

// NamingConfig defines naming rules for generated rclone remotes.
type NamingConfig struct {
	Template string `yaml:"template"`
}

// DefaultAppConfig returns the fallback configuration when no config file is present.
func DefaultAppConfig() AppConfig {
	return AppConfig{
		Version: 1,
		Naming: NamingConfig{
			Template: "{alias}",
		},
	}
}
```

- [x] **Step 4: Write failing unit test for `app_config.go`**

Create `internal/config/app_config_test.go`:
```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveAppConfigPath(t *testing.T) {
	// Custom CLI path
	path, err := ResolveAppConfigPath("/tmp/custom.yaml")
	if err != nil || path != "/tmp/custom.yaml" {
		t.Fatalf("expected /tmp/custom.yaml, got %s (err: %v)", path, err)
	}

	// XDG_CONFIG_HOME
	t.Setenv("XDG_CONFIG_HOME", "/custom/xdg")
	path, err = ResolveAppConfigPath("")
	if err != nil || path != "/custom/xdg/ssh2rclone/config.yaml" {
		t.Fatalf("expected /custom/xdg/ssh2rclone/config.yaml, got %s (err: %v)", path, err)
	}
}

func TestLoadAppConfig_DefaultWhenMissing(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "does-not-exist.yaml")

	cfg, err := LoadAppConfig(nonExistent, false)
	if err != nil {
		t.Fatalf("unexpected error loading missing default config: %v", err)
	}
	if cfg.Naming.Template != "{alias}" {
		t.Fatalf("expected default template {alias}, got %s", cfg.Naming.Template)
	}
}

func TestLoadAppConfig_ErrorWhenCustomMissing(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "missing-custom.yaml")

	_, err := LoadAppConfig(nonExistent, true)
	if err == nil {
		t.Fatalf("expected error when custom config file is missing, got nil")
	}
}

func TestLoadAppConfig_ValidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "config.yaml")
	content := []byte("version: 1\nnaming:\n  template: \"{user:-root}_{alias}\"\n")
	if err := os.WriteFile(yamlFile, content, 0o600); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	cfg, err := LoadAppConfig(yamlFile, false)
	if err != nil {
		t.Fatalf("failed to load valid config: %v", err)
	}
	if cfg.Naming.Template != "{user:-root}_{alias}" {
		t.Fatalf("expected {user:-root}_{alias}, got %s", cfg.Naming.Template)
	}
}
```

- [x] **Step 5: Run test to verify failure**

Run: `go test -v ./internal/config/... -run TestResolveAppConfigPath`
Expected: FAIL (functions not defined)

- [x] **Step 6: Implement `internal/config/app_config.go`**

```go
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"ssh2rclone/internal/model"
)

// ResolveAppConfigPath locates the application configuration file.
func ResolveAppConfigPath(cliPath string) (string, error) {
	if cliPath != "" {
		return ExpandHome(cliPath)
	}

	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "ssh2rclone", "config.yaml"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "ssh2rclone", "config.yaml"), nil
}

// LoadAppConfig reads and deserializes the YAML configuration.
func LoadAppConfig(path string, isCustom bool) (model.AppConfig, error) {
	defaultCfg := model.DefaultAppConfig()

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			if isCustom {
				return defaultCfg, fmt.Errorf("config file not found at %s", path)
			}
			return defaultCfg, nil
		}
		return defaultCfg, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return defaultCfg, fmt.Errorf("failed to read config file at %s: %w", path, err)
	}

	cfg := defaultCfg
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return defaultCfg, fmt.Errorf("failed to parse yaml config at %s: %w", path, err)
	}

	if cfg.Naming.Template == "" {
		cfg.Naming.Template = "{alias}"
	}

	return cfg, nil
}
```

- [x] **Step 7: Run test to verify it passes**

Run: `go test -v ./internal/config/...`
Expected: PASS

---

### Task 2: Naming Engine, Sanitization & Collision Resolution

**Files:**
- Create: `internal/services/naming.go`
- Test: `internal/services/naming_test.go`
- Modify: `internal/services/rclone_sync.go:29-49`
- Modify: `internal/services/rclone_sync_test.go`

**Interfaces:**
- Produces:
  - `services.FormatRemoteName(entry model.SSHEntry, tmpl string) string`
  - `services.SanitizeRemoteName(name string) string`
  - `services.BuildRemotes(entries []model.SSHEntry, tmpl string) []model.RcloneRemote`

- [x] **Step 1: Write failing tests for Naming Engine**

Create `internal/services/naming_test.go`:
```go
package services

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"ssh2rclone/internal/model"
)

func TestFormatRemoteName(t *testing.T) {
	entry := model.SSHEntry{
		Alias:    "myserver",
		HostName: "server.example.com",
		User:     "deploy",
		Port:     "2200",
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{"default alias", "{alias}", "myserver"},
		{"alternate name", "{name}", "myserver"},
		{"user and alias", "{user}_{alias}", "deploy_myserver"},
		{"username alias", "{username}_{alias}", "deploy_myserver"},
		{"host and port", "{host}:{port}", "server.example.com:2200"},
		{"hostname token", "{hostname}", "server.example.com"},
		{"fallback not triggered", "{user:-root}_{alias}", "deploy_myserver"},
		{"literal chars", "sftp-{alias}", "sftp-myserver"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := FormatRemoteName(entry, tt.template)
			if res != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, res)
			}
		})
	}
}

func TestFormatRemoteName_Fallbacks(t *testing.T) {
	entryWithoutUser := model.SSHEntry{
		Alias:    "srv",
		HostName: "srv.lan",
		User:     "",
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{"fallback triggered", "{user:-root}_{alias}", "root_srv"},
		{"empty without fallback", "{user}_{alias}", "_srv"},
		{"empty template fallback to alias", "", "srv"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := FormatRemoteName(entryWithoutUser, tt.template)
			if res != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, res)
			}
		})
	}
}

func TestSanitizeRemoteName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal-name", "normal-name"},
		{"spaces in name", "spaces-in-name"},
		{"brackets [bad]", "bad"},
		{"tab\tand\nnewline", "tab-and-newline"},
		{"---trim-edges---", "trim-edges"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			res := SanitizeRemoteName(tt.input)
			if res != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, res)
			}
		})
	}
}

func TestBuildRemotes_InternalDuplicateDetection(t *testing.T) {
	entries := []model.SSHEntry{
		{Alias: "srv1", HostName: "10.0.0.1"},
		{Alias: "srv2", HostName: "10.0.0.1"}, // Duplicate when template is {host}
	}

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	remotes := BuildRemotes(entries, "{host}")

	w.Close()
	os.Stderr = oldStderr
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if len(remotes) != 1 {
		t.Fatalf("expected 1 remote due to collision, got %d", len(remotes))
	}
	if remotes[0].Name != "10.0.0.1" {
		t.Fatalf("expected remote name 10.0.0.1, got %s", remotes[0].Name)
	}
	if !strings.Contains(output, "Warning: duplicate remote name") {
		t.Fatalf("expected duplicate warning in stderr, got: %s", output)
	}
}
```

- [x] **Step 2: Run test to verify failure**

Run: `go test -v ./internal/services/... -run TestFormatRemoteName`
Expected: FAIL (undefined functions)

- [x] **Step 3: Implement `internal/services/naming.go`**

```go
package services

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"ssh2rclone/internal/model"
)

var tokenRegex = regexp.MustCompile(`\{([a-zA-Z0-9_]+)(?::-([^}]*))?\}`)

// FormatRemoteName renders an SSHEntry into a remote name using the given template.
func FormatRemoteName(entry model.SSHEntry, tmpl string) string {
	if strings.TrimSpace(tmpl) == "" {
		tmpl = "{alias}"
	}

	rendered := tokenRegex.ReplaceAllStringFunc(tmpl, func(match string) string {
		submatches := tokenRegex.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		varName := strings.ToLower(submatches[1])
		fallback := ""
		if len(submatches) >= 3 {
			fallback = submatches[2]
		}

		var val string
		switch varName {
		case "alias", "name":
			val = entry.Alias
		case "user", "username":
			val = entry.User
		case "host", "hostname":
			val = entry.HostName
		case "port":
			val = entry.Port
			if val == "" {
				val = "22"
			}
		default:
			return match
		}

		if val != "" {
			return val
		}
		return fallback
	})

	sanitized := SanitizeRemoteName(rendered)
	if sanitized == "" {
		return entry.Alias
	}
	return sanitized
}

// SanitizeRemoteName removes invalid rclone INI characters and normalizes spaces.
func SanitizeRemoteName(name string) string {
	// Strip characters that break INI section parsing: [, ], newlines
	replacer := strings.NewReplacer("[", "", "]", "", "\r", "", "\n", "")
	cleaned := replacer.Replace(name)

	// Replace tabs and spaces with single dashes
	spaceRegex := regexp.MustCompile(`[\s\t]+`)
	cleaned = spaceRegex.ReplaceAllString(cleaned, "-")

	// Collapse multiple dashes
	dashRegex := regexp.MustCompile(`-+`)
	cleaned = dashRegex.ReplaceAllString(cleaned, "-")

	// Trim leading and trailing dashes
	return strings.Trim(cleaned, "-")
}
```

- [x] **Step 4: Update `BuildRemotes` in `internal/services/rclone_sync.go`**

Update `BuildRemotes` to take `tmpl string` and check internal duplicate names:
```go
// BuildRemotes converts SSHEntry models to RcloneRemote models using the given naming template.
func BuildRemotes(entries []model.SSHEntry, tmpl string) []model.RcloneRemote {
	var remotes []model.RcloneRemote
	seen := make(map[string]bool)

	for _, e := range entries {
		remoteName := FormatRemoteName(e, tmpl)
		if seen[remoteName] {
			fmt.Fprintf(os.Stderr, "Warning: duplicate remote name '%s' generated for SSH alias '%s', skipping\n", remoteName, e.Alias)
			continue
		}
		seen[remoteName] = true

		remote := model.RcloneRemote{
			Name: remoteName,
			Type: "sftp",
		}
		if e.IsComplex {
			remote.SSHCommand = fmt.Sprintf("ssh %s", e.Alias)
		} else {
			remote.Host = e.HostName
			remote.User = e.User
			remote.Port = e.Port
			remote.KeyFile = e.KeyFile
		}
		remotes = append(remotes, remote)
	}
	return remotes
}
```

- [x] **Step 5: Fix callers in `internal/services/rclone_sync_test.go` and `main.go`**

Pass `"{alias}"` or default template in existing test calls `BuildRemotes(entries, "{alias}")`.

- [x] **Step 6: Run tests to verify they pass**

Run: `go test -v ./internal/services/...`
Expected: PASS

---

### Task 3: CLI Flags & Pipeline Integration

**Files:**
- Modify: `main.go`
- Modify: `main_test.go`

**Interfaces:**
- Flags:
  - `-config` / `--config`: string
  - `-name-template` / `--name-template`: string

- [x] **Step 1: Write test in `main_test.go` for custom config and name-template flags**

Add test cases in `main_test.go`:
```go
func TestMain_CustomNameTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	sshConfig := filepath.Join(tmpDir, "ssh_config")
	rcloneConfig := filepath.Join(tmpDir, "rclone.conf")

	sshContent := "Host srv1\n    HostName 192.168.1.10\n    User deploy\n"
	_ = os.WriteFile(sshConfig, []byte(sshContent), 0o600)
	_ = os.WriteFile(rcloneConfig, []byte(""), 0o600)

	// Test passing --name-template "{user}_{alias}"
	output := runCLI(t, "--ssh-config", sshConfig, "--rclone-config", rcloneConfig, "--name-template", "{user}_{alias}")
	if !strings.Contains(output, "deploy_srv1") {
		t.Fatalf("expected deploy_srv1 in output, got: %s", output)
	}
}

func TestMain_ConfigYamlLoading(t *testing.T) {
	tmpDir := t.TempDir()
	sshConfig := filepath.Join(tmpDir, "ssh_config")
	rcloneConfig := filepath.Join(tmpDir, "rclone.conf")
	appConfig := filepath.Join(tmpDir, "config.yaml")

	sshContent := "Host srv1\n    HostName 192.168.1.10\n"
	appConfigContent := "version: 1\nnaming:\n  template: \"{user:-guest}_{alias}\"\n"

	_ = os.WriteFile(sshConfig, []byte(sshContent), 0o600)
	_ = os.WriteFile(rcloneConfig, []byte(""), 0o600)
	_ = os.WriteFile(appConfig, []byte(appConfigContent), 0o600)

	output := runCLI(t, "--ssh-config", sshConfig, "--rclone-config", rcloneConfig, "--config", appConfig)
	if !strings.Contains(output, "guest_srv1") {
		t.Fatalf("expected guest_srv1 in output, got: %s", output)
	}
}
```

- [x] **Step 2: Run test to verify failure**

Run: `go test -v -run TestMain_CustomNameTemplate .`
Expected: FAIL (flag not provided)

- [x] **Step 3: Update `main.go` to parse flags and wire configuration**

In `main.go`:
1. Register `-config` and `-name-template` in `flag.CommandLine`.
2. Resolve app config path:
   ```go
   resolvedAppConfigPath, err := config.ResolveAppConfigPath(opts.ConfigPath)
   if err != nil {
       fmt.Fprintf(os.Stderr, "Error resolving app config path: %v\n", err)
       os.Exit(1)
   }
   appCfg, err := config.LoadAppConfig(resolvedAppConfigPath, opts.ConfigPath != "")
   if err != nil {
       fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
       os.Exit(1)
   }
   if opts.NameTemplate != "" {
       appCfg.Naming.Template = opts.NameTemplate
   }
   ```
3. Pass `appCfg.Naming.Template` into `services.BuildRemotes(entries, appCfg.Naming.Template)`.

- [x] **Step 4: Run tests to verify all tests pass**

Run: `go test -v -race ./...`
Expected: PASS

---

### Task 4: Documentation & Regression Verification

**Files:**
- Modify: `README.md`
- Verify: `make test` and `make lint`

- [x] **Step 1: Update `README.md`**

Add a dedicated section **Configuration & Remote Naming** explaining:
- The XDG path (`~/.config/ssh2rclone/config.yaml`).
- Template syntax (`{alias}`, `{user}`, `{host}`, `{port}`) and inline defaults (`{user:-root}`).
- CLI options (`--config`, `--name-template`).

- [x] **Step 2: Run full verification suite**

Run: `make test`
Expected: PASS with 0 race conditions.

Run: `make lint` (if golangci-lint is present)
Expected: Clean.
