package di

import (
	"context"
	"os"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"ssh2rclone/internal/app"
	"ssh2rclone/internal/helper"
	"ssh2rclone/internal/model"
	"ssh2rclone/internal/services"
)

// Module encapsulates all core services and helpers as Uber Fx providers.
var Module = fx.Options(
	fx.Provide(
		helper.NewOsFileHelper,
		services.NewRemoteNameFormatter,
		services.NewSSHParser,
		services.NewRcloneSyncer,
		app.NewApp,
	),
)

// Execute boots the Uber Fx container and executes the application workflow.
func Execute(opts model.SyncOptions) error {
	var runErr error

	fxApp := fx.New(
		Module,
		fx.Provide(func() model.SyncOptions {
			return opts
		}),
		fx.WithLogger(func() fxevent.Logger {
			if opts.Verbose {
				return &fxevent.ConsoleLogger{W: os.Stderr}
			}
			return fxevent.NopLogger
		}),
		fx.Invoke(func(application *app.App, lc fx.Lifecycle, shutdowner fx.Shutdowner) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					defer func() {
						_ = shutdowner.Shutdown()
					}()
					runErr = application.Run(ctx)
					return runErr
				},
			})
		}),
	)

	if err := fxApp.Start(context.Background()); err != nil {
		return err
	}
	<-fxApp.Done()
	_ = fxApp.Stop(context.Background())

	return runErr
}
