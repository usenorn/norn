package repository

import "context"

//go:generate go tool mockgen -source=domain_ownership.go -destination=domainownership/mock_domain_ownership.go -package=domainownership -mock_names=DomainOwnership=MockDomainOwnership

type DomainOwnership interface {
	PublishesTXT(ctx context.Context, domain, record string) (bool, error)
}
