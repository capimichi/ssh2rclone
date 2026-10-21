package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"ssh2rclone/internal/model"
)

// ResolveAppConfigPath locates the application configuration file.
func ResolveAppConfigPath(cliPath string) (string, error) {
	if cliPath != "" {
		return ExpandHome(cliPath)
	}

	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "ssh2rclone", "config.yaml"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "ssh2rclone", "config.yaml"), nil
}

// LoadAppConfig reads and deserializes the YAML configuration.
func LoadAppConfig(path string, isCustom bool) (model.AppConfig, error) {
	defaultCfg := model.DefaultAppConfig()

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			if isCustom {
				return defaultCfg, fmt.Errorf("config file not found at %s", path)
			}
			return defaultCfg, nil
		}
		return defaultCfg, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return defaultCfg, fmt.Errorf("failed to read config file at %s: %w", path, err)
	}

	cfg := defaultCfg
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return defaultCfg, fmt.Errorf("failed to parse yaml config at %s: %w", path, err)
	}

	if cfg.Naming.Template == "" {
		cfg.Naming.Template = "{alias}"
	}

	return cfg, nil
}
