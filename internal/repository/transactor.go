package repository

import "context"

//go:generate go tool mockgen -source=transactor.go -destination=transactor/mock_transactor.go -package=transactor -mock_names=Transactor=MockTransactor

type Transactor interface {
	WithTx(ctx context.Context, fn func(context.Context) error) error
}
