package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=intake.go -destination=intake/mock_intake.go -package=intake -mock_names=Intake=MockIntake

type Intake interface {
	Address(ctx context.Context, workspaceID, teamID uuid.UUID) (entity.IntakeAddress, error)
	AddressOf(ctx context.Context, localPart, domain string) (entity.IntakeAddress, error)
	Save(ctx context.Context, address entity.IntakeAddress) (entity.IntakeAddress, error)
	Disable(ctx context.Context, workspaceID, teamID uuid.UUID) error
	Record(ctx context.Context, delivery entity.IntakeDelivery) (uuid.UUID, error)
	Delivery(ctx context.Context, deliveryID uuid.UUID) (entity.IntakeDelivery, error)
	Settle(
		ctx context.Context,
		deliveryID uuid.UUID,
		outcome entity.IntakeDeliveryOutcome,
		issueID uuid.UUID,
		failure string,
		at time.Time,
	) error
}
