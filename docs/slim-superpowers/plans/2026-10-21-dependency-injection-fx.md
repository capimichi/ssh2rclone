# Dependency Injection (Uber Fx) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Refactor `ssh2rclone` to introduce a unified Dependency Injection architecture using Uber Fx (`go.uber.org/fx`), decoupling services behind interfaces for enhanced testability and clean architecture.

**Architecture:** Each core component (`FileHelper`, `SSHParser`, `RemoteNameFormatter`, `RcloneSyncer`) is defined by an interface and constructed via `New...` constructors. `internal/app` coordinates the business workflow, while `internal/di` acts as the single composition root using Uber Fx modules and lifecycle hooks. `main.go` parses CLI flags and boots the DI container.

**Tech Stack:** Go 1.23+, `go.uber.org/fx`, `go.uber.org/fx/fxtest`, OpenSSH parser (`github.com/kevinburke/ssh_config`).

## Global Constraints

- Never modify, delete, or re-order any remote or configuration outside `# --- BEGIN ssh2rclone ...` and `# --- END ssh2rclone ...`.
- Skip hosts that conflict with external manual remotes and print a warning to `os.Stderr`.
- Maintain 100% CLI flag compatibility (`-ssh-config`, `-rclone-config`, `-config`, `-name-template`, `-dry-run`, `-all-ssh`, `-no-backup`, `-v`, `-version`).
- Keep internal Fx logs suppressed (`fxevent.NopLogger`) unless `-v` / `--verbose` is specified.
- Retain atomic file writes (`AtomicWriteFile`) and backup generation (`.bak`).

---

### Task 1: Add Dependency and FileHelper Interface (`internal/helper`)

**Files:**
- Modify: `go.mod`
- Modify: `internal/helper/file.go`
- Create: `internal/helper/mock_file.go`
- Test: `internal/helper/file_test.go`

**Interfaces:**
- Consumes: Standard Go `os` package.
- Produces:
  - `helper.FileHelper`: `ReadFile(path string) ([]byte, error)`, `AtomicWriteFile(path string, data []byte, perm os.FileMode) error`, `CreateBackup(srcPath string) (string, error)`, `FileExists(path string) bool`.
  - `helper.NewOsFileHelper() helper.FileHelper`
  - `helper.NewMockFileHelper() *helper.MockFileHelper`

- [x] **Step 1: Add `go.uber.org/fx` to `go.mod`**

Run: `go get go.uber.org/fx@v1.23.0`
Expected: `go.mod` updated with `go.uber.org/fx` and dependencies.

- [x] **Step 2: Write failing test for `OsFileHelper` and `MockFileHelper`**

Add to `internal/helper/file_test.go`:
```go
func TestOsFileHelper(t *testing.T) {
	fh := NewOsFileHelper()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	err := fh.AtomicWriteFile(filePath, []byte("hello world"), 0600)
	if err != nil {
		t.Fatalf("AtomicWriteFile failed: %v", err)
	}

	if !fh.FileExists(filePath) {
		t.Fatalf("expected file to exist")
	}

	data, err := fh.ReadFile(filePath)
	if err != nil || string(data) != "hello world" {
		t.Fatalf("ReadFile failed: got %q, err %v", string(data), err)
	}

	bak, err := fh.CreateBackup(filePath)
	if err != nil || bak == "" {
		t.Fatalf("CreateBackup failed: %v", err)
	}
}

func TestMockFileHelper(t *testing.T) {
	mf := NewMockFileHelper()
	mf.Files["/etc/rclone.conf"] = []byte("[test]\ntype = sftp\n")

	if !mf.FileExists("/etc/rclone.conf") {
		t.Fatalf("expected /etc/rclone.conf to exist in mock")
	}

	data, err := mf.ReadFile("/etc/rclone.conf")
	if err != nil || string(data) != "[test]\ntype = sftp\n" {
		t.Fatalf("unexpected content: %s", string(data))
	}

	err = mf.AtomicWriteFile("/etc/rclone.conf", []byte("[updated]\n"), 0600)
	if err != nil || string(mf.Files["/etc/rclone.conf"]) != "[updated]\n" {
		t.Fatalf("mock AtomicWriteFile failed")
	}
}
```

- [x] **Step 3: Run test to verify it fails**

Run: `go test ./internal/helper/... -run "TestOsFileHelper|TestMockFileHelper"`
Expected: FAIL compilation error (undefined `NewOsFileHelper`, `NewMockFileHelper`).

- [x] **Step 4: Implement `FileHelper` interface, `OsFileHelper`, and `MockFileHelper`**

