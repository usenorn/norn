package job

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/service"
)

type TelegramUpdateHandler struct {
	updates service.TelegramUpdates
}

func NewTelegramUpdateHandler(updates service.TelegramUpdates) *TelegramUpdateHandler {
	return &TelegramUpdateHandler{updates: updates}
}

func (h *TelegramUpdateHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload entity.TelegramUpdatePayload

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return errors.Join(fmt.Errorf("decode telegram update payload: %w", err), asynq.SkipRetry)
	}

	if payload.UpdateID == uuid.Nil {
		return errors.Join(errors.New("telegram update payload is incomplete"), asynq.SkipRetry)
	}

	return h.updates.Apply(ctx, payload.UpdateID)
}

type TelegramDecisionHandler struct {
	updates service.TelegramUpdates
}

func NewTelegramDecisionHandler(updates service.TelegramUpdates) *TelegramDecisionHandler {
	return &TelegramDecisionHandler{updates: updates}
}

func (h *TelegramDecisionHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	decision, err := telegramDecision(task)
	if err != nil {
		return err
	}

	return h.updates.Relay(ctx, decision)
}

type TelegramSettlementHandler struct {
	updates service.TelegramUpdates
}

func NewTelegramSettlementHandler(updates service.TelegramUpdates) *TelegramSettlementHandler {
	return &TelegramSettlementHandler{updates: updates}
}

func (h *TelegramSettlementHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	decision, err := telegramDecision(task)
	if err != nil {
		return err
	}

	return h.updates.Settle(ctx, decision)
}

type TelegramSweepHandler struct {
	updates service.TelegramUpdates
}

func NewTelegramSweepHandler(updates service.TelegramUpdates) *TelegramSweepHandler {
	return &TelegramSweepHandler{updates: updates}
}

func (h *TelegramSweepHandler) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	swept, err := h.updates.Sweep(ctx)
	if err != nil {
		return err
	}

	logging.From(ctx).InfoContext(ctx, "telegram updates swept", "swept", swept)

	return nil
}

func telegramDecision(task *asynq.Task) (entity.TelegramDecision, error) {
	var decision entity.TelegramDecision

	if err := json.Unmarshal(task.Payload(), &decision); err != nil {
		return decision, errors.Join(fmt.Errorf("decode telegram decision payload: %w", err), asynq.SkipRetry)
	}

	if !decision.Complete() {
		return decision, errors.Join(errors.New("telegram decision payload is incomplete"), asynq.SkipRetry)
	}

	return decision, nil
}
