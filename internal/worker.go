package internal

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/job"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/taskqueue"
)

type Worker struct {
	saml          config.SAML
	cycles        config.Cycles
	attachments   config.Attachments
	notifications config.Notifications
	tokens        config.APITokens
	server        *taskqueue.Server
	scheduler     *taskqueue.Scheduler
	mux           *asynq.ServeMux
	logger        *slog.Logger
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
	certificateSweep *job.SSOCertificateSweepHandler,
	cycleGeneration *job.CycleGenerationHandler,
	attachmentReclaim *job.AttachmentReclaimHandler,
	notificationFanOut *job.NotificationFanOutHandler,
	notificationDigest *job.NotificationDigestHandler,
	tokenExpirySweep *job.APITokenExpirySweepHandler,
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
	mux.Handle(entity.TaskTypeSSOCertificateSweep, certificateSweep)
	mux.Handle(entity.TaskTypeCycleGeneration, cycleGeneration)
	mux.Handle(entity.TaskTypeAttachmentReclaim, attachmentReclaim)
	mux.Handle(entity.TaskTypeNotificationFanOut, notificationFanOut)
	mux.Handle(entity.TaskTypeNotificationDigest, notificationDigest)
	mux.Handle(entity.TaskTypeAPITokenExpirySweep, tokenExpirySweep)

	return mux
}

func NewWorker(
	saml config.SAML,
	cycles config.Cycles,
	attachments config.Attachments,
	notifications config.Notifications,
	tokens config.APITokens,
	server *taskqueue.Server,
	scheduler *taskqueue.Scheduler,
	mux *asynq.ServeMux,
	logger *slog.Logger,
) *Worker {
	return &Worker{
		saml:          saml,
		cycles:        cycles,
		attachments:   attachments,
		notifications: notifications,
		tokens:        tokens,
		server:        server,
		scheduler:     scheduler,
		mux:           mux,
		logger:        logger,
	}
}

func (w *Worker) Run(ctx context.Context) error {
	ctx = logging.Into(ctx, w.logger)

	if _, err := w.scheduler.Register(
		w.saml.CertificateSweepSchedule,
		asynq.NewTask(entity.TaskTypeSSOCertificateSweep, nil),
		asynq.Queue(entity.QueueDefault),
	); err != nil {
		return fmt.Errorf("register certificate sweep: %w", err)
	}

	if _, err := w.scheduler.Register(
		w.cycles.GenerationSchedule,
		asynq.NewTask(entity.TaskTypeCycleGeneration, nil),
		asynq.Queue(entity.QueueDefault),
	); err != nil {
		return fmt.Errorf("register cycle generation: %w", err)
	}

	if _, err := w.scheduler.Register(
		w.attachments.ReclaimSchedule,
		asynq.NewTask(entity.TaskTypeAttachmentReclaim, nil),
		asynq.Queue(entity.QueueDefault),
	); err != nil {
		return fmt.Errorf("register attachment reclaim: %w", err)
	}

	if _, err := w.scheduler.Register(
		w.notifications.FanOutSchedule,
		asynq.NewTask(entity.TaskTypeNotificationFanOut, nil),
		asynq.Queue(entity.QueueDefault),
	); err != nil {
		return fmt.Errorf("register notification fan-out: %w", err)
	}

	if _, err := w.scheduler.Register(
		w.notifications.DigestSchedule,
		asynq.NewTask(entity.TaskTypeNotificationDigest, nil),
		asynq.Queue(entity.QueueMail),
	); err != nil {
		return fmt.Errorf("register notification digest: %w", err)
	}

	if _, err := w.scheduler.Register(
		w.tokens.ExpirySweepSchedule,
		asynq.NewTask(entity.TaskTypeAPITokenExpirySweep, nil),
		asynq.Queue(entity.QueueMail),
	); err != nil {
		return fmt.Errorf("register api token expiry sweep: %w", err)
	}

	if err := w.server.Start(w.mux); err != nil {
		return fmt.Errorf("start worker: %w", err)
	}

	if err := w.scheduler.Start(); err != nil {
		w.server.Shutdown()

		return fmt.Errorf("start scheduler: %w", err)
	}

	logging.From(ctx).InfoContext(ctx, "worker started")

	<-ctx.Done()

	logging.From(ctx).InfoContext(ctx, "worker draining")
	w.scheduler.Shutdown()
	w.server.Shutdown()

	return nil
}
