package repository

import (
	"context"

	"github.com/google/uuid"
)

//go:generate go tool mockgen -source=samlreplay.go -destination=samlreplay/mock_samlreplay.go -package=samlreplay -mock_names=SAMLReplay=MockSAMLReplay

type SAMLReplay interface {
	Claim(ctx context.Context, workspaceID uuid.UUID, assertionID string) (bool, error)
}
