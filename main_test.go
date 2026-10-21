package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ssh2rclone/internal/model"
	"ssh2rclone/internal/services"
)

func TestEndToEndSync(t *testing.T) {
	tmpDir := t.TempDir()
	sshPath := filepath.Join(tmpDir, "ssh_config")
	rclonePath := filepath.Join(tmpDir, "rclone.conf")

	sshContent := `
Host web-server
    HostName 192.168.1.100
    User dev
    Port 2202

Host db-proxy
    HostName 10.0.0.50
    ProxyJump bastion.example.com
`
	if err := os.WriteFile(sshPath, []byte(sshContent), 0600); err != nil {
		t.Fatalf("failed to write ssh config: %v", err)
	}

	opts := model.SyncOptions{
		SSHConfigPath:    sshPath,
		RcloneConfigPath: rclonePath,
		DryRun:           false,
		NoBackup:         false,
		Verbose:          false,
	}

	entries, err := services.ParseSSHConfig(sshPath, opts)
	if err != nil {
		t.Fatalf("failed to parse ssh config: %v", err)
	}

	res, err := services.SyncRcloneConfig(entries, opts)
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	if len(res.AddedRemotes) != 2 {
		t.Fatalf("expected 2 remotes added, got %d", len(res.AddedRemotes))
	}

	// Verify content of generated rclone.conf
	data, err := os.ReadFile(rclonePath)
	if err != nil {
		t.Fatalf("failed reading rclone config: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "[web-server]\ntype = sftp\nhost = 192.168.1.100\nuser = dev\nport = 2202") {
		t.Fatalf("web-server entry missing or wrong: %s", content)
	}
	if !strings.Contains(content, "[db-proxy]\ntype = sftp\nssh = ssh db-proxy") {
		t.Fatalf("db-proxy entry missing or wrong: %s", content)
	}
}
