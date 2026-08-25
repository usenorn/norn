package agent

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

var errAgentRollback = errors.New("roll back the agent fixture")

func liveAgentRepository(t *testing.T) (*postgres.Client, func()) {
	t.Helper()

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

	return client, cleanup
}

func TestAgentIconAndLifecycleRoundTrip(t *testing.T) {
	client, cleanup := liveAgentRepository(t)
	defer cleanup()

	var failure error
	err := client.WithTx(context.Background(), func(ctx context.Context) error {
		workspaceID, ownerID, accountID := uuid.New(), uuid.New(), uuid.New()

		statements := []struct {
			query string
			args  []any
		}{
			{
				query: `INSERT INTO accounts (id, status, kind, email, display_name, timezone)
                        VALUES ($1, 'active', 'person', $2, 'Owner', 'UTC')`,
				args: []any{ownerID, ownerID.String() + "@example.test"},
			},
			{
				query: `INSERT INTO accounts (id, status, kind, display_name, timezone)
                        VALUES ($1, 'active', 'agent', 'Agent', 'UTC')`,
				args: []any{accountID},
			},
			{
				query: "INSERT INTO workspaces (id, slug, name) VALUES ($1, $2, 'Agent roundtrip')",
				args:  []any{workspaceID, "agent-roundtrip-" + workspaceID.String()[:8]},
			},
		}

		for _, statement := range statements {
			if _, err := client.Querier(ctx).ExecContext(ctx, statement.query, statement.args...); err != nil {
				failure = fmt.Errorf("lay agent fixture: %w", err)

				return errAgentRollback
			}
		}

		repository := New(client)
		created, err := repository.Create(ctx, entity.Agent{
			WorkspaceID:    workspaceID,
			AccountID:      accountID,
			OwnerAccountID: ownerID,
			Name:           "roundtrip-agent",
			Icon:           entity.AgentIconShieldCheck,
		})
		if err != nil {
			failure = fmt.Errorf("create agent: %w", err)

			return errAgentRollback
		}

		if created.Icon != entity.AgentIconShieldCheck {
			failure = fmt.Errorf("icon = %q, want shield-check", created.Icon)

			return errAgentRollback
		}

		disabledAt := time.Now().UTC()
		if err := repository.Disable(ctx, workspaceID, created.ID, disabledAt); err != nil {
			failure = fmt.Errorf("disable agent: %w", err)

			return errAgentRollback
		}

		if err := repository.Enable(ctx, workspaceID, created.ID); err != nil {
			failure = fmt.Errorf("enable agent: %w", err)

			return errAgentRollback
		}

		restored, err := repository.GetByID(ctx, workspaceID, created.ID)
		if err != nil {
			failure = fmt.Errorf("read enabled agent: %w", err)

			return errAgentRollback
		}

		if restored.Status != entity.AgentStatusActive || restored.DisabledAt != nil {
			failure = fmt.Errorf("enabled agent = %+v", restored)

			return errAgentRollback
		}

		if err := repository.Enable(ctx, workspaceID, created.ID); !errors.Is(err, entity.ErrAgentActive) {
			failure = fmt.Errorf("second enable = %v, want ErrAgentActive", err)
		}

		return errAgentRollback
	})

	if !errors.Is(err, errAgentRollback) {
		t.Fatalf("fixture rollback: %v", err)
	}

	if failure != nil {
		t.Fatal(failure)
	}
}
