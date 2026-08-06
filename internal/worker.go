package internal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/hibiken/asynq"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/job"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/taskqueue"
)

type Worker struct {
	cfg           config.Worker
	saml          config.SAML
	cycles        config.Cycles
	attachments   config.Attachments
	notifications config.Notifications
	tokens        config.APITokens
	audit         config.Audit
	webhooks      config.Webhooks
	imports       config.Imports
	server        *taskqueue.Server
	scheduler     *taskqueue.Scheduler
	inspector     *taskqueue.Inspector
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
	auditSweep *job.AuditSweepHandler,
	webhookFanOut *job.WebhookFanOutHandler,
	webhookDeliver *job.WebhookDeliverHandler,
	webhookSweep *job.WebhookSweepHandler,
	importStage *job.ImportStageHandler,
	importExecute *job.ImportExecuteHandler,
	importRevert *job.ImportRevertHandler,
	importRescue *job.ImportRescueHandler,
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
	mux.Handle(entity.TaskTypeAuditSweep, auditSweep)
	mux.Handle(entity.TaskTypeWebhookFanOut, webhookFanOut)
	mux.Handle(entity.TaskTypeWebhookDeliver, webhookDeliver)
	mux.Handle(entity.TaskTypeWebhookSweep, webhookSweep)
	mux.Handle(entity.TaskTypeImportStage, importStage)
	mux.Handle(entity.TaskTypeImportExecute, importExecute)
	mux.Handle(entity.TaskTypeImportRevert, importRevert)
	mux.Handle(entity.TaskTypeImportRescue, importRescue)

	return mux
}

func NewWorker(
	cfg config.Worker,
	saml config.SAML,
	cycles config.Cycles,
	attachments config.Attachments,
	notifications config.Notifications,
	tokens config.APITokens,
	audit config.Audit,
	webhooks config.Webhooks,
	imports config.Imports,
	server *taskqueue.Server,
	scheduler *taskqueue.Scheduler,
	inspector *taskqueue.Inspector,
	mux *asynq.ServeMux,
	logger *slog.Logger,
) *Worker {
	return &Worker{
		cfg:           cfg,
		saml:          saml,
		cycles:        cycles,
		attachments:   attachments,
		notifications: notifications,
		tokens:        tokens,
		audit:         audit,
		webhooks:      webhooks,
		imports:       imports,
		server:        server,
		scheduler:     scheduler,
		inspector:     inspector,
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

	if _, err := w.scheduler.Register(
		w.audit.SweepSchedule,
		asynq.NewTask(entity.TaskTypeAuditSweep, nil),
		asynq.Queue(entity.QueueMail),
	); err != nil {
		return fmt.Errorf("register audit retention sweep: %w", err)
	}

	if _, err := w.scheduler.Register(
		w.webhooks.FanOutSchedule,
		asynq.NewTask(entity.TaskTypeWebhookFanOut, nil),
		asynq.Queue(entity.QueueWebhook),
	); err != nil {
		return fmt.Errorf("register webhook fan-out: %w", err)
	}

	if _, err := w.scheduler.Register(
		w.webhooks.SweepSchedule,
		asynq.NewTask(entity.TaskTypeWebhookSweep, nil),
		asynq.Queue(entity.QueueWebhook),
	); err != nil {
		return fmt.Errorf("register webhook retention sweep: %w", err)
	}

	if _, err := w.scheduler.Register(
		w.imports.RescueSchedule,
		asynq.NewTask(entity.TaskTypeImportRescue, nil),
		asynq.Queue(entity.QueueImport),
	); err != nil {
		return fmt.Errorf("register import rescue: %w", err)
	}

	if err := w.server.Start(w.mux); err != nil {
		return fmt.Errorf("start worker: %w", err)
	}

	if err := w.scheduler.Start(); err != nil {
		w.server.Shutdown()

		return fmt.Errorf("start scheduler: %w", err)
	}

	health := w.healthServer(ctx)
	serving := make(chan error, 1)

	go func() {
		logging.From(ctx).InfoContext(
			ctx, "worker health listening", slog.String("addr", w.cfg.HealthAddr),
		)

		if err := health.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serving <- fmt.Errorf("serve worker health: %w", err)

			return
		}

		serving <- nil
	}()

	logging.From(ctx).InfoContext(ctx, "worker started")

	var err error

	select {
	case err = <-serving:
	case <-ctx.Done():
	}

	logging.From(ctx).InfoContext(ctx, "worker draining")

	shutdownCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx), w.cfg.ShutdownTimeout,
	)
	defer cancel()

	_ = health.Shutdown(shutdownCtx)
	w.scheduler.Shutdown()
	w.server.Shutdown()

	return err
}

// The worker serves no application traffic, so this exists only so Kubernetes can tell a live
// process from a wedged one: readiness round-trips to Redis, which is the dependency that
// silently stops all background work when it stalls.
func (w *Worker) healthServer(ctx context.Context) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("GET /readyz", func(rw http.ResponseWriter, r *http.Request) {
		if _, err := w.inspector.Queues(); err != nil {
			logging.From(r.Context()).WarnContext(
				r.Context(), "worker not ready", slog.String("error", err.Error()),
			)
			rw.WriteHeader(http.StatusServiceUnavailable)

			return
		}

		rw.WriteHeader(http.StatusOK)
	})

	return &http.Server{
		Addr:              w.cfg.HealthAddr,
		Handler:           mux,
		ReadHeaderTimeout: entity.WorkerHealthReadHeaderTimeout,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
}