In `internal/helper/file.go`, define the interface and struct:
```go
type FileHelper interface {
	ReadFile(path string) ([]byte, error)
	AtomicWriteFile(path string, data []byte, perm os.FileMode) error
	CreateBackup(srcPath string) (string, error)
	FileExists(path string) bool
}

type OsFileHelper struct{}

func NewOsFileHelper() FileHelper {
	return &OsFileHelper{}
}

func (h *OsFileHelper) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (h *OsFileHelper) AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	return AtomicWriteFile(path, data, perm)
}

func (h *OsFileHelper) CreateBackup(srcPath string) (string, error) {
	return CreateBackup(srcPath)
}

func (h *OsFileHelper) FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
```

In `internal/helper/mock_file.go`:
```go
package helper

import (
	"fmt"
	"os"
)

type MockFileHelper struct {
	Files   map[string][]byte
	Backups []string
	FailOn  map[string]error
}

func NewMockFileHelper() *MockFileHelper {
	return &MockFileHelper{
		Files:  make(map[string][]byte),
		FailOn: make(map[string]error),
	}
}

func (m *MockFileHelper) ReadFile(path string) ([]byte, error) {
	if err, ok := m.FailOn["ReadFile:"+path]; ok {
		return nil, err
	}
	content, ok := m.Files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return content, nil
}

func (m *MockFileHelper) AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	if err, ok := m.FailOn["AtomicWriteFile:"+path]; ok {
		return err
	}
	m.Files[path] = data
	return nil
}

func (m *MockFileHelper) CreateBackup(srcPath string) (string, error) {
	if err, ok := m.FailOn["CreateBackup:"+srcPath]; ok {
		return "", err
	}
	if !m.FileExists(srcPath) {
		return "", nil
	}
	bakPath := srcPath + ".bak"
	m.Files[bakPath] = m.Files[srcPath]
	m.Backups = append(m.Backups, bakPath)
	return bakPath, nil
}

func (m *MockFileHelper) FileExists(path string) bool {
	_, ok := m.Files[path]
	return ok
}
```

- [x] **Step 5: Run tests and verify they pass**

Run: `go test -v ./internal/helper/...`
Expected: PASS

---

### Task 2: Refactor RemoteNameFormatter to Interface and Constructor (`internal/services/naming.go`)

**Files:**
- Modify: `internal/services/naming.go`
- Test: `internal/services/naming_test.go`

**Interfaces:**
- Consumes: `model.SSHEntry`.
- Produces:
  - `services.RemoteNameFormatter`: `Format(entry model.SSHEntry, tmpl string) (string, error)`.
  - `services.NewRemoteNameFormatter() services.RemoteNameFormatter`.
  - `services.FormatRemoteName(entry model.SSHEntry, tmpl string) (string, error)` (compatibility function).

- [x] **Step 1: Write test for `RemoteNameFormatter` interface**

In `internal/services/naming_test.go`:
```go
func TestRemoteNameFormatterInterface(t *testing.T) {
	formatter := NewRemoteNameFormatter()
	entry := model.SSHEntry{
		Alias:    "srv1",
		HostName: "srv1.internal",
		User:     "admin",
	}

	name, err := formatter.Format(entry, "{user:-root}_{alias}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "admin_srv1" {
		t.Fatalf("expected admin_srv1, got %s", name)
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/services/... -run TestRemoteNameFormatterInterface`
Expected: FAIL compilation error (undefined `NewRemoteNameFormatter`).

- [x] **Step 3: Implement `RemoteNameFormatter` interface**

In `internal/services/naming.go`:
```go
type RemoteNameFormatter interface {
	Format(entry model.SSHEntry, tmpl string) (string, error)
}

type remoteNameFormatter struct{}

func NewRemoteNameFormatter() RemoteNameFormatter {
	return &remoteNameFormatter{}
}

func (f *remoteNameFormatter) Format(entry model.SSHEntry, tmpl string) (string, error) {
	return FormatRemoteName(entry, tmpl)
}
```

- [x] **Step 4: Run tests and verify they pass**

Run: `go test -v ./internal/services/... -run "TestFormatRemoteName.*|TestRemoteNameFormatterInterface"`
Expected: PASS

---

### Task 3: Refactor SSHParser to Interface and Constructor (`internal/services/ssh_parser.go`)

**Files:**
- Modify: `internal/services/ssh_parser.go`
- Test: `internal/services/ssh_parser_test.go`

