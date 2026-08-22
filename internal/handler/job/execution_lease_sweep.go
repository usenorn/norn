package job

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/usenorn/norn/internal/service"
)

type ExecutionLeaseSweepHandler struct {
	executions service.Executions
}

func NewExecutionLeaseSweepHandler(executions service.Executions) *ExecutionLeaseSweepHandler {
	return &ExecutionLeaseSweepHandler{executions: executions}
}

func (h *ExecutionLeaseSweepHandler) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	return h.executions.SweepLeases(ctx)
}
