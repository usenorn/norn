package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/usenorn/norn/internal"
)

func newJobsCommand() *cobra.Command {
	jobs := &cobra.Command{
		Use:   "jobs",
		Short: "Inspect and operate the background job queues",
	}

	jobs.AddCommand(
		newJobsQueuesCommand(),
		newJobsListCommand(),
		newJobsRunCommand(),
		newJobsArchiveCommand(),
		newJobsDeleteCommand(),
	)

	return jobs
}

func withJobsAdmin(cmd *cobra.Command, run func(context.Context, *internal.JobsAdmin) error) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	admin, cleanup, err := internal.InitJobsAdmin(cfgFile)
	if err != nil {
		return err
	}
	defer cleanup()

	return run(ctx, admin)
}

func newJobsQueuesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "queues",
		Short: "Report queue depths",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withJobsAdmin(cmd, func(ctx context.Context, admin *internal.JobsAdmin) error {
				return admin.Queues(ctx)
			})
		},
	}
}

func newJobsListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list <queue> <state>",
		Short: "List tasks in a queue and state",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withJobsAdmin(cmd, func(ctx context.Context, admin *internal.JobsAdmin) error {
				return admin.List(ctx, args[0], args[1])
			})
		},
	}
}

func newJobsRunCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "run <queue> <task-id>",
		Short: "Schedule a task to run immediately",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withJobsAdmin(cmd, func(ctx context.Context, admin *internal.JobsAdmin) error {
				return admin.Run(ctx, args[0], args[1])
			})
		},
	}
}

func newJobsArchiveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "archive <queue> <task-id>",
		Short: "Move a task to the dead-letter queue",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withJobsAdmin(cmd, func(ctx context.Context, admin *internal.JobsAdmin) error {
				return admin.Archive(ctx, args[0], args[1])
			})
		},
	}
}

func newJobsDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <queue> <task-id>",
		Short: "Delete a task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withJobsAdmin(cmd, func(ctx context.Context, admin *internal.JobsAdmin) error {
				return admin.Delete(ctx, args[0], args[1])
			})
		},
	}
}