**Interfaces:**
- Consumes: `model.SyncOptions`.
- Produces:
  - `services.SSHParser`: `Parse(path string, opts model.SyncOptions) ([]model.SSHEntry, error)`.
  - `services.NewSSHParser() services.SSHParser`.
  - `services.ParseSSHConfig(path string, opts model.SyncOptions) ([]model.SSHEntry, error)` (compatibility wrapper).

- [x] **Step 1: Write test for `SSHParser` interface**

In `internal/services/ssh_parser_test.go`:
```go
func TestSSHParserInterface(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config")
	content := `Host web
    HostName 192.168.1.10
    User webmaster
`
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	parser := NewSSHParser()
	entries, err := parser.Parse(cfgPath, model.SyncOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].Alias != "web" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/services/... -run TestSSHParserInterface`
Expected: FAIL compilation error (undefined `NewSSHParser`).

- [x] **Step 3: Implement `SSHParser` interface**

In `internal/services/ssh_parser.go`:
```go
type SSHParser interface {
	Parse(path string, opts model.SyncOptions) ([]model.SSHEntry, error)
}

type sshParser struct{}

func NewSSHParser() SSHParser {
	return &sshParser{}
}

func (p *sshParser) Parse(path string, opts model.SyncOptions) ([]model.SSHEntry, error) {
	return ParseSSHConfig(path, opts)
}
```

- [x] **Step 4: Run tests and verify they pass**

Run: `go test -v ./internal/services/... -run "TestParseSSHConfig.*|TestSSHParserInterface"`
Expected: PASS

---

### Task 4: Refactor RcloneSyncer to Interface and Inject FileHelper (`internal/services/rclone_sync.go`)

**Files:**
- Modify: `internal/services/rclone_sync.go`
- Test: `internal/services/rclone_sync_test.go`

**Interfaces:**
- Consumes: `helper.FileHelper`, `services.RemoteNameFormatter`, `model.SyncOptions`, `model.SSHEntry`.
- Produces:
  - `services.RcloneSyncer`: `Sync(entries []model.SSHEntry, opts model.SyncOptions) (*model.SyncResult, error)`.
  - `services.NewRcloneSyncer(fh helper.FileHelper, nf RemoteNameFormatter) services.RcloneSyncer`.
  - `services.SyncRcloneConfig(entries []model.SSHEntry, opts model.SyncOptions) (*model.SyncResult, error)` (backward compatibility wrapper).

- [x] **Step 1: Write unit test for `RcloneSyncer` using `MockFileHelper`**

In `internal/services/rclone_sync_test.go`:
```go
func TestRcloneSyncerWithMockFileHelper(t *testing.T) {
	mockHelper := helper.NewMockFileHelper()
	rclonePath := "/home/user/.config/rclone/rclone.conf"
	existingConfig := `[manual_remote]
type = sftp
host = 1.2.3.4
`
	mockHelper.Files[rclonePath] = []byte(existingConfig)

	syncer := NewRcloneSyncer(mockHelper, NewRemoteNameFormatter())
	entries := []model.SSHEntry{
		{
			Alias:    "manual_remote", // Should conflict and be skipped
			HostName: "1.2.3.4",
		},
		{
			Alias:    "new_remote",
			HostName: "5.6.7.8",
			User:     "root",
			Port:     "22",
		},
	}

	opts := model.SyncOptions{
		RcloneConfigPath: rclonePath,
		DryRun:           false,
	}

	res, err := syncer.Sync(entries, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.SkippedConflicts) != 1 || res.SkippedConflicts[0] != "manual_remote" {
		t.Errorf("expected conflict on manual_remote, got %+v", res.SkippedConflicts)
	}
	if len(res.AddedRemotes) != 1 || res.AddedRemotes[0].Name != "new_remote" {
		t.Errorf("expected new_remote added, got %+v", res.AddedRemotes)
	}

	// Verify updated config was saved via mockHelper
	saved := string(mockHelper.Files[rclonePath])
	if !strings.Contains(saved, "[manual_remote]") || !strings.Contains(saved, "[new_remote]") {
		t.Fatalf("saved config missing remotes:\n%s", saved)
	}
	if !strings.Contains(saved, BeginDelimiter) || !strings.Contains(saved, EndDelimiter) {
		t.Fatalf("saved config missing delimiters:\n%s", saved)
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/services/... -run TestRcloneSyncerWithMockFileHelper`
Expected: FAIL compilation error (undefined `NewRcloneSyncer`).

- [x] **Step 3: Implement `RcloneSyncer` interface and inject dependencies**

