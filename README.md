# ssh2rclone

Convert SSH configurations (`~/.ssh/config`) to `rclone` remote definitions (`rclone.conf`) safely and idempotently.

## Features

- **Idempotent Updates:** Encloses generated remotes within dedicated delimiters (`# --- BEGIN ssh2rclone ... # --- END ssh2rclone`). Rerunning cleanly updates the block.
- **Strict Isolation:** Never modifies or deletes any remote defined manually outside the delimiter block.
- **Conflict Prevention:** Skips any SSH host with a warning if a manual remote with the same name already exists outside the block.
- **Smart Delegation:**
  - Standard hosts: converted to native rclone SFTP remotes (`host`, `user`, `port`, `key_file`).
  - Jump/Proxy hosts: automatically configured to delegate to OpenSSH (`ssh = ssh <alias>`), inheriting `ProxyJump`, `ProxyCommand`, and agent keys.
- **Automatic Path Discovery:** Detects `rclone.conf` through CLI flag, `$RCLONE_CONFIG`, `rclone config file`, or XDG standard paths.
- **Safe Operations:** Atomic file writes (temp file + rename) and automatic backup (`rclone.conf.bak`).

## Installation

```bash
# Build locally
make build

# Or install to $GOPATH/bin
go install
```

Pre-built binaries for Linux, macOS, and Windows (amd64 / arm64) are also available on the [GitHub Releases](https://github.com/capimichi/ssh2rclone/releases) page.

## Usage

```bash
# Synchronize default ~/.ssh/config to detected rclone.conf
ssh2rclone

# Preview changes without writing to disk
ssh2rclone --dry-run

# Force all hosts to delegate through OpenSSH client
ssh2rclone --all-ssh

# Custom config paths
ssh2rclone --ssh-config /path/to/ssh_config --rclone-config /path/to/rclone.conf

# Disable automatic .bak backup
ssh2rclone --no-backup

# Verbose logging
ssh2rclone -v

# Print version
ssh2rclone --version
```

## Development

The project includes a `Makefile` to streamline local development:

```bash
# Run tests with race detector
make test

# Run linter (requires golangci-lint)
make lint

# Build binary with git version metadata to bin/ssh2rclone
make build

# Test GoReleaser build locally without publishing (requires goreleaser)
make snapshot

# Clean build artifacts
make clean
```

## Release & Versioning Guide

Releases are fully automated via **GitHub Actions** and **GoReleaser**.

### Creating a Release

1. Ensure all tests and linters pass:
   ```bash
   make test
   make lint
   ```
2. Create and push a signed/annotated Git tag following [Semantic Versioning](https://semver.org/) (`vMAJOR.MINOR.PATCH`):
   ```bash
   git tag -a v0.1.0 -m "Release v0.1.0"
   git push origin v0.1.0
   ```
3. GitHub Actions automatically triggers the `Release` workflow:
   - Builds binaries for Linux, macOS, and Windows on `amd64` and `arm64`.
   - Injects the version tag, commit hash, and build timestamp into the binary.
   - Generates release archives (`.tar.gz` and `.zip`) and `checksums.txt`.
   - Creates the GitHub Release with an auto-generated changelog.
