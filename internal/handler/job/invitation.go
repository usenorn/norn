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

type InvitationHandler struct {
	invitations service.Invitations
}

func NewInvitationHandler(invitations service.Invitations) *InvitationHandler {
	return &InvitationHandler{invitations: invitations}
}

func (h *InvitationHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload entity.InvitationPayload

	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return errors.Join(fmt.Errorf("decode invitation payload: %w", err), asynq.SkipRetry)
	}

	if payload.InvitationID == uuid.Nil || payload.Token == "" {
		return errors.Join(errors.New("invitation payload is incomplete"), asynq.SkipRetry)
	}

	return h.invitations.SendInvitation(ctx, payload.InvitationID, payload.Token)
}
