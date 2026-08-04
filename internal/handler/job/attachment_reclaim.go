package job

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/usenorn/norn/internal/service"
)

type AttachmentReclaimHandler struct {
	attachments service.Attachments
}

func NewAttachmentReclaimHandler(attachments service.Attachments) *AttachmentReclaimHandler {
	return &AttachmentReclaimHandler{attachments: attachments}
}

func (h *AttachmentReclaimHandler) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	return h.attachments.Reclaim(ctx)
}
