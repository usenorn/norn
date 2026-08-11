package job

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/usenorn/norn/internal/service"
)

type CheckExpirySweepHandler struct {
	checks service.Checks
}

func NewCheckExpirySweepHandler(checks service.Checks) *CheckExpirySweepHandler {
	return &CheckExpirySweepHandler{checks: checks}
}

func (h *CheckExpirySweepHandler) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	return h.checks.SweepExpiry(ctx)
}
