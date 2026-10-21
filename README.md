# ssh2rclone

Convert SSH configurations (`~/.ssh/config`) to `rclone` remote definitions (`rclone.conf`) safely and idempotently.

## Features

- **Idempotent Updates:** Encloses generated remotes within dedicated delimiters (`# --- BEGIN ssh2rclone ... # --- END ssh2rclone`). Rerunning cleanly updates the block.
- **Strict Isolation:** Never modifies or deletes any remote defined manually outside the delimiter block.
- **Conflict Prevention:** Skips any SSH host with a warning if a manual remote with the same name already exists outside the block.
- **Smart Delegation:**
  - Standard hosts: converted to native rclone SFTP remotes (`host`, `user`, `port`, `key_file`).
  - Jump/Proxy hosts: automatically configured to delegate to OpenSSH (`ssh = ssh <alias>`), inheriting `ProxyJump`, `ProxyCommand`, and agent keys.
- **Custom Remote Naming & XDG Config:** Define custom naming patterns (e.g. `{user:-root}_{alias}`) via `$XDG_CONFIG_HOME/ssh2rclone/config.yaml` or CLI flags.
- **Automatic Path Discovery:** Detects `rclone.conf` through CLI flag, `$RCLONE_CONFIG`, `rclone config file`, or XDG standard paths.
- **Safe Operations:** Atomic file writes (temp file + rename) and automatic backup (`rclone.conf.bak`).

## Installation

### 1. Pre-built Binaries (Linux, macOS, Windows)

Download the latest release archive for your operating system and architecture from the [GitHub Releases](https://github.com/capimichi/ssh2rclone/releases/latest) page.

#### macOS & Linux (Manual install)

```bash
# 1. Download the archive for your OS (e.g. macOS Apple Silicon: darwin_arm64)
# Supported: linux_amd64, linux_arm64, darwin_amd64, darwin_arm64
VERSION="0.1.0"
OS="darwin"    # or "linux"
ARCH="arm64"   # or "amd64"

curl -LO "https://github.com/capimichi/ssh2rclone/releases/download/v${VERSION}/ssh2rclone_${VERSION}_${OS}_${ARCH}.tar.gz"

# 2. Extract the binary
tar -xzf "ssh2rclone_${VERSION}_${OS}_${ARCH}.tar.gz"

# 3. Move it to a directory in your PATH (e.g. /usr/local/bin or ~/.local/bin)
sudo mv ssh2rclone /usr/local/bin/

# 4. Verify installation
ssh2rclone --version
```

#### Windows (PowerShell)

```powershell
# 1. Download the zip archive for Windows (amd64 or arm64)
$VERSION="0.1.0"
$ARCH="amd64" # or "arm64"
Invoke-WebRequest -Uri "https://github.com/capimichi/ssh2rclone/releases/download/v$VERSION/ssh2rclone_${VERSION}_windows_${ARCH}.zip" -OutFile "ssh2rclone.zip"

# 2. Extract and run
Expand-Archive ssh2rclone.zip -DestinationPath .
.\ssh2rclone.exe --version
```

---

### 2. Using Go

If you have Go 1.23+ installed:

```bash
go install github.com/capimichi/ssh2rclone@latest
```
*(Ensure `$GOPATH/bin` or `$HOME/go/bin` is in your `$PATH`)*

---

### 3. Build from Source

```bash
git clone https://github.com/capimichi/ssh2rclone.git
cd ssh2rclone
make build

# The compiled binary will be in ./bin/ssh2rclone
sudo cp bin/ssh2rclone /usr/local/bin/
```

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

# Override remote naming template via CLI
ssh2rclone --name-template "{user:-root}_{alias}"

# Specify custom configuration file
ssh2rclone --config /path/to/config.yaml

# Disable automatic .bak backup
ssh2rclone --no-backup

# Verbose logging
ssh2rclone -v

# Print version
ssh2rclone --version
```

## Configuration & Remote Naming

`ssh2rclone` supports an extensible configuration file stored in accordance with the XDG Base Directory specification:
* `$XDG_CONFIG_HOME/ssh2rclone/config.yaml`
* `~/.config/ssh2rclone/config.yaml` (default fallback)

If the configuration file does not exist, `ssh2rclone` defaults to `{alias}`, maintaining 100% backward compatibility.

### Sample `config.yaml`

```yaml
version: 1

naming:
  # Template used to generate rclone remote section names
  template: "{user:-root}_{alias}"
```

### Template Variables & Syntax

| Token | Description | Example Value |
|---|---|---|
| `{alias}` (or `{name}`) | SSH host alias | `web-prod` |
| `{user}` (or `{username}`) | SSH `User` directive (empty string if unset) | `deploy` |
| `{host}` (or `{hostname}`) | Resolved `HostName` (falls back to alias if unset) | `192.168.1.50` |
| `{port}` | SSH `Port` (defaults to `22` if unset) | `2202` |

#### Inline Defaults (`{var:-default}`)
You can specify fallback values if a variable is unset in `~/.ssh/config`:
* `{user:-root}_{alias}`: If `User` is missing, produces `root_web-prod`.
* `{user}_{alias}`: If `User` is missing, produces `_web-prod`.

#### Sanitization & Collision Protection
* Any spaces, tabs, or newlines in rendered names are normalized to hyphens (`-`).
* Brackets (`[` and `]`) are stripped to prevent invalid INI section syntax.
* If multiple SSH entries produce the exact same remote name, subsequent duplicates are skipped with a warning on `os.Stderr`, preserving the first entry.

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