In `internal/services/rclone_sync.go`:
```go
type RcloneSyncer interface {
	Sync(entries []model.SSHEntry, opts model.SyncOptions) (*model.SyncResult, error)
}

type rcloneSyncer struct {
	fileHelper helper.FileHelper
	formatter  RemoteNameFormatter
}

func NewRcloneSyncer(fh helper.FileHelper, nf RemoteNameFormatter) RcloneSyncer {
	if fh == nil {
		fh = helper.NewOsFileHelper()
	}
	if nf == nil {
		nf = NewRemoteNameFormatter()
	}
	return &rcloneSyncer{
		fileHelper: fh,
		formatter:  nf,
	}
}

func (s *rcloneSyncer) Sync(entries []model.SSHEntry, opts model.SyncOptions) (*model.SyncResult, error) {
    // Read rclone.conf via s.fileHelper.ReadFile
    // Format names via s.formatter.Format
    // Create backup via s.fileHelper.CreateBackup
    // Write updated content via s.fileHelper.AtomicWriteFile
}

// Preserve existing SyncRcloneConfig as wrapper
func SyncRcloneConfig(entries []model.SSHEntry, opts model.SyncOptions) (*model.SyncResult, error) {
	syncer := NewRcloneSyncer(helper.NewOsFileHelper(), NewRemoteNameFormatter())
	return syncer.Sync(entries, opts)
}
```

- [x] **Step 4: Run tests and verify they pass**

Run: `go test -v ./internal/services/...`
Expected: PASS

---

### Task 5: Implement Application Orchestrator (`internal/app`)

**Files:**
- Create: `internal/app/app.go`
- Create: `internal/app/app_test.go`

**Interfaces:**
- Consumes: `services.SSHParser`, `services.RcloneSyncer`, `model.SyncOptions`.
- Produces:
  - `app.Runner`: `Run(ctx context.Context) error`.
  - `app.NewApp(parser services.SSHParser, syncer services.RcloneSyncer, opts model.SyncOptions) *app.App`.

- [x] **Step 1: Write test for `app.App`**

In `internal/app/app_test.go`:
```go
package app

import (
	"context"
	"testing"

	"ssh2rclone/internal/helper"
	"ssh2rclone/internal/model"
	"ssh2rclone/internal/services"
)

type mockSSHParser struct {
	entries []model.SSHEntry
	err     error
}

func (m *mockSSHParser) Parse(path string, opts model.SyncOptions) ([]model.SSHEntry, error) {
	return m.entries, m.err
}

func TestAppRunDryRun(t *testing.T) {
	mockParser := &mockSSHParser{
		entries: []model.SSHEntry{
			{Alias: "srv", HostName: "1.2.3.4"},
		},
	}
	mockHelper := helper.NewMockFileHelper()
	syncer := services.NewRcloneSyncer(mockHelper, services.NewRemoteNameFormatter())

	opts := model.SyncOptions{
		SSHConfigPath:    "/tmp/ssh_config",
		RcloneConfigPath: "/tmp/rclone.conf",
		DryRun:           true,
	}

	application := NewApp(mockParser, syncer, opts)
	err := application.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/...`
Expected: FAIL (package does not exist or types not found).

- [x] **Step 3: Implement `internal/app/app.go`**

```go
package app

import (
	"context"
	"fmt"
	"os"

	"ssh2rclone/internal/model"
	"ssh2rclone/internal/services"
)

type Runner interface {
	Run(ctx context.Context) error
}

type App struct {
	parser services.SSHParser
	syncer services.RcloneSyncer
	opts   model.SyncOptions
}

func NewApp(parser services.SSHParser, syncer services.RcloneSyncer, opts model.SyncOptions) *App {
	return &App{
		parser: parser,
		syncer: syncer,
		opts:   opts,
	}
}

func (a *App) Run(ctx context.Context) error {
	if a.opts.Verbose {
		fmt.Printf("Using SSH config: %s\n", a.opts.SSHConfigPath)
		fmt.Printf("Using rclone config: %s\n", a.opts.RcloneConfigPath)
		fmt.Printf("Using naming template: %s\n", a.opts.NameTemplate)
	}

	entries, err := a.parser.Parse(a.opts.SSHConfigPath, a.opts)
	if err != nil {
		return fmt.Errorf("parsing SSH config: %w", err)
	}

	if a.opts.Verbose {
		fmt.Printf("Discovered %d concrete host(s) in SSH config\n", len(entries))
	}

	res, err := a.syncer.Sync(entries, a.opts)
	if err != nil {
		return fmt.Errorf("updating rclone config: %w", err)
	}

	if a.opts.DryRun {
		fmt.Println("=== DRY RUN PREVIEW ===")
		fmt.Print(res.RenderedBlock)
		fmt.Printf("Summary: %d remote(s) to add, %d skipped due to external conflicts.\n",
			len(res.AddedRemotes), len(res.SkippedConflicts))
		return nil
	}

	for _, skipped := range res.SkippedConflicts {
		fmt.Fprintf(os.Stderr, "[WARN] Remote '%s' already exists in manual rclone config: skipped.\n", skipped)
	}

	if res.BackupPath != "" && a.opts.Verbose {
		fmt.Printf("Backup created at: %s\n", res.BackupPath)
	}

	fmt.Printf("Successfully synchronized %d remote(s) to %s\n", len(res.AddedRemotes), a.opts.RcloneConfigPath)
	return nil
}
```

