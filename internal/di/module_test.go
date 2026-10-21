package di_test

import (
	"testing"

	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"ssh2rclone/internal/app"
	"ssh2rclone/internal/di"
	"ssh2rclone/internal/model"
)

func TestFxGraphResolution(t *testing.T) {
	opts := model.SyncOptions{
		SSHConfigPath:    "/tmp/test_ssh",
		RcloneConfigPath: "/tmp/test_rclone",
		DryRun:           true,
	}

	testApp := fxtest.New(
		t,
		di.Module,
		fx.Provide(func() model.SyncOptions {
			return opts
		}),
		fx.Invoke(func(runner *app.App) {
			if runner == nil {
				t.Fatal("expected runner to be provided")
			}
		}),
	)

	testApp.RequireStart()
	testApp.RequireStop()
}

func TestExecuteDryRun(t *testing.T) {
	tmpDir := t.TempDir()
	sshFile := tmpDir + "/ssh_config"
	rcloneFile := tmpDir + "/rclone.conf"

	opts := model.SyncOptions{
		SSHConfigPath:    sshFile,
		RcloneConfigPath: rcloneFile,
		DryRun:           true,
	}

	// Should fail cleanly if ssh config doesn't exist
	err := di.Execute(opts)
	if err == nil {
		t.Fatalf("expected error for non-existent ssh config")
	}
}
