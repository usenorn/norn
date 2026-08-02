package repository

import (
	"context"
	"net/netip"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=sign_in_throttle.go -destination=signinthrottle/mock_sign_in_throttle.go -package=signinthrottle -mock_names=SignInThrottle=MockSignInThrottle

type SignInThrottle interface {
	Get(ctx context.Context, subjectHash string) (entity.SignInThrottle, error)
	RecordFailure(ctx context.Context, subjectHash string) (entity.SignInThrottle, error)
	Clear(ctx context.Context, subjectHash string) error
	RecordAddressAttempt(ctx context.Context, ip netip.Addr) (int, error)
}
