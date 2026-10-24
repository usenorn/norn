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

type KnowledgeRefreshHandler struct {
	indexes service.KnowledgeIndexes
}

func NewKnowledgeRefreshHandler(indexes service.KnowledgeIndexes) *KnowledgeRefreshHandler {
	return &KnowledgeRefreshHandler{indexes: indexes}
}

func (h *KnowledgeRefreshHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload entity.KnowledgeRefreshPayload

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return errors.Join(fmt.Errorf("decode knowledge refresh payload: %w", err), asynq.SkipRetry)
	}

	if payload.WorkspaceID == uuid.Nil {
		return errors.Join(errors.New("knowledge refresh payload is incomplete"), asynq.SkipRetry)
	}

	err := h.indexes.Refresh(ctx, payload.WorkspaceID)
	if _, refused := entity.AIProviderFailureOf(err); refused {
		return errors.Join(err, asynq.SkipRetry)
	}

	return err
}

type KnowledgeSweepHandler struct {
	indexes service.KnowledgeIndexes
}

func NewKnowledgeSweepHandler(indexes service.KnowledgeIndexes) *KnowledgeSweepHandler {
	return &KnowledgeSweepHandler{indexes: indexes}
}

func (h *KnowledgeSweepHandler) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	return h.indexes.Sweep(ctx)
}
