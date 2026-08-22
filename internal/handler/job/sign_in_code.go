package job

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type SignInCodeHandler struct {
	sessions service.Sessions
}

func NewSignInCodeHandler(sessions service.Sessions) *SignInCodeHandler {
	return &SignInCodeHandler{sessions: sessions}
}

func (h *SignInCodeHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload entity.SignInCodePayload

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return errors.Join(fmt.Errorf("decode sign-in code payload: %w", err), asynq.SkipRetry)
	}

	if payload.ChallengeID == "" || payload.Code == "" {
		return errors.Join(errors.New("sign-in code payload is incomplete"), asynq.SkipRetry)
	}

	return h.sessions.SendSignInCode(ctx, payload.ChallengeID, payload.Code)
}
