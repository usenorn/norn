package breachcheck

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/pwned"
	"github.com/usenorn/norn/internal/repository"
)

type breachCheckRepository struct {
	client *pwned.Client
}

func New(client *pwned.Client) repository.BreachCheck {
	return &breachCheckRepository{client: client}
}

func (r *breachCheckRepository) Compromised(ctx context.Context, password string) (bool, error) {
	if !r.client.Enabled() {
		return false, nil
	}

	prefix, suffix := entity.PasswordBreachDigest(password)

	body, err := r.client.Range(ctx, prefix)
	if err != nil {
		return false, fmt.Errorf("%w: %w", entity.ErrPasswordBreachCheckUnavailable, err)
	}

	defer func() { _ = body.Close() }()

	scanner := bufio.NewScanner(body)

	for scanner.Scan() {
		candidate, _, found := strings.Cut(scanner.Text(), ":")
		if !found {
			continue
		}

		if strings.EqualFold(strings.TrimSpace(candidate), suffix) {
			return true, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("%w: %w", entity.ErrPasswordBreachCheckUnavailable, err)
	}

	return false, nil
}
