package main

import (
	"flag"
	"fmt"
	"os"

	"ssh2rclone/internal/config"
	"ssh2rclone/internal/model"
	"ssh2rclone/internal/services"
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

	if opts.Verbose {
		fmt.Printf("Using SSH config: %s\n", opts.SSHConfigPath)
		fmt.Printf("Using rclone config: %s\n", opts.RcloneConfigPath)
		fmt.Printf("Using naming template: %s\n", opts.NameTemplate)
	}

	// Parse SSH config
	entries, err := services.ParseSSHConfig(opts.SSHConfigPath, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing SSH config: %v\n", err)
		os.Exit(1)
	}

	if opts.Verbose {
		fmt.Printf("Discovered %d concrete host(s) in SSH config\n", len(entries))
	}

	// Sync with rclone config
	res, err := services.SyncRcloneConfig(entries, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error updating rclone config: %v\n", err)
		os.Exit(1)
	}

	if opts.DryRun {
		fmt.Println("=== DRY RUN PREVIEW ===")
		fmt.Print(res.RenderedBlock)
		fmt.Printf("Summary: %d remote(s) to add, %d skipped due to external conflicts.\n",
			len(res.AddedRemotes), len(res.SkippedConflicts))
		return
	}

	for _, skipped := range res.SkippedConflicts {
		fmt.Printf("[WARN] Remote '%s' already exists in manual rclone config: skipped.\n", skipped)
	}

	if res.BackupPath != "" && opts.Verbose {
		fmt.Printf("Backup created at: %s\n", res.BackupPath)
	}

	fmt.Printf("Successfully synchronized %d remote(s) to %s\n", len(res.AddedRemotes), opts.RcloneConfigPath)
}
