package repository

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=oidcstate.go -destination=oidcstate/mock_oidcstate.go -package=oidcstate -mock_names=OIDCState=MockOIDCState

type OIDCState interface {
	Put(ctx context.Context, state string, attempt entity.OIDCState) error
	Take(ctx context.Context, state string) (entity.OIDCState, error)
}
