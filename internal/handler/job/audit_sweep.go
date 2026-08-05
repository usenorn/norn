package job

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/usenorn/norn/internal/service"
)

type AuditSweepHandler struct {
	retention service.AuditRetention
}

func NewAuditSweepHandler(retention service.AuditRetention) *AuditSweepHandler {
	return &AuditSweepHandler{retention: retention}
}

func (h *AuditSweepHandler) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	_, err := h.retention.Sweep(ctx)

	return err
}
