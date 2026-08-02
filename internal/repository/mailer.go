package repository

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=mailer.go -destination=mailer/mock_mailer.go -package=mailer -mock_names=Mailer=MockMailer

type Mailer interface {
	Send(ctx context.Context, mail entity.Mail) error
}
