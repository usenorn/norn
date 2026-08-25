package apitoken

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
)

var errTokenRollback = errors.New("roll back the token fixture")

func TestLatestTokenByOwnerIncludesRevokedCredentials(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("NORN_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("NORN_POSTGRES_DSN is unset, so there is no database to test")
	}

	client, cleanup, err := postgres.New(config.Postgres{
		DSN:             dsn,
		MaxConns:        2,
		MinConns:        1,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
		ConnectTimeout:  10 * time.Second,
	})
	if err != nil {
		t.Fatalf("connect to postgres: %v", err)
	}
	defer cleanup()

	var failure error
	err = client.WithTx(context.Background(), func(ctx context.Context) error {
		accountID := uuid.New()
		if _, err := client.Querier(ctx).ExecContext(
			ctx,
			`INSERT INTO accounts (id, status, kind, display_name, timezone)
             VALUES ($1, 'active', 'agent', 'Token owner', 'UTC')`,
			accountID,
		); err != nil {
			failure = fmt.Errorf("create token owner: %w", err)

			return errTokenRollback
		}

		repository := New(client)
		first, err := repository.Create(ctx, entity.APIToken{
			AccountID: accountID,
			Name:      "first",
			TokenHash: []byte("first-token-hash"),
			Scopes:    entity.APIScopeSet{entity.NewAPIScope(entity.ResourceIssue, entity.ActionRead)},
		})
		if err != nil {
			failure = fmt.Errorf("create first token: %w", err)

			return errTokenRollback
		}

		second, err := repository.Create(ctx, entity.APIToken{
			AccountID: accountID,
			Name:      "second",
			TokenHash: []byte("second-token-hash"),
			Scopes:    entity.APIScopeSet{entity.NewAPIScope(entity.ResourceIssue, entity.ActionManage)},
		})
		if err != nil {
			failure = fmt.Errorf("create second token: %w", err)

			return errTokenRollback
		}

		now := time.Now().UTC()
		for _, tokenID := range []uuid.UUID{first.ID, second.ID} {
			if err := repository.Revoke(ctx, tokenID, now); err != nil {
				failure = fmt.Errorf("revoke token: %w", err)

				return errTokenRollback
			}
		}

		if _, err := client.Querier(ctx).ExecContext(
			ctx,
			"UPDATE api_tokens SET created_at = $2 WHERE id = $1",
			first.ID,
			now.Add(-time.Hour),
		); err != nil {
			failure = fmt.Errorf("order first token: %w", err)

			return errTokenRollback
		}

		if _, err := client.Querier(ctx).ExecContext(
			ctx,
			"UPDATE api_tokens SET created_at = $2 WHERE id = $1",
			second.ID,
			now,
		); err != nil {
			failure = fmt.Errorf("order second token: %w", err)

			return errTokenRollback
		}

		latest, err := repository.GetLatestByOwner(ctx, accountID)
		if err != nil {
			failure = fmt.Errorf("read latest token: %w", err)

			return errTokenRollback
		}

		if latest.ID != second.ID || !latest.Revoked() {
			failure = fmt.Errorf("latest token = %+v, want revoked second token", latest)
		}

		return errTokenRollback
	})

	if !errors.Is(err, errTokenRollback) {
		t.Fatalf("fixture rollback: %v", err)
	}

	if failure != nil {
		t.Fatal(failure)
	}
}
