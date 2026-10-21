package main

import (
	"flag"
	"fmt"
	"os"

	"ssh2rclone/internal/config"
	"ssh2rclone/internal/di"
	"ssh2rclone/internal/model"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var opts model.SyncOptions
	var showVersion bool

	flag.BoolVar(&showVersion, "version", false, "Print version information and exit")
	flag.StringVar(&opts.SSHConfigPath, "ssh-config", "", "Path to SSH config file (default: ~/.ssh/config)")
	flag.StringVar(&opts.RcloneConfigPath, "rclone-config", "", "Path to rclone.conf (default: auto-detected via env/binary/XDG)")
	flag.StringVar(&opts.ConfigPath, "config", "", "Path to ssh2rclone config.yaml (default: XDG config path)")
	flag.StringVar(&opts.NameTemplate, "name-template", "", "Naming template for generated remotes (e.g. '{user:-root}_{alias}')")
	flag.BoolVar(&opts.DryRun, "dry-run", false, "Preview changes without modifying rclone.conf")
	flag.BoolVar(&opts.AllSSH, "all-ssh", false, "Force OpenSSH delegation (ssh = ssh <alias>) for all hosts")
	flag.BoolVar(&opts.NoBackup, "no-backup", false, "Disable automatic backup of rclone.conf (.bak)")
	flag.BoolVar(&opts.Verbose, "v", false, "Verbose output")
	flag.BoolVar(&opts.Verbose, "verbose", false, "Verbose output")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ssh2rclone [options]\n\nOptions:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if showVersion {
		fmt.Printf("ssh2rclone version %s (commit: %s, date: %s)\n", version, commit, date)
		os.Exit(0)
	}

	// Resolve SSH config path
	sshPath, err := config.ResolveSSHConfigPath(opts.SSHConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving SSH config path: %v\n", err)
		os.Exit(1)
	}
	opts.SSHConfigPath = sshPath

	if _, err := os.Stat(opts.SSHConfigPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: SSH config file not found at %s\n", opts.SSHConfigPath)
		os.Exit(1)
	}

	// Resolve rclone config path
	rclonePath, err := config.ResolveRcloneConfigPath(opts.RcloneConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving rclone config path: %v\n", err)
		os.Exit(1)
	}
	opts.RcloneConfigPath = rclonePath

	// Resolve and load ssh2rclone application config
	appConfigPath, err := config.ResolveAppConfigPath(opts.ConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving app config path: %v\n", err)
		os.Exit(1)
	}

	appCfg, err := config.LoadAppConfig(appConfigPath, opts.ConfigPath != "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// CLI flag overrides config file
	if opts.NameTemplate != "" {
		appCfg.Naming.Template = opts.NameTemplate
	}
	opts.NameTemplate = appCfg.Naming.Template

	if err := di.Execute(opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
