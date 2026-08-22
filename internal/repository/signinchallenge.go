package repository

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=signinchallenge.go -destination=signinchallenge/mock_signinchallenge.go -package=signinchallenge -mock_names=SignInChallenge=MockSignInChallenge

type SignInChallenge interface {
	Put(ctx context.Context, id string, challenge entity.SignInChallenge) error
	Get(ctx context.Context, id string) (entity.SignInChallenge, error)
	Delete(ctx context.Context, id string) error
}
