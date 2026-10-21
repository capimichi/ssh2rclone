# Agent Guidelines for ssh2rclone

This document contains operational instructions, architecture invariants, and release automation guidelines for AI agents working in this repository.

---

## 1. Architecture & Repository Layout

`ssh2rclone` converts `~/.ssh/config` into idempotent SFTP remote blocks in `rclone.conf`.

- `main.go`: CLI entry point, argument parsing, version output (`--version`), and orchestration.
- `internal/model/`: Data structures (`SSHEntry`, `RcloneRemote`, `SyncOptions`, `SyncResult`).
- `internal/config/`: Path resolution for SSH configs and `rclone.conf` (CLI, env, binary, XDG).
- `internal/services/`: Core logic:
  - `ssh_parser.go`: SSH config parser, wildcard filters (`*`, `*.lan`), inheritance, jump/proxy detection.
  - `rclone_sync.go`: Block delimiter management, conflict detection with external remotes, and atomic replacement.
- `internal/helper/`: Atomic file writes (`AtomicWriteFile`) and backups (`CreateBackup`).
- `.github/workflows/`:
  - `ci.yml`: CI running tests with race detector and `golangci-lint` on pushes/PRs.
  - `release.yml`: Release automation with GoReleaser triggered on `v*` tags.
- `.goreleaser.yaml`: GoReleaser v2 specification for multi-arch builds (`linux`, `darwin`, `windows` on `amd64`, `arm64`).
- `Makefile`: Standard developer targets (`build`, `test`, `lint`, `snapshot`, `clean`).

---

## 2. Core Invariants & Rules

1. **Strict Isolation:** NEVER modify, delete, or re-order any remote or configuration located *outside* the `# --- BEGIN ssh2rclone ...` and `# --- END ssh2rclone ...` delimiter block.
2. **Conflict Resolution:** If an SSH host has the same name as a remote defined *outside* the delimiter block, DO NOT overwrite it or rename it; skip it and emit a warning to `os.Stderr`.
3. **Idempotency:** Re-running `ssh2rclone` multiple times with the same input must produce the exact same `rclone.conf` state.
4. **Safety First:** Always use atomic writes (write to temp file in same directory + `os.Rename`) and keep backups enabled by default (`.bak`).

---

## 3. Development Checklist

Before committing any changes or creating a release:
- Always run the test suite:
  ```bash
  make test
  ```
- If `golangci-lint` is available, verify linting:
  ```bash
  make lint
  ```
- Ensure zero breaking changes to existing delimiter contracts or configuration resolution.

---

## 4. Release & Tagging Guidelines for Agents

When requested by the user to release or tag a new version:

1. **Verify Git State:**
   - Ensure the working tree is clean (`git status --porcelain` is empty).
   - Ensure all changes are committed and pushed to `main`.
2. **Follow Semantic Versioning (SemVer):**
   - Format: `vMAJOR.MINOR.PATCH` (e.g., `v0.1.0`, `v0.1.1`, `v0.2.0`).
   - Use `v0.x.y` while the project is in initial feature development.
3. **Tagging Steps:**
   - Create an annotated Git tag:
     ```bash
     git tag -a vX.Y.Z -m "Release vX.Y.Z"
     ```
   - Push the tag to `origin`:
     ```bash
     git push origin vX.Y.Z
     ```
4. **Automated Pipeline Trigger:**
   - Pushing a tag matching `v*` immediately triggers `.github/workflows/release.yml`.
   - GoReleaser compiles binaries for all 6 target architectures, calculates SHA-256 checksums, and publishes the release on GitHub.
   - Do NOT run manual GitHub release creation if GoReleaser is active.
