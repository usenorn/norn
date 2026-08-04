package service

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=authorizers.go -destination=authorizer/mock_authorizers.go -package=authorizer -mock_names=Authorizer=MockAuthorizer

type Authorizer interface {
	Decide(ctx context.Context, request entity.AccessRequest) (entity.Decision, error)
	SeedPolicy(ctx context.Context) error
}
