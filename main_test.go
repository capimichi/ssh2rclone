package main

import (
	"os"
	"os/exec"
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

func TestVersionFlag(t *testing.T) {
	cmd := exec.Command("go", "run", "main.go", "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected -version to exit with 0, got error: %v, output: %s", err, string(output))
	}
	outStr := string(output)
	if !strings.Contains(outStr, "ssh2rclone version") {
		t.Errorf("expected output to contain 'ssh2rclone version', got: %s", outStr)
	}
}

func TestMain_CustomNameTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	sshConfig := filepath.Join(tmpDir, "ssh_config")
	rcloneConfig := filepath.Join(tmpDir, "rclone.conf")

	sshContent := "Host srv1\n    HostName 192.168.1.10\n    User deploy\n"
	_ = os.WriteFile(sshConfig, []byte(sshContent), 0o600)
	_ = os.WriteFile(rcloneConfig, []byte(""), 0o600)

	cmd := exec.Command("go", "run", "main.go", "--ssh-config", sshConfig, "--rclone-config", rcloneConfig, "--name-template", "{user}_{alias}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %v, output: %s", err, string(output))
	}

	data, err := os.ReadFile(rcloneConfig)
	if err != nil {
		t.Fatalf("failed reading rclone config: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "[deploy_srv1]") {
		t.Fatalf("expected [deploy_srv1] in rclone.conf, got:\n%s", content)
	}
}

func TestMain_ConfigYamlLoading(t *testing.T) {
	tmpDir := t.TempDir()
	sshConfig := filepath.Join(tmpDir, "ssh_config")
	rcloneConfig := filepath.Join(tmpDir, "rclone.conf")
	appConfig := filepath.Join(tmpDir, "config.yaml")

	sshContent := "Host srv1\n    HostName 192.168.1.10\n"
	appConfigContent := "version: 1\nnaming:\n  template: \"{user:-guest}_{alias}\"\n"

	_ = os.WriteFile(sshConfig, []byte(sshContent), 0o600)
	_ = os.WriteFile(rcloneConfig, []byte(""), 0o600)
	_ = os.WriteFile(appConfig, []byte(appConfigContent), 0o600)

	cmd := exec.Command("go", "run", "main.go", "--ssh-config", sshConfig, "--rclone-config", rcloneConfig, "--config", appConfig)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %v, output: %s", err, string(output))
	}

	data, err := os.ReadFile(rcloneConfig)
	if err != nil {
		t.Fatalf("failed reading rclone config: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "[guest_srv1]") {
		t.Fatalf("expected [guest_srv1] in rclone.conf, got:\n%s", content)
	}
}
