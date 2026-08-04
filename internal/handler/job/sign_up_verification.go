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

type SignUpVerificationHandler struct {
	accounts service.Accounts
}

func NewSignUpVerificationHandler(accounts service.Accounts) *SignUpVerificationHandler {
	return &SignUpVerificationHandler{accounts: accounts}
}

func (h *SignUpVerificationHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload entity.SignUpVerificationPayload

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return errors.Join(fmt.Errorf("decode sign-up verification payload: %w", err), asynq.SkipRetry)
	}

	if payload.SignUpID == uuid.Nil || payload.Token == "" {
		return errors.Join(errors.New("sign-up verification payload is incomplete"), asynq.SkipRetry)
	}

	return h.accounts.SendSignUpVerification(ctx, payload.SignUpID, payload.Token)
}
