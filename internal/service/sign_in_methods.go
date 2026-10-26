package service

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=sign_in_methods.go -destination=signinmethod/mock_sign_in_methods.go -package=signinmethod -mock_names=SignInMethods=MockSignInMethods

type SignInMethods interface {
	ForWorkspace(ctx context.Context, slug string) (entity.WorkspaceSignIn, error)
	Discover(ctx context.Context, input DiscoverSignInInput) (entity.SignInOptions, error)
}

type DiscoverSignInInput struct {
	Email  string
	Client entity.SessionClient
}
