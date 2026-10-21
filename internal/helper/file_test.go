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

func TestOsFileHelper(t *testing.T) {
	fh := helper.NewOsFileHelper()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	err := fh.AtomicWriteFile(filePath, []byte("hello world"), 0600)
	if err != nil {
		t.Fatalf("AtomicWriteFile failed: %v", err)
	}

	if !fh.FileExists(filePath) {
		t.Fatalf("expected file to exist")
	}

	data, err := fh.ReadFile(filePath)
	if err != nil || string(data) != "hello world" {
		t.Fatalf("ReadFile failed: got %q, err %v", string(data), err)
	}

	bak, err := fh.CreateBackup(filePath)
	if err != nil || bak == "" {
		t.Fatalf("CreateBackup failed: %v", err)
	}
}

func TestMockFileHelper(t *testing.T) {
	mf := helper.NewMockFileHelper()
	mf.Files["/etc/rclone.conf"] = []byte("[test]\ntype = sftp\n")

	if !mf.FileExists("/etc/rclone.conf") {
		t.Fatalf("expected /etc/rclone.conf to exist in mock")
	}

	data, err := mf.ReadFile("/etc/rclone.conf")
	if err != nil || string(data) != "[test]\ntype = sftp\n" {
		t.Fatalf("unexpected content: %s", string(data))
	}

	err = mf.AtomicWriteFile("/etc/rclone.conf", []byte("[updated]\n"), 0600)
	if err != nil || string(mf.Files["/etc/rclone.conf"]) != "[updated]\n" {
		t.Fatalf("mock AtomicWriteFile failed")
	}
}

