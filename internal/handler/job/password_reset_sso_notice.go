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

type PasswordResetSSONoticeHandler struct {
	accounts service.Accounts
}

func NewPasswordResetSSONoticeHandler(accounts service.Accounts) *PasswordResetSSONoticeHandler {
	return &PasswordResetSSONoticeHandler{accounts: accounts}
}

func (h *PasswordResetSSONoticeHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload entity.PasswordResetSSONoticePayload

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return errors.Join(fmt.Errorf("decode password reset sso notice payload: %w", err), asynq.SkipRetry)
	}

	if payload.AccountID == uuid.Nil {
		return errors.Join(errors.New("password reset sso notice payload is incomplete"), asynq.SkipRetry)
	}

	return h.accounts.SendPasswordResetSSONotice(ctx, payload.AccountID)
}
