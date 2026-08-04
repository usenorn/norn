package internal

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/job"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/taskqueue"
)

type Worker struct {
	server *taskqueue.Server
	mux    *asynq.ServeMux
	logger *slog.Logger
}

func NewServeMux(
	signUpVerification *job.SignUpVerificationHandler,
	emailChangeConfirmation *job.EmailChangeConfirmationHandler,
	passwordReset *job.PasswordResetHandler,
	passwordResetSSONotice *job.PasswordResetSSONoticeHandler,
	invitation *job.InvitationHandler,
	workspacePurge *job.WorkspacePurgeHandler,
	issuePurge *job.IssuePurgeHandler,
	bulkApply *job.BulkApplyHandler,
) *asynq.ServeMux {
	mux := asynq.NewServeMux()
	mux.Handle(entity.TaskTypeSignUpVerification, signUpVerification)
	mux.Handle(entity.TaskTypeEmailChangeConfirmation, emailChangeConfirmation)
	mux.Handle(entity.TaskTypePasswordReset, passwordReset)
	mux.Handle(entity.TaskTypePasswordResetSSONotice, passwordResetSSONotice)
	mux.Handle(entity.TaskTypeInvitation, invitation)
	mux.Handle(entity.TaskTypeWorkspacePurge, workspacePurge)
	mux.Handle(entity.TaskTypeIssuePurge, issuePurge)
	mux.Handle(entity.TaskTypeBulkApply, bulkApply)

	return mux
}

func NewWorker(server *taskqueue.Server, mux *asynq.ServeMux, logger *slog.Logger) *Worker {
	return &Worker{server: server, mux: mux, logger: logger}
}

func (w *Worker) Run(ctx context.Context) error {
	ctx = logging.Into(ctx, w.logger)

	if err := w.server.Start(w.mux); err != nil {
		return fmt.Errorf("start worker: %w", err)
	}

	logging.From(ctx).InfoContext(ctx, "worker started")

	<-ctx.Done()

	logging.From(ctx).InfoContext(ctx, "worker draining")
	w.server.Shutdown()

	return nil
}
