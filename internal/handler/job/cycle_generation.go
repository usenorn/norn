package job

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/usenorn/norn/internal/service"
)

type CycleGenerationHandler struct {
	cycles service.Cycles
}

func NewCycleGenerationHandler(cycles service.Cycles) *CycleGenerationHandler {
	return &CycleGenerationHandler{cycles: cycles}
}

func (h *CycleGenerationHandler) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	return h.cycles.Generate(ctx)
}
