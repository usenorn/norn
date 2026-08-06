package job

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/usenorn/norn/internal/service"
)

type ImportRescueHandler struct {
	imports service.ImportRescue
}

func NewImportRescueHandler(imports service.ImportRescue) *ImportRescueHandler {
	return &ImportRescueHandler{imports: imports}
}

func (h *ImportRescueHandler) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	if _, err := h.imports.Rescue(ctx); err != nil {
		return err
	}

	_, err := h.imports.Sweep(ctx)

	return err
}
