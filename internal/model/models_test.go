package model_test

import (
	"testing"

	"ssh2rclone/internal/model"
)

func testModelsInstantiate(t *testing.T) {
	entry := model.SSHEntry{
		Alias:        "srv1",
		HostName:     "192.168.1.10",
		User:         "root",
		Port:         "22",
		KeyFile:      "/home/user/.ssh/id_ed25519",
		ProxyJump:    "",
		ProxyCommand: "",
		IsComplex:    false,
	}
	if entry.Alias != "srv1" {
		t.Fatalf("expected srv1, got %s", entry.Alias)
	}

	remote := model.RcloneRemote{
		Name:    "srv1",
		Type:    "sftp",
		Host:    "192.168.1.10",
		User:    "root",
		Port:    "22",
		KeyFile: "/home/user/.ssh/id_ed25519",
	}
	if remote.Type != "sftp" {
		t.Fatalf("expected sftp, got %s", remote.Type)
	}

	opts := model.SyncOptions{
		DryRun:  true,
		AllSSH:  false,
		Verbose: true,
	}
	if !opts.DryRun {
		t.Fatalf("expected DryRun true")
	}
}

func TestModels(t *testing.T) {
	testModelsInstantiate(t)
}
