package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveAppConfigPath(t *testing.T) {
	// Custom CLI path
	path, err := ResolveAppConfigPath("/tmp/custom.yaml")
	if err != nil || path != "/tmp/custom.yaml" {
		t.Fatalf("expected /tmp/custom.yaml, got %s (err: %v)", path, err)
	}

	// XDG_CONFIG_HOME
	t.Setenv("XDG_CONFIG_HOME", "/custom/xdg")
	path, err = ResolveAppConfigPath("")
	if err != nil || path != "/custom/xdg/ssh2rclone/config.yaml" {
		t.Fatalf("expected /custom/xdg/ssh2rclone/config.yaml, got %s (err: %v)", path, err)
	}
}

func TestLoadAppConfig_DefaultWhenMissing(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "does-not-exist.yaml")

	cfg, err := LoadAppConfig(nonExistent, false)
	if err != nil {
		t.Fatalf("unexpected error loading missing default config: %v", err)
	}
	if cfg.Naming.Template != "{alias}" {
		t.Fatalf("expected default template {alias}, got %s", cfg.Naming.Template)
	}
}

func TestLoadAppConfig_ErrorWhenCustomMissing(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "missing-custom.yaml")

	_, err := LoadAppConfig(nonExistent, true)
	if err == nil {
		t.Fatalf("expected error when custom config file is missing, got nil")
	}
}

func TestLoadAppConfig_ValidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "config.yaml")
	content := []byte("version: 1\nnaming:\n  template: \"{user:-root}_{alias}\"\n")
	if err := os.WriteFile(yamlFile, content, 0o600); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	cfg, err := LoadAppConfig(yamlFile, false)
	if err != nil {
		t.Fatalf("failed to load valid config: %v", err)
	}
	if cfg.Naming.Template != "{user:-root}_{alias}" {
		t.Fatalf("expected {user:-root}_{alias}, got %s", cfg.Naming.Template)
	}
}
