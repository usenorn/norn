package agent

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestAgentInstructionsSurviveBeingReadBackByEveryLookup(t *testing.T) {
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
				query: "INSERT INTO workspaces (id, slug, name) VALUES ($1, $2, 'Instructions roundtrip')",
				args:  []any{workspaceID, "agent-instructions-" + workspaceID.String()[:8]},
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
			Name:           "instructed-agent",
		})
		if err != nil {
			failure = fmt.Errorf("create agent: %w", err)

			return errAgentRollback
		}

		if created.AgentInstructions != "" {
			failure = fmt.Errorf("a fresh agent was told %q, want nothing", created.AgentInstructions)

			return errAgentRollback
		}

		written := "Ask before deleting anything."

		saved, err := repository.SetInstructions(ctx, workspaceID, created.ID, written)
		if err != nil {
			failure = fmt.Errorf("set agent instructions: %w", err)

			return errAgentRollback
		}

		byID, err := repository.GetByID(ctx, workspaceID, created.ID)
		if err != nil {
			failure = fmt.Errorf("read agent by id: %w", err)

			return errAgentRollback
		}

		byAccount, err := repository.GetByAccountID(ctx, accountID)
		if err != nil {
			failure = fmt.Errorf("read agent by account: %w", err)

			return errAgentRollback
		}

		listed, err := repository.ListByWorkspaceID(ctx, workspaceID)
		if err != nil {
			failure = fmt.Errorf("list agents: %w", err)

			return errAgentRollback
		}

		if len(listed) != 1 {
			failure = fmt.Errorf("listed %d agents, want 1", len(listed))

			return errAgentRollback
		}

		for name, read := range map[string]string{
			"the write itself": saved.AgentInstructions,
			"by id":            byID.AgentInstructions,
			"by account":       byAccount.AgentInstructions,
			"in the list":      listed[0].AgentInstructions,
		} {
			if read != written {
				failure = fmt.Errorf("%s returned %q, want %q", name, read, written)

				return errAgentRollback
			}
		}

		cleared, err := repository.SetInstructions(ctx, workspaceID, created.ID, "")
		if err != nil {
			failure = fmt.Errorf("clear agent instructions: %w", err)

			return errAgentRollback
		}

		if cleared.AgentInstructions != "" {
			failure = fmt.Errorf("instructions after clearing = %q, want nothing", cleared.AgentInstructions)

			return errAgentRollback
		}

		if _, err := repository.SetInstructions(ctx, workspaceID, uuid.New(), "nobody"); !errors.Is(err, entity.ErrAgentNotFound) {
			failure = fmt.Errorf("instructions for an unknown agent = %v, want ErrAgentNotFound", err)
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
