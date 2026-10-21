package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"ssh2rclone/internal/config"
)

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("could not get home dir: %v", err)
	}

	got, err := config.ExpandHome("~/.ssh/config")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(home, ".ssh/config")
	if got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}

	plain, err := config.ExpandHome("/var/log")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plain != "/var/log" {
		t.Fatalf("expected /var/log, got %s", plain)
	}
}

func TestResolveSSHConfigPath(t *testing.T) {
	custom := "/custom/path/ssh_config"
	got, err := config.ResolveSSHConfigPath(custom)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != custom {
		t.Fatalf("expected %s, got %s", custom, got)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("could not get home dir: %v", err)
	}
	def, err := config.ResolveSSHConfigPath("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(home, ".ssh/config")
	if def != expected {
		t.Fatalf("expected %s, got %s", expected, def)
	}
}

func TestResolveRcloneConfigPathCLIOverride(t *testing.T) {
	custom := "/tmp/my_rclone.conf"
	got, err := config.ResolveRcloneConfigPath(custom)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != custom {
		t.Fatalf("expected %s, got %s", custom, got)
	}
}

func TestResolveRcloneConfigPathEnv(t *testing.T) {
	t.Setenv("RCLONE_CONFIG", "/tmp/env_rclone.conf")
	got, err := config.ResolveRcloneConfigPath("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/tmp/env_rclone.conf" {
		t.Fatalf("expected /tmp/env_rclone.conf, got %s", got)
	}
}
