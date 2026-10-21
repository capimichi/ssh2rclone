# Design Doc: Dependency Injection Architecture with Uber Fx

**Date:** 2026-10-21  
**Status:** Approved  
**Topic:** Refactoring of `ssh2rclone` to introduce Dependency Injection via `go.uber.org/fx`

---

## 1. Overview & Motivation

`ssh2rclone` synchronizes OpenSSH configuration blocks into `rclone.conf`. Prior to this refactoring, components in `internal/services` and `internal/helper` were implemented as standalone package-level functions, and `main.go` orchestrated configuration loading, path resolution, parsing, and synchronization imperatively.

This design introduces a unified **Dependency Injection (DI)** architecture using **Uber Fx** (`go.uber.org/fx`). Each major concern is decoupled behind a Go interface, constructed via constructor functions (`New...`), and injected into consumers. This improves testability, enforces separation of concerns, and simplifies future extensions (e.g., daemon mode, custom resolvers).

---

## 2. Architectural Blueprint

```
                      +-------------------+
                      |      main.go      |
                      +---------+---------+
                                |
                                v
                      +-------------------+
                      |   internal/di     |
                      |   (Fx Module)     |
                      +---+-----+-----+---+
                          |     |     |
          +---------------+     |     +----------------+
          v                     v                      v
+-------------------+ +-------------------+ +---------------------+
|  internal/helper  | | internal/services | | internal/config     |
|  (FileHelper)     | | (SSHParser,       | | (ConfigResolver,    |
|                   | |  NamingFormatter) | |  ResolvedConfig)    |
+---------+---------+ +---------+---------+ +----------+----------+
          |                     |                      |
          +--------------+------+                      |
                         v                             |
              +---------------------+                  |
              |  internal/services  |                  |
              |   (RcloneSyncer)    |                  |
              +----------+----------+                  |
                         |                             |
                         +--------------+--------------+
                                        v
                              +-------------------+
                              |   internal/app    |
                              |   (AppRunner)     |
                              +-------------------+
```

---

## 3. Component Specifications & Interfaces

### 3.1. File System Operations (`internal/helper`)
Decouples disk I/O, atomic writes, and backups from business logic.

```go
package helper

import "os"

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
```

### 3.2. SSH Parsing (`internal/services`)
Decouples SSH config file parsing and host extraction.

```go
package services

import "ssh2rclone/internal/model"

type SSHParser interface {
    Parse(path string, opts model.SyncOptions) ([]model.SSHEntry, error)
}

type sshParser struct{}

func NewSSHParser() SSHParser {
    return &sshParser{}
}
```

### 3.3. Remote Naming Formatter (`internal/services`)
Formats remote section names based on user templates.

```go
package services

import "ssh2rclone/internal/model"

type RemoteNameFormatter interface {
    Format(entry model.SSHEntry, tmpl string) (string, error)
}

type remoteNameFormatter struct{}

func NewRemoteNameFormatter() RemoteNameFormatter {
    return &remoteNameFormatter{}
}
```

### 3.4. Rclone Synchronization (`internal/services`)
Encapsulates delimiter management, conflict detection, and `rclone.conf` atomic replacement. Receives `FileHelper` and `RemoteNameFormatter`.

```go
package services

import (
    "ssh2rclone/internal/helper"
    "ssh2rclone/internal/model"
)

type RcloneSyncer interface {
    Sync(entries []model.SSHEntry, opts model.SyncOptions) (*model.SyncResult, error)
}

type rcloneSyncer struct {
    fileHelper helper.FileHelper
    formatter  RemoteNameFormatter
}

func NewRcloneSyncer(fh helper.FileHelper, nf RemoteNameFormatter) RcloneSyncer {
    return &rcloneSyncer{
        fileHelper: fh,
        formatter:  nf,
    }
}
```

### 3.5. Application Orchestration (`internal/app`)
Coordinates the workflow, dry-run mode, and CLI reporting.

```go
package app

import (
    "context"
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
    // 1. Parse SSH config entries
    // 2. Sync to rclone.conf
    // 3. Render dry-run output or print sync summary and warnings
}
```

---

## 4. Uber Fx Wiring (`internal/di`)

The composition root lives in `internal/di`:

- **Providers**:
  - `helper.NewOsFileHelper` -> provides `helper.FileHelper`
  - `services.NewRemoteNameFormatter` -> provides `services.RemoteNameFormatter`
  - `services.NewSSHParser` -> provides `services.SSHParser`
  - `services.NewRcloneSyncer` -> provides `services.RcloneSyncer`
  - `app.NewApp` -> provides `*app.App`
  - Configuration provider -> loads and resolves paths into `model.SyncOptions`
- **Logging Strategy**:
  - Internal Fx logs are suppressed by default via `fxevent.NopLogger`.
  - When `--verbose` / `-v` is active, Fx logs to `os.Stderr` via `fxevent.ConsoleLogger`.
- **Entrypoint Hook**:
  - An `fx.Invoke` registers an `fx.Hook` on `fx.Lifecycle`.
  - On `OnStart`, execution runs `app.Run(ctx)` and signals graceful shutdown via `fx.Shutdowner`.

---

## 5. Invariants & Safety

1. **Strict Block Isolation:** All changes to `rclone.conf` remain strictly bounded between `# --- BEGIN ssh2rclone ...` and `# --- END ssh2rclone ...`.
2. **Conflict Resolution:** Manual remotes outside the block always take precedence over SSH hosts.
3. **CLI Consistency:** All existing CLI flags (`-ssh-config`, `-rclone-config`, `-config`, `-name-template`, `-dry-run`, `-all-ssh`, `-no-backup`, `-v`, `-version`) preserve identical semantics and exit codes.
4. **Atomic Writes & Backups:** Replaced configurations are always saved atomically with `.bak` generated prior to writing unless `-no-backup` is set.

---

## 6. Testing Strategy

1. **Unit Tests with Fakes**:
   - `services.RcloneSyncer` tested with an in-memory `MockFileHelper` to test edge cases (missing config, conflicts, permissions) without disk I/O.
   - `services.SSHParser` and `services.RemoteNameFormatter` isolated unit tests.
2. **Fx Graph Verification**:
   - Use `go.uber.org/fx/fxtest` to validate that the dependency graph builds cleanly and all providers satisfy their consumers.
3. **Integration & End-to-End Tests**:
   - Existing end-to-end tests in `main_test.go` and `services/*_test.go` verifying atomic disk writes and CLI flags remain intact.
