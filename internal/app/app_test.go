package app_test

import (
	"context"
	"testing"

	"ssh2rclone/internal/app"
	"ssh2rclone/internal/helper"
	"ssh2rclone/internal/model"
	"ssh2rclone/internal/services"
)

type mockSSHParser struct {
	entries []model.SSHEntry
	err     error
}

func (m *mockSSHParser) Parse(path string, opts model.SyncOptions) ([]model.SSHEntry, error) {
	return m.entries, m.err
}

func TestAppRunDryRun(t *testing.T) {
	mockParser := &mockSSHParser{
		entries: []model.SSHEntry{
			{Alias: "srv", HostName: "1.2.3.4"},
		},
	}
	mockHelper := helper.NewMockFileHelper()
	syncer := services.NewRcloneSyncer(mockHelper, services.NewRemoteNameFormatter())

	opts := model.SyncOptions{
		SSHConfigPath:    "/tmp/ssh_config",
		RcloneConfigPath: "/tmp/rclone.conf",
		DryRun:           true,
	}

	application := app.NewApp(mockParser, syncer, opts)
	err := application.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAppRunRealSync(t *testing.T) {
	mockParser := &mockSSHParser{
		entries: []model.SSHEntry{
			{Alias: "web", HostName: "10.0.0.1", User: "deploy"},
		},
	}
	mockHelper := helper.NewMockFileHelper()
	syncer := services.NewRcloneSyncer(mockHelper, services.NewRemoteNameFormatter())

	opts := model.SyncOptions{
		SSHConfigPath:    "/tmp/ssh_config",
		RcloneConfigPath: "/tmp/rclone.conf",
		DryRun:           false,
		NoBackup:         true,
	}

	application := app.NewApp(mockParser, syncer, opts)
	err := application.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error in real sync: %v", err)
	}

	if !mockHelper.FileExists("/tmp/rclone.conf") {
		t.Fatalf("expected /tmp/rclone.conf to be written")
	}
}
