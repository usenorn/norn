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

func TestAProjectADocIsSharedWithRefusesTheDeletion(t *testing.T) {
	client, cleanup := liveProjectRepository(t)
	defer cleanup()

	var failure error

	err := client.WithTx(context.Background(), func(ctx context.Context) error {
		workspaceID := uuid.New()

		if _, err := client.Querier(ctx).ExecContext(
			ctx,
			"INSERT INTO workspaces (id, slug, name) VALUES ($1, $2, 'Shared deletion')",
			workspaceID,
			"shared-delete-"+workspaceID.String()[:8],
		); err != nil {
			failure = fmt.Errorf("lay deletion fixture: %w", err)

			return errProjectRollback
		}

		projects := New(client)

		shared, err := projects.Create(ctx, entity.Project{
			WorkspaceID: workspaceID,
			Slug:        "handbook",
			Name:        "Handbook",
			State:       entity.ProjectStateActive,
		})
		if err != nil {
			failure = fmt.Errorf("create project: %w", err)

			return errProjectRollback
		}

		if _, err := client.Querier(ctx).ExecContext(
			ctx,
			`INSERT INTO workspace_docs (workspace_id, title, content, visibility, project_id)
			 VALUES ($1, 'Release checklist', '{"type":"doc"}', 'project', $2)`,
			workspaceID,
			shared.ID,
		); err != nil {
			failure = fmt.Errorf("share a doc with the project: %w", err)

			return errProjectRollback
		}

		if err := projects.Delete(ctx, shared.ID); !errors.Is(err, entity.ErrProjectHasDocs) {
			failure = fmt.Errorf(
				"deleting a project a doc is shared with returned %v, want ErrProjectHasDocs. "+
					"The doc would be left with an audience nobody can belong to",
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

func TestAProjectAMemoryIsSharedWithOrAppliesToRefusesTheDeletion(t *testing.T) {
	for name, recordMemory := range map[string]string{
		"shared with it": `INSERT INTO workspace_memories
			(workspace_id, kind, statement, visibility, project_id, status, basis, source, effective_from)
			VALUES ($1, 'decision', 'Ship weekly', 'project', $2, 'accepted', 'observation', 'person', current_date)`,
		"applying to it": `INSERT INTO workspace_memories
			(workspace_id, kind, statement, visibility, applies_project_id, status, basis, source, effective_from)
			VALUES ($1, 'decision', 'Ship weekly', 'workspace', $2, 'accepted', 'observation', 'person', current_date)`,
	} {
		t.Run(name, func(t *testing.T) {
			client, cleanup := liveProjectRepository(t)
			defer cleanup()

			var failure error

			err := client.WithTx(context.Background(), func(ctx context.Context) error {
				workspaceID := uuid.New()

				if _, err := client.Querier(ctx).ExecContext(
					ctx,
					"INSERT INTO workspaces (id, slug, name) VALUES ($1, $2, 'Memory deletion')",
					workspaceID,
					"memory-delete-"+workspaceID.String()[:8],
				); err != nil {
					failure = fmt.Errorf("lay deletion fixture: %w", err)

					return errProjectRollback
				}

				projects := New(client)

				project, err := projects.Create(ctx, entity.Project{
					WorkspaceID: workspaceID,
					Slug:        "platform",
					Name:        "Platform",
					State:       entity.ProjectStateActive,
				})
				if err != nil {
					failure = fmt.Errorf("create project: %w", err)

					return errProjectRollback
				}

				if _, err := client.Querier(ctx).ExecContext(
					ctx,
					recordMemory,
					workspaceID,
					project.ID,
				); err != nil {
					failure = fmt.Errorf("record a memory about the project: %w", err)

					return errProjectRollback
				}

				if err := projects.Delete(ctx, project.ID); !errors.Is(err, entity.ErrProjectHasMemories) {
					failure = fmt.Errorf("deleting a project a memory refers to returned %v, want ErrProjectHasMemories", err)
				}

				return errProjectRollback
			})

			if !errors.Is(err, errProjectRollback) {
				t.Fatalf("fixture rollback: %v", err)
			}

			if failure != nil {
				t.Fatal(failure)
			}
		})
	}
}
