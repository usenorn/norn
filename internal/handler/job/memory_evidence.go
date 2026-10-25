package job

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type MemoryEvidenceCheckHandler struct {
	checks service.MemoryEvidenceChecks
}

func NewMemoryEvidenceCheckHandler(checks service.MemoryEvidenceChecks) *MemoryEvidenceCheckHandler {
	return &MemoryEvidenceCheckHandler{checks: checks}
}

func (h *MemoryEvidenceCheckHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload entity.MemoryEvidenceCheckPayload

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return errors.Join(fmt.Errorf("decode memory evidence check payload: %w", err), asynq.SkipRetry)
	}

	if payload.WorkspaceID == uuid.Nil {
		return errors.Join(errors.New("memory evidence check payload is incomplete"), asynq.SkipRetry)
	}

	return h.checks.Check(ctx, payload.WorkspaceID)
}

type MemoryEvidenceSweepHandler struct {
	checks service.MemoryEvidenceChecks
}

func NewMemoryEvidenceSweepHandler(checks service.MemoryEvidenceChecks) *MemoryEvidenceSweepHandler {
	return &MemoryEvidenceSweepHandler{checks: checks}
}

func (h *MemoryEvidenceSweepHandler) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	return h.checks.Sweep(ctx)
}
