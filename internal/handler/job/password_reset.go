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

type PasswordResetHandler struct {
	accounts service.Accounts
}

func NewPasswordResetHandler(accounts service.Accounts) *PasswordResetHandler {
	return &PasswordResetHandler{accounts: accounts}
}

func (h *PasswordResetHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload entity.PasswordResetPayload

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return errors.Join(fmt.Errorf("decode password reset payload: %w", err), asynq.SkipRetry)
	}

	if payload.PasswordResetID == uuid.Nil || payload.Token == "" {
		return errors.Join(errors.New("password reset payload is incomplete"), asynq.SkipRetry)
	}

	return h.accounts.SendPasswordReset(ctx, payload.PasswordResetID, payload.Token)
}
