package domainownership

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
)

type dnsOwnership struct {
	resolver *net.Resolver
}

func New() repository.DomainOwnership {
	return &dnsOwnership{resolver: net.DefaultResolver}
}

func (o *dnsOwnership) PublishesTXT(ctx context.Context, domain, record string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, entity.WorkspaceDomainLookupTimeout)
	defer cancel()

	published, err := o.resolver.LookupTXT(ctx, domain)
	if err != nil {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			return false, nil
		}

		return false, fmt.Errorf("%w: %s: %w", entity.ErrWorkspaceDomainLookupFailed, domain, err)
	}

	for _, value := range published {
		if strings.TrimSpace(value) == record {
			return true, nil
		}
	}

	return false, nil
}
