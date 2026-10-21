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
go build -o ssh2rclone main.go
# or install to $GOPATH/bin
go install
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

# Disable automatic .bak backup
ssh2rclone --no-backup

# Verbose logging
ssh2rclone -v
```
