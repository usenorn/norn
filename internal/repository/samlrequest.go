package repository

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=samlrequest.go -destination=samlrequest/mock_samlrequest.go -package=samlrequest -mock_names=SAMLRequest=MockSAMLRequest

type SAMLRequest interface {
	Put(ctx context.Context, relayState string, attempt entity.SAMLAttempt) error
	Take(ctx context.Context, relayState string) (entity.SAMLAttempt, error)
}
