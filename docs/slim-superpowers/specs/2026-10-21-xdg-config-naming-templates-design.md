# Design Doc: XDG Configuration & Remote Naming Templates

## 1. Overview
`ssh2rclone` converts SSH hosts from `~/.ssh/config` into SFTP remotes in `rclone.conf`.
Currently, the remote name in `rclone.conf` strictly matches the SSH `Host` alias (`SSHEntry.Alias`).

This design introduces:
1. An extensible application configuration file stored according to XDG specifications (`$XDG_CONFIG_HOME/ssh2rclone/config.yaml` or `~/.config/ssh2rclone/config.yaml`).
2. A flexible remote naming convention engine supporting placeholders (`{user}`, `{host}`, `{alias}`, `{port}`) and shell-style inline defaults (`{user:-default}`).
3. CLI overrides via `--config` and `--name-template`.
4. Internal collision detection ensuring duplicate generated remote names are skipped with a clear warning on `os.Stderr`.

---

## 2. Goals & Non-Goals

### Goals
- **Extensible Configuration:** Support structured YAML configuration for current and future tool options.
- **Custom Naming Conventions:** Allow users to format remote names (e.g. `{user:-root}_{alias}`).
- **Graceful Fallback:** If no config file is found, retain full backward compatibility using `{alias}` without requiring user intervention.
- **Safety & Sanitization:** Sanitize generated remote names against invalid rclone characters (spaces, brackets, control characters).
- **Collision Protection:** Warn and skip subsequent duplicates if multiple SSH entries evaluate to identical remote names.

### Non-Goals
- Modifying remotes outside the `# --- BEGIN ssh2rclone ...` delimiter block.
- Replacing external rclone configuration management.

---

## 3. Configuration Architecture

### 3.1 File Locations & Precedence
1. **CLI Flag:** `--config <path>` (supports `~` home directory expansion).
2. **Environment Variable:** `$XDG_CONFIG_HOME/ssh2rclone/config.yaml` (if set).
3. **Default Location:** `~/.config/ssh2rclone/config.yaml`.
4. **Missing File Handling:**
   - If no `--config` flag is passed and the default file does not exist, use built-in default values (`template: "{alias}"`) with no error.
   - If `--config` is explicitly provided and the target path does not exist, return an error.

### 3.2 YAML Schema (`config.yaml`)
```yaml
version: 1

naming:
  template: "{user:-root}_{alias}"
```

### 3.3 Go Data Structures (`internal/model/config.go`)
```go
package model

// AppConfig represents the root application configuration.
type AppConfig struct {
	Version int          `yaml:"version"`
	Naming  NamingConfig `yaml:"naming"`
}

// NamingConfig defines naming rules for generated rclone remotes.
type NamingConfig struct {
	Template string `yaml:"template"`
}

// DefaultAppConfig returns the standard fallback configuration.
func DefaultAppConfig() AppConfig {
	return AppConfig{
		Version: 1,
		Naming: NamingConfig{
			Template: "{alias}",
		},
	}
}
```

---

## 4. Naming Engine & Template Syntax

### 4.1 Supported Placeholders
| Placeholder | Alternate Alias | Description | Example Value |
|---|---|---|---|
| `{alias}` | `{name}` | The SSH host alias from `Host` | `web-prod` |
| `{user}` | `{username}` | The SSH `User` directive (empty string if unset) | `ubuntu` |
| `{host}` | `{hostname}` | The resolved `HostName` (defaults to alias if unset) | `192.168.1.50` |
| `{port}` | | The SSH `Port` (defaults to `22` if unset) | `22` |

### 4.2 Inline Default Syntax (`{var:-default}`)
- If the variable is defined and non-empty, its value is used.
- If the variable is empty or unset in SSH config, the fallback string after `:-` is used.
- Example: `{user:-root}_{alias}`:
  - If `User` is `admin`: `admin_web-prod`
  - If `User` is not specified: `root_web-prod`
- If no fallback is provided and the variable is empty (e.g. `{user}_{alias}` where `user` is unset), the placeholder evaluates to an empty string (`_web-prod`).

### 4.3 Sanitization Rules
Names in `rclone.conf` identify INI sections (`[remote_name]`). To prevent corruption:
- Characters forbidden in section headers (`[`, `]`, `\n`, `\r`) are stripped.
- Whitespace and tabs are replaced with dashes (`-`).
- Leading and trailing whitespace or dashes are trimmed.
- If a name is completely empty after rendering and sanitization, it falls back to `{alias}`.

### 4.4 Internal Collision Resolution
When iterating through parsed SSH entries:
- Maintain a `seenRemotes := make(map[string]bool)`.
- If a generated remote name already exists in `seenRemotes`:
  - Emit a warning to `os.Stderr`:
    `Warning: duplicate remote name "%s" generated for SSH alias "%s", skipping`
  - Do not append to the generated remotes slice.
- Note: Conflicts with remotes outside the delimiter block continue to be handled by existing logic in `internal/services/rclone_sync.go`.

---

## 5. CLI Integration & Execution Flow

### 5.1 CLI Flags
- `-config <path>`: Path to custom `config.yaml`.
- `-name-template <pattern>`: Override the naming template at runtime.

### 5.2 Updated Execution Pipeline
1. **Flag Parsing:** Parse CLI arguments in `main.go`.
2. **Config Loading:**
   - Resolve config path via `config.ResolveAppConfigPath(opts.ConfigPath)`.
   - Call `config.LoadAppConfig(resolvedPath)`.
   - If `-name-template` flag was provided, override `appConfig.Naming.Template`.
3. **SSH Parsing:** Read `~/.ssh/config` into `[]model.SSHEntry`.
4. **Remote Generation:**
   - Pass `naming.Template` into `services.BuildRemotes(entries, template)`.
   - Format each remote name using `services.FormatRemoteName(entry, template)`.
   - Apply sanitization and deduplication.
5. **Rclone Sync:**
   - Compare with remotes outside delimiter block.
   - Atomically update `rclone.conf` or output dry-run changes.

---

## 6. Testing Strategy

1. **Naming Engine Tests (`internal/services/naming_test.go`):**
   - Simple placeholders: `{alias}`, `{host}`, `{user}`, `{port}`.
   - Inline defaults: `{user:-root}`, `{host:-localhost}`.
   - Missing fields without default.
   - Sanitization of spaces, brackets, and empty names.
   - Internal duplicate remote detection and filtering.
2. **Config Tests (`internal/config/app_config_test.go`):**
   - Valid YAML deserialization.
   - Default configuration fallback when file does not exist.
   - Error handling when explicit `--config` path is not found.
   - XDG environment variable precedence.
3. **Integration Tests (`main_test.go` / `rclone_sync_test.go`):**
   - End-to-end sync using custom templates.
   - Verification that backward compatibility is preserved when no config is present.
