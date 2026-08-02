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

type EmailChangeConfirmationHandler struct {
	accounts service.Accounts
}

func NewEmailChangeConfirmationHandler(accounts service.Accounts) *EmailChangeConfirmationHandler {
	return &EmailChangeConfirmationHandler{accounts: accounts}
}

func (h *EmailChangeConfirmationHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload entity.EmailChangeConfirmationPayload

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return errors.Join(fmt.Errorf("decode email change confirmation payload: %w", err), asynq.SkipRetry)
	}

	if payload.EmailChangeID == uuid.Nil || payload.Token == "" {
		return errors.Join(errors.New("email change confirmation payload is incomplete"), asynq.SkipRetry)
	}

	return h.accounts.SendEmailChangeConfirmation(ctx, payload.EmailChangeID, payload.Token)
}
