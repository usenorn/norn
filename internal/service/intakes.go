package service

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=intakes.go -destination=intake/mock_intakes.go -package=intake -mock_names=Intakes=MockIntakes

type Intakes interface {
	Address(ctx context.Context, workspaceID, teamID uuid.UUID) (entity.IntakeAddress, error)
	Enable(ctx context.Context, workspaceID, teamID uuid.UUID) (entity.IntakeAddress, error)
	Rotate(ctx context.Context, workspaceID, teamID uuid.UUID) (entity.IntakeAddress, error)
	Disable(ctx context.Context, workspaceID, teamID uuid.UUID) error
	Accept(ctx context.Context, header http.Header, body []byte) (uuid.UUID, error)
	Apply(ctx context.Context, deliveryID uuid.UUID) error
}
