package app

import (
	"context"
	"fmt"
	"os"

	"ssh2rclone/internal/model"
	"ssh2rclone/internal/services"
)

// Runner coordinates the synchronization process.
type Runner interface {
	Run(ctx context.Context) error
}

// App encapsulates the core application execution.
type App struct {
	parser services.SSHParser
	syncer services.RcloneSyncer
	opts   model.SyncOptions
}

// NewApp creates a new App with injected dependencies.
func NewApp(parser services.SSHParser, syncer services.RcloneSyncer, opts model.SyncOptions) *App {
	return &App{
		parser: parser,
		syncer: syncer,
		opts:   opts,
	}
}

// Run executes the full synchronization workflow.
func (a *App) Run(ctx context.Context) error {
	if a.opts.Verbose {
		fmt.Printf("Using SSH config: %s\n", a.opts.SSHConfigPath)
		fmt.Printf("Using rclone config: %s\n", a.opts.RcloneConfigPath)
		fmt.Printf("Using naming template: %s\n", a.opts.NameTemplate)
	}

	entries, err := a.parser.Parse(a.opts.SSHConfigPath, a.opts)
	if err != nil {
		return fmt.Errorf("parsing SSH config: %w", err)
	}

	if a.opts.Verbose {
		fmt.Printf("Discovered %d concrete host(s) in SSH config\n", len(entries))
	}

	res, err := a.syncer.Sync(entries, a.opts)
	if err != nil {
		return fmt.Errorf("updating rclone config: %w", err)
	}

	if a.opts.DryRun {
		fmt.Println("=== DRY RUN PREVIEW ===")
		fmt.Print(res.RenderedBlock)
		fmt.Printf("Summary: %d remote(s) to add, %d skipped due to external conflicts.\n",
			len(res.AddedRemotes), len(res.SkippedConflicts))
		return nil
	}

	for _, skipped := range res.SkippedConflicts {
		fmt.Fprintf(os.Stderr, "[WARN] Remote '%s' already exists in manual rclone config: skipped.\n", skipped)
	}

	if res.BackupPath != "" && a.opts.Verbose {
		fmt.Printf("Backup created at: %s\n", res.BackupPath)
	}

	fmt.Printf("Successfully synchronized %d remote(s) to %s\n", len(res.AddedRemotes), a.opts.RcloneConfigPath)
	return nil
}
