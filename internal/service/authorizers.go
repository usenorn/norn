package service

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=authorizers.go -destination=authorizer/mock_authorizers.go -package=authorizer -mock_names=Authorizer=MockAuthorizer

type Authorizer interface {
	Authorize(ctx context.Context, role entity.MembershipRole, resource, action string) error
	SeedPolicy(ctx context.Context) error
}
