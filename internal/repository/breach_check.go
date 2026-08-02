package repository

import "context"

//go:generate go tool mockgen -source=breach_check.go -destination=breachcheck/mock_breach_check.go -package=breachcheck -mock_names=BreachCheck=MockBreachCheck

type BreachCheck interface {
	Compromised(ctx context.Context, password string) (bool, error)
}
