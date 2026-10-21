package model

// AppConfig represents the root configuration file for ssh2rclone.
type AppConfig struct {
	Version int          `yaml:"version"`
	Naming  NamingConfig `yaml:"naming"`
}

// NamingConfig defines naming rules for generated rclone remotes.
type NamingConfig struct {
	Template string `yaml:"template"`
}

// DefaultAppConfig returns the fallback configuration when no config file is present.
func DefaultAppConfig() AppConfig {
	return AppConfig{
		Version: 1,
		Naming: NamingConfig{
			Template: "{alias}",
		},
	}
}
