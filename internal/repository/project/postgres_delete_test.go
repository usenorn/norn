package project

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestAnAgentScopedBetweenTheCheckAndTheDeleteStillRefusesTheDeletion(t *testing.T) {
	client, cleanup := liveProjectRepository(t)
	defer cleanup()

	var failure error

	err := client.WithTx(context.Background(), func(ctx context.Context) error {
		workspaceID, ownerID, accountID := uuid.New(), uuid.New(), uuid.New()

		statements := []struct {
			query string
			args  []any
		}{
			{
				query: "INSERT INTO workspaces (id, slug, name) VALUES ($1, $2, 'Scoped deletion')",
				args:  []any{workspaceID, "scoped-delete-" + workspaceID.String()[:8]},
			},
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
		}

		for _, statement := range statements {
			if _, err := client.Querier(ctx).ExecContext(ctx, statement.query, statement.args...); err != nil {
				failure = fmt.Errorf("lay deletion fixture: %w", err)

				return errProjectRollback
			}
		}

		projects := New(client)

		created, err := projects.Create(ctx, entity.Project{
			WorkspaceID: workspaceID,
			Slug:        "checkout-rebuild",
			Name:        "Checkout rebuild",
			State:       entity.ProjectStateActive,
		})
		if err != nil {
			failure = fmt.Errorf("create project: %w", err)

			return errProjectRollback
		}

		if err := projects.Delete(ctx, created.ID); err != nil {
			failure = fmt.Errorf("delete a project nothing is scoped to: %w", err)

			return errProjectRollback
		}

		restored, err := projects.Create(ctx, entity.Project{
			WorkspaceID: workspaceID,
			Slug:        "checkout-rebuild",
			Name:        "Checkout rebuild",
			State:       entity.ProjectStateActive,
		})
		if err != nil {
			failure = fmt.Errorf("create project again: %w", err)

			return errProjectRollback
		}

		if _, err := client.Querier(ctx).ExecContext(
			ctx,
			`INSERT INTO workspace_agents
				(workspace_id, account_id, owner_account_id, name, scope, project_id)
			 VALUES ($1, $2, $3, 'late-arrival', 'project', $4)`,
			workspaceID,
			accountID,
			ownerID,
			restored.ID,
		); err != nil {
			failure = fmt.Errorf("scope an agent to the project: %w", err)

			return errProjectRollback
		}

		if err := projects.Delete(ctx, restored.ID); !errors.Is(err, entity.ErrProjectHasScopedAgents) {
			failure = fmt.Errorf(
				"deleting a project an agent points at returned %v, want ErrProjectHasScopedAgents. "+
					"The service checks before it deletes, so a reference that arrives in between "+
					"reaches the caller as a 500 unless this layer translates it",
				err,
			)
		}

		return errProjectRollback
	})

	if !errors.Is(err, errProjectRollback) {
		t.Fatalf("fixture rollback: %v", err)
	}

	if failure != nil {
		t.Fatal(failure)
	}
}
