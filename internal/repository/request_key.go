package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=request_key.go -destination=requestkey/mock_request_key.go -package=requestkey -mock_names=RequestKey=MockRequestKey

type RequestKey interface {
	Claim(ctx context.Context, key entity.RequestKey) error
	Settle(ctx context.Context, workspaceID, keyID, issueID uuid.UUID) error
	Find(ctx context.Context, key entity.RequestKey) (entity.RequestKey, error)
}
