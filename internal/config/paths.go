package config

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ExpandHome replaces a leading "~" or "~/" with the user's home directory.
func ExpandHome(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if path == "~" {
		return os.UserHomeDir()
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

// ResolveSSHConfigPath returns the path to the SSH config file.
func ResolveSSHConfigPath(cliPath string) (string, error) {
	if cliPath != "" {
		return ExpandHome(cliPath)
	}
	return ExpandHome("~/.ssh/config")
}

// ResolveRcloneConfigPath determines the rclone.conf location using:
// 1. CLI flag
// 2. RCLONE_CONFIG env var
// 3. `rclone config file` command if rclone is in $PATH
// 4. XDG_CONFIG_HOME or ~/.config/rclone/rclone.conf (with legacy ~/.rclone.conf check).
func ResolveRcloneConfigPath(cliPath string) (string, error) {
	if cliPath != "" {
		return ExpandHome(cliPath)
	}

	if envPath := os.Getenv("RCLONE_CONFIG"); envPath != "" {
		return ExpandHome(envPath)
	}

	// Try querying rclone binary directly if installed
	if rcloneBin, err := exec.LookPath("rclone"); err == nil && rcloneBin != "" {
		cmd := exec.Command("rclone", "config", "file")
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err == nil {
			lines := strings.Split(out.String(), "\n")
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if strings.HasSuffix(trimmed, "rclone.conf") {
					return trimmed, nil
				}
			}
		}
	}

	// Check legacy ~/.rclone.conf
	home, err := os.UserHomeDir()
	if err == nil {
		legacy := filepath.Join(home, ".rclone.conf")
		if _, err := os.Stat(legacy); err == nil {
			return legacy, nil
		}
	}

	// XDG standard location
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "rclone", "rclone.conf"), nil
	}

	if home != "" {
		return filepath.Join(home, ".config", "rclone", "rclone.conf"), nil
	}

	return "", fmt.Errorf("unable to determine rclone config path")
}
