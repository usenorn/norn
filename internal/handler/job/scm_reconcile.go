package job

import (
	"context"
	"time"

	"github.com/hibiken/asynq"

	"github.com/usenorn/norn/internal/service"
)

type SCMReconcileHandler struct {
	sync service.SourceControlSync
}

func NewSCMReconcileHandler(sync service.SourceControlSync) *SCMReconcileHandler {
	return &SCMReconcileHandler{sync: sync}
}

func (h *SCMReconcileHandler) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	return h.sync.Reconcile(ctx, time.Now().UTC())
}
