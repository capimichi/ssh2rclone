package services

import (
	"fmt"
	"os"
	"strings"

	"github.com/kevinburke/ssh_config"
	"ssh2rclone/internal/config"
	"ssh2rclone/internal/model"
)

// isWildcard checks if an alias contains pattern matching characters.
func isWildcard(alias string) bool {
	return strings.ContainsAny(alias, "*?")
}

// ParseSSHConfig reads an SSH config file and extracts concrete SSHEntry records.
func ParseSSHConfig(filePath string, opts model.SyncOptions) ([]model.SSHEntry, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open ssh config at %s: %w", filePath, err)
	}
	defer f.Close()

	cfg, err := ssh_config.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ssh config: %w", err)
	}

	var entries []model.SSHEntry
	seen := make(map[string]bool)

	for _, host := range cfg.Hosts {
		for _, pattern := range host.Patterns {
			alias := pattern.String()
			if isWildcard(alias) || seen[alias] {
				continue
			}
			seen[alias] = true

			// Query resolved properties through ssh_config
			hostname, _ := cfg.Get(alias, "HostName")
			if hostname == "" {
				hostname = alias
			}

			user, _ := cfg.Get(alias, "User")
			port, _ := cfg.Get(alias, "Port")
			if port == "" {
				port = "22"
			}

			keyFile, _ := cfg.Get(alias, "IdentityFile")
			if keyFile != "" {
				expanded, err := config.ExpandHome(keyFile)
				if err == nil {
					keyFile = expanded
				}
			}

			proxyJump, _ := cfg.Get(alias, "ProxyJump")
			proxyCommand, _ := cfg.Get(alias, "ProxyCommand")

			isComplex := opts.AllSSH || proxyJump != "" || proxyCommand != ""

			entries = append(entries, model.SSHEntry{
				Alias:        alias,
				HostName:     hostname,
				User:         user,
				Port:         port,
				KeyFile:      keyFile,
				ProxyJump:    proxyJump,
				ProxyCommand: proxyCommand,
				IsComplex:    isComplex,
			})
		}
	}

	return entries, nil
}
