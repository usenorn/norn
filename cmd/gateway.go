package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/usenorn/norn/internal"
)

func newGatewayCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "gateway",
		Short: "Serve preview traffic and carry it down the machines' tunnels",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			gateway, err := internal.InitGateway(cfgFile)
			if err != nil {
				return err
			}

			return gateway.Run(ctx)
		},
	}
}
