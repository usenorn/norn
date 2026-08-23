package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/usenorn/norn/internal"
)

func newPreviewGatewayCommand() *cobra.Command {
	gateways := &cobra.Command{
		Use:   "preview-gateway",
		Short: "Enrol, list and revoke the gateways that serve preview traffic",
	}

	gateways.AddCommand(
		newPreviewGatewayEnrolCommand(),
		newPreviewGatewayListCommand(),
		newPreviewGatewayRevokeCommand(),
	)

	return gateways
}

func withGatewaysAdmin(
	cmd *cobra.Command,
	run func(context.Context, *internal.GatewaysAdmin) error,
) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	admin, cleanup, err := internal.InitGatewaysAdmin(cfgFile)
	if err != nil {
		return err
	}
	defer cleanup()

	return run(ctx, admin)
}

func newPreviewGatewayEnrolCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "enrol <name>",
		Short: "Create a gateway credential; its secret is printed once and never again",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withGatewaysAdmin(cmd, func(
				ctx context.Context, admin *internal.GatewaysAdmin,
			) error {
				return admin.Enrol(ctx, args[0])
			})
		},
	}
}

func newPreviewGatewayListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the gateways this instance knows",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withGatewaysAdmin(cmd, func(
				ctx context.Context, admin *internal.GatewaysAdmin,
			) error {
				return admin.List(ctx)
			})
		},
	}
}

func newPreviewGatewayRevokeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "revoke <id>",
		Short: "Revoke a gateway and shut out everything it is already holding",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withGatewaysAdmin(cmd, func(
				ctx context.Context, admin *internal.GatewaysAdmin,
			) error {
				return admin.Revoke(ctx, args[0])
			})
		},
	}
}
