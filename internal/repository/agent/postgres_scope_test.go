package agent

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
)

func TestAnAgentScopeSurvivesBeingReadBackByEveryLookup(t *testing.T) {
	client, cleanup := liveAgentRepository(t)
	defer cleanup()

	var failure error

	err := client.WithTx(context.Background(), func(ctx context.Context) error {
		workspaceID, ownerID, accountID := uuid.New(), uuid.New(), uuid.New()
		projectID := uuid.New()

		if err := layScopeFixture(ctx, client, workspaceID, ownerID, accountID, projectID); err != nil {
			failure = err

			return errAgentRollback
		}

		repository := New(client)

		created, err := repository.Create(ctx, entity.Agent{
			WorkspaceID:    workspaceID,
			AccountID:      accountID,
			OwnerAccountID: ownerID,
			Name:           "scoped-agent",
			Scope:          entity.AgentScopeProject,
			ProjectID:      &projectID,
		})
		if err != nil {
			failure = fmt.Errorf("create agent: %w", err)

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

		for name, read := range map[string]entity.Agent{
			"the write itself": created,
			"by id":            byID,
			"by account":       byAccount,
			"in the list":      listed[0],
		} {
			if read.Scope != entity.AgentScopeProject {
				failure = fmt.Errorf("%s returned scope %q, want project", name, read.Scope)

				return errAgentRollback
			}

			if read.ProjectID == nil || *read.ProjectID != projectID {
				failure = fmt.Errorf("%s returned project %v, want %s", name, read.ProjectID, projectID)

				return errAgentRollback
			}
		}

		scoped, err := repository.ScopedToProject(ctx, workspaceID, projectID)
		if err != nil {
			failure = fmt.Errorf("find agents scoped to project: %w", err)

			return errAgentRollback
		}

		if !scoped {
			failure = errors.New("the project reported no agents scoped to it while one was")

			return errAgentRollback
		}

		narrowed, err := repository.SetScope(ctx, workspaceID, created.ID, entity.AgentScopeMember, nil)
		if err != nil {
			failure = fmt.Errorf("set agent scope: %w", err)

			return errAgentRollback
		}

		if narrowed.Scope != entity.AgentScopeMember || narrowed.ProjectID != nil {
			failure = fmt.Errorf(
				"narrowing left scope %q and project %v, want member and nothing",
				narrowed.Scope, narrowed.ProjectID,
			)

			return errAgentRollback
		}

		if _, err := repository.SetScope(
			ctx, workspaceID, uuid.New(), entity.AgentScopeMember, nil,
		); !errors.Is(err, entity.ErrAgentNotFound) {
			failure = fmt.Errorf("scoping an unknown agent = %v, want ErrAgentNotFound", err)
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

func TestClosingAWorkspaceTakesItsProjectScopedAgentsWithIt(t *testing.T) {
	client, cleanup := liveAgentRepository(t)
	defer cleanup()

	var failure error

	err := client.WithTx(context.Background(), func(ctx context.Context) error {
		workspaceID, ownerID, accountID := uuid.New(), uuid.New(), uuid.New()
		projectID := uuid.New()

		if err := layScopeFixture(ctx, client, workspaceID, ownerID, accountID, projectID); err != nil {
			failure = err

			return errAgentRollback
		}

		if _, err := New(client).Create(ctx, entity.Agent{
			WorkspaceID:    workspaceID,
			AccountID:      accountID,
			OwnerAccountID: ownerID,
			Name:           "scoped-agent",
			Scope:          entity.AgentScopeProject,
			ProjectID:      &projectID,
		}); err != nil {
			failure = fmt.Errorf("create agent: %w", err)

			return errAgentRollback
		}

		if _, err := client.Querier(ctx).ExecContext(
			ctx,
			"DELETE FROM workspaces WHERE id = $1",
			workspaceID,
		); err != nil {
			failure = fmt.Errorf(
				"closing a workspace whose agent is scoped to one of its projects: %w. The "+
					"project reference must not be checked before the agent row is gone",
				err,
			)
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

func layScopeFixture(
	ctx context.Context,
	client *postgres.Client,
	workspaceID, ownerID, accountID, projectID uuid.UUID,
) error {
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
			query: "INSERT INTO workspaces (id, slug, name) VALUES ($1, $2, 'Scope roundtrip')",
			args:  []any{workspaceID, "agent-scope-" + workspaceID.String()[:8]},
		},
		{
			query: `INSERT INTO workspace_projects (id, workspace_id, slug, name)
                    VALUES ($1, $2, $3, 'Scoped project')`,
			args: []any{projectID, workspaceID, "scoped-" + projectID.String()[:8]},
		},
	}

	for _, statement := range statements {
		if _, err := client.Querier(ctx).ExecContext(ctx, statement.query, statement.args...); err != nil {
			return fmt.Errorf("lay agent fixture: %w", err)
		}
	}

	return nil
}
