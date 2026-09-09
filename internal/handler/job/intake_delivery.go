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

type IntakeDeliveryHandler struct {
	intakes service.Intakes
}

func NewIntakeDeliveryHandler(intakes service.Intakes) *IntakeDeliveryHandler {
	return &IntakeDeliveryHandler{intakes: intakes}
}

func (h *IntakeDeliveryHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload entity.IntakeDeliveryPayload

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return errors.Join(fmt.Errorf("decode intake delivery payload: %w", err), asynq.SkipRetry)
	}

	if payload.DeliveryID == uuid.Nil {
		return errors.Join(errors.New("intake delivery payload is incomplete"), asynq.SkipRetry)
	}

	return h.intakes.Apply(ctx, payload.DeliveryID)
}
