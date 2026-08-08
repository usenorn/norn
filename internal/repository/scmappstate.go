package repository

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=scmappstate.go -destination=scmappstate/mock_scmappstate.go -package=scmappstate -mock_names=SCMAppState=MockSCMAppState

type SCMAppState interface {
	Put(ctx context.Context, state string, attempt entity.SCMAppState) error
	Read(ctx context.Context, state string) (entity.SCMAppState, error)
	Take(ctx context.Context, state string) (entity.SCMAppState, error)
}
