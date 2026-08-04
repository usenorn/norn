package job

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/usenorn/norn/internal/service"
)

type SSOCertificateSweepHandler struct {
	connections service.SSOConnections
}

func NewSSOCertificateSweepHandler(connections service.SSOConnections) *SSOCertificateSweepHandler {
	return &SSOCertificateSweepHandler{connections: connections}
}

func (h *SSOCertificateSweepHandler) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	return h.connections.SweepCertificates(ctx)
}
