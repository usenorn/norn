package job

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/usenorn/norn/internal/service"
)

type ExecutionUploadSweepHandler struct {
	uploads service.ExecutionUploads
}

func NewExecutionUploadSweepHandler(uploads service.ExecutionUploads) *ExecutionUploadSweepHandler {
	return &ExecutionUploadSweepHandler{uploads: uploads}
}

func (h *ExecutionUploadSweepHandler) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	return h.uploads.Prune(ctx)
}
