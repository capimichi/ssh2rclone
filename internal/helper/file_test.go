package helper_test

import (
	"os"
	"path/filepath"
	"testing"

	"ssh2rclone/internal/helper"
)

func TestAtomicWriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "testfile.conf")

	content := []byte("[test]\ntype = sftp\n")
	if err := helper.AtomicWriteFile(target, content, 0600); err != nil {
		t.Fatalf("failed atomic write: %v", err)
	}

	read, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("failed reading target file: %v", err)
	}
	if string(read) != string(content) {
		t.Fatalf("expected %q, got %q", string(content), string(read))
	}
}

func TestCreateBackup(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "rclone.conf")

	content := []byte("[existing]\ntype = drive\n")
	if err := os.WriteFile(target, content, 0600); err != nil {
		t.Fatalf("failed creating test file: %v", err)
	}

	bakPath, err := helper.CreateBackup(target)
	if err != nil {
		t.Fatalf("failed creating backup: %v", err)
	}
	expectedBak := target + ".bak"
	if bakPath != expectedBak {
		t.Fatalf("expected %s, got %s", expectedBak, bakPath)
	}

	bakContent, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf("failed reading backup: %v", err)
	}
	if string(bakContent) != string(content) {
		t.Fatalf("backup content mismatch")
	}
}
