package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/usenorn/norn/internal"
)

func newWorkerCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "worker",
		Short: "Process background jobs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			worker, cleanup, err := internal.InitWorker(cfgFile)
			if err != nil {
				return err
			}
			defer cleanup()

			return worker.Run(ctx)
		},
	}
}
