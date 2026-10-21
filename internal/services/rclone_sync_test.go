package services_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ssh2rclone/internal/model"
	"ssh2rclone/internal/services"
)

func TestExtractExternalSectionsAndIdempotence(t *testing.T) {
	initial := `[manual_remote]
type = drive
scope = full

# --- BEGIN ssh2rclone GENERATED BLOCK (DO NOT EDIT MANUALLY) ---
[old_ssh]
type = sftp
host = 1.1.1.1
# --- END ssh2rclone GENERATED BLOCK ---

[another_manual]
type = s3
`
	extSections, prefix, suffix := services.ExtractExternalSections(initial)
	if !extSections["manual_remote"] {
		t.Fatalf("expected manual_remote in external sections")
	}
	if !extSections["another_manual"] {
		t.Fatalf("expected another_manual in external sections")
	}
	if extSections["old_ssh"] {
		t.Fatalf("old_ssh is inside the managed block and should NOT be considered external")
	}

	if !strings.Contains(prefix, "[manual_remote]") {
		t.Fatalf("prefix does not contain manual_remote")
	}
	if !strings.Contains(suffix, "[another_manual]") {
		t.Fatalf("suffix does not contain another_manual")
	}
}

func TestSyncRcloneConfigSkipConflict(t *testing.T) {
	tmpDir := t.TempDir()
	rclonePath := filepath.Join(tmpDir, "rclone.conf")

	initial := `[existing_server]
type = sftp
host = manual.example.com
`
	if err := os.WriteFile(rclonePath, []byte(initial), 0600); err != nil {
		t.Fatalf("failed to write rclone conf: %v", err)
	}

	entries := []model.SSHEntry{
		{
			Alias:    "existing_server", // Conflict: should be skipped!
			HostName: "10.0.0.1",
			User:     "user1",
			Port:     "22",
		},
		{
			Alias:    "new_server",
			HostName: "10.0.0.2",
			User:     "user2",
			Port:     "22",
		},
	}

	res, err := services.SyncRcloneConfig(entries, model.SyncOptions{
		RcloneConfigPath: rclonePath,
		DryRun:           false,
		NoBackup:         false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.SkippedConflicts) != 1 || res.SkippedConflicts[0] != "existing_server" {
		t.Fatalf("expected existing_server to be skipped, got: %v", res.SkippedConflicts)
	}
	if len(res.AddedRemotes) != 1 || res.AddedRemotes[0] != "new_server" {
		t.Fatalf("expected new_server to be added, got: %v", res.AddedRemotes)
	}

	// Verify file content: manual remote preserved, new_server added in block
	updated, err := os.ReadFile(rclonePath)
	if err != nil {
		t.Fatalf("failed to read updated file: %v", err)
	}
	content := string(updated)
	if !strings.Contains(content, "[existing_server]\ntype = sftp\nhost = manual.example.com") {
		t.Fatalf("manual config was altered: %s", content)
	}
	if !strings.Contains(content, "[new_server]") {
		t.Fatalf("new_server was not added in block: %s", content)
	}

	// Verify backup file was created
	if _, err := os.Stat(rclonePath + ".bak"); err != nil {
		t.Fatalf("backup file was not created: %v", err)
	}
}

func TestRenderBlockSimpleAndComplex(t *testing.T) {
	remotes := []model.RcloneRemote{
		{
			Name:    "simple",
			Type:    "sftp",
			Host:    "simple.com",
			User:    "u1",
			Port:    "22",
			KeyFile: "/home/user/.ssh/id_rsa",
		},
		{
			Name:       "complex",
			Type:       "sftp",
			SSHCommand: "ssh complex",
		},
	}

	rendered := services.RenderBlock(remotes)
	if !strings.Contains(rendered, "[simple]\ntype = sftp\nhost = simple.com\nuser = u1\nport = 22\nkey_file = /home/user/.ssh/id_rsa") {
		t.Fatalf("simple remote rendered incorrectly: %s", rendered)
	}
	if !strings.Contains(rendered, "[complex]\ntype = sftp\nssh = ssh complex") {
		t.Fatalf("complex remote rendered incorrectly: %s", rendered)
	}
}