- [x] **Step 4: Run tests and verify they pass**

Run: `go test -v ./internal/app/...`
Expected: PASS

---

### Task 6: Implement DI Module and Composition Root (`internal/di`)

**Files:**
- Create: `internal/di/module.go`
- Create: `internal/di/module_test.go`

**Interfaces:**
- Consumes: `helper`, `services`, `app`, `model.SyncOptions`.
- Produces:
  - `di.Module`: `fx.Option`
  - `di.Execute(opts model.SyncOptions) error`

- [x] **Step 1: Write test validating Fx Graph**

In `internal/di/module_test.go`:
```go
package di

import (
	"context"
	"testing"

	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"ssh2rclone/internal/app"
	"ssh2rclone/internal/model"
)

func TestFxGraphResolution(t *testing.T) {
	opts := model.SyncOptions{
		SSHConfigPath:    "/tmp/test_ssh",
		RcloneConfigPath: "/tmp/test_rclone",
		DryRun:           true,
	}

	testApp := fxtest.New(
		t,
		Module,
		fx.Provide(func() model.SyncOptions {
			return opts
		}),
		fx.Invoke(func(runner *app.App) {
			if runner == nil {
				t.Fatal("expected runner to be provided")
			}
		}),
	)

	testApp.RequireStart()
	testApp.RequireStop()
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/di/...`
Expected: FAIL (module not defined).

- [x] **Step 3: Implement `internal/di/module.go`**

```go
package di

import (
	"context"
	"os"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"ssh2rclone/internal/app"
	"ssh2rclone/internal/helper"
	"ssh2rclone/internal/model"
	"ssh2rclone/internal/services"
)

var Module = fx.Options(
	fx.Provide(
		helper.NewOsFileHelper,
		services.NewRemoteNameFormatter,
		services.NewSSHParser,
		services.NewRcloneSyncer,
		app.NewApp,
	),
)

func Execute(opts model.SyncOptions) error {
	var runErr error

	fxApp := fx.New(
		Module,
		fx.Provide(func() model.SyncOptions {
			return opts
		}),
		fx.WithLogger(func() fxevent.Logger {
			if opts.Verbose {
				return &fxevent.ConsoleLogger{W: os.Stderr}
			}
			return fxevent.NopLogger
		}),
		fx.Invoke(func(application *app.App, lc fx.Lifecycle, shutdowner fx.Shutdowner) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					runErr = application.Run(ctx)
					_ = shutdowner.Shutdown()
					return nil
				},
			})
		}),
	)

	if err := fxApp.Start(context.Background()); err != nil {
		return err
	}
	<-fxApp.Done()
	_ = fxApp.Stop(context.Background())

	return runErr
}
```

- [x] **Step 4: Run tests and verify they pass**

Run: `go test -v ./internal/di/...`
Expected: PASS

---

### Task 7: Rewire `main.go` and End-to-End Verification

**Files:**
- Modify: `main.go`

**Interfaces:**
- Consumes: `di.Execute(opts)`.

- [x] **Step 1: Update `main.go` to use `di.Execute`**

Replace the imperative parsing and execution lines in `main.go` with `di.Execute(opts)`:
```go
	// CLI flag overrides config file
	if opts.NameTemplate != "" {
		appCfg.Naming.Template = opts.NameTemplate
	}
	opts.NameTemplate = appCfg.Naming.Template

	if err := di.Execute(opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
```

- [x] **Step 2: Run entire test suite**

Run: `make test`
Expected: All tests pass with 0 failures and race detector clean.

- [x] **Step 3: Verify binary build and CLI execution**

Run:
```bash
make build
./ssh2rclone --version
./ssh2rclone -dry-run
```
Expected:
- Version printed with commit & date.
- Dry run executes smoothly via DI and prints preview/summary.
