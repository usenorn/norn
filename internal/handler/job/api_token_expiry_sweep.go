package job

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/usenorn/norn/internal/service"
)

type APITokenExpirySweepHandler struct {
	tokens service.APITokens
}

func NewAPITokenExpirySweepHandler(tokens service.APITokens) *APITokenExpirySweepHandler {
	return &APITokenExpirySweepHandler{tokens: tokens}
}

func (h *APITokenExpirySweepHandler) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	return h.tokens.SweepExpiring(ctx)
}
