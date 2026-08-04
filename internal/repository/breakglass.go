package repository

import (
	"context"

	"github.com/google/uuid"
)

//go:generate go tool mockgen -source=breakglass.go -destination=breakglass/mock_breakglass.go -package=breakglass -mock_names=BreakGlass=MockBreakGlass

type BreakGlass interface {
	Replace(ctx context.Context, workspaceID uuid.UUID, issuedBy *uuid.UUID, hashes [][]byte) error
	Redeem(ctx context.Context, workspaceID uuid.UUID, hash []byte, from string) error
	Discard(ctx context.Context, workspaceID uuid.UUID) error
	CountUnredeemed(ctx context.Context, workspaceID uuid.UUID) (int, error)
}
