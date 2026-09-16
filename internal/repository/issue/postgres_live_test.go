package issue

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
)

var errIssueRollback = errors.New("roll back the issue fixture")

func liveIssueDatabase(t *testing.T) (*postgres.Client, func()) {
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

func held(issues []entity.Issue, issueID uuid.UUID) bool {
	return slices.ContainsFunc(issues, func(issue entity.Issue) bool { return issue.ID == issueID })
}

func TestAnIssueAssignedToSomebodyIsListedForThemUnlessItIsWaitingInTriage(t *testing.T) {
	client, cleanup := liveIssueDatabase(t)
	defer cleanup()

	var failure error

	err := client.WithTx(context.Background(), func(ctx context.Context) error {
		workspaceID, teamID, stateID := uuid.New(), uuid.New(), uuid.New()
		personID, decidedID, waitingID := uuid.New(), uuid.New(), uuid.New()

		statements := []struct {
			query string
			args  []any
		}{
			{
				query: `INSERT INTO accounts (id, status, kind, email, display_name, timezone)
                        VALUES ($1, 'active', 'person', $2, 'Rae Okafor', 'UTC')`,
				args: []any{personID, personID.String() + "@example.test"},
			},
			{
				query: "INSERT INTO workspaces (id, slug, name) VALUES ($1, $2, 'Northwind')",
				args:  []any{workspaceID, "triage-listing-" + workspaceID.String()[:8]},
			},
			{
				query: "INSERT INTO workspace_teams (id, workspace_id, key, name) VALUES ($1, $2, 'TRI', 'Triage')",
				args:  []any{teamID, workspaceID},
			},
			{
				query: `INSERT INTO workspace_workflow_states (id, workspace_id, team_id, name, category, position)
                        VALUES ($1, $2, $3, 'Todo', 'not_started', 1)`,
				args: []any{stateID, workspaceID, teamID},
			},
			{
				query: `INSERT INTO workspace_issues
                            (id, workspace_id, team_id, number, title, state_id, reference_key, rank, assignee_account_id)
                        VALUES ($1, $2, $3, 1, 'Raised by an agent for a person', $4, 'TRI', 'n', $5)`,
				args: []any{decidedID, workspaceID, teamID, stateID, personID},
			},
			{
				query: `INSERT INTO workspace_issues
                            (id, workspace_id, team_id, number, title, state_id, reference_key, rank,
                             assignee_account_id, triage_state, triage_source)
                        VALUES ($1, $2, $3, 2, 'Still waiting in triage', $4, 'TRI', 'o', $5, 'waiting', 'agent')`,
				args: []any{waitingID, workspaceID, teamID, stateID, personID},
			},
		}

		for _, statement := range statements {
			if _, err := client.Querier(ctx).ExecContext(ctx, statement.query, statement.args...); err != nil {
				failure = fmt.Errorf("lay issue fixture: %w", err)

				return errIssueRollback
			}
		}

		repository := New(client)
		scope := entity.TeamScope{WorkspaceID: workspaceID, TeamIDs: []uuid.UUID{teamID}}

		assignee := entity.IssueFilter{
			Field:  entity.IssueFilterFieldAssignee,
			Op:     entity.IssueFilterOpIs,
			Values: []string{personID.String()},
		}

		surfaces := map[string]*entity.IssueFilter{
			"the team's issue list":     nil,
			"a filter by that assignee": &assignee,
			"that person's My tasks": {
				All: []entity.IssueFilter{assignee, {
					Field:  entity.IssueFilterFieldStateCategory,
					Op:     entity.IssueFilterOpIn,
					Values: []string{string(entity.StateCategoryNotStarted), string(entity.StateCategoryActive)},
				}},
			},
		}

		for surface, filter := range surfaces {
			listed, err := repository.ListVisible(ctx, scope, entity.IssuePage{
				Limit:    50,
				Statuses: []entity.IssueStatus{entity.IssueStatusActive},
				Filter:   filter,
			})
			if err != nil {
				failure = fmt.Errorf("list issues for %s: %w", surface, err)

				return errIssueRollback
			}

			if !held(listed, decidedID) {
				failure = fmt.Errorf(
					"%s left out an issue assigned to the person it belongs to. Work handed to "+
						"somebody has to be findable where they look for it, not only by a direct link",
					surface,
				)

				return errIssueRollback
			}

			if held(listed, waitingID) {
				failure = fmt.Errorf(
					"%s showed an issue still waiting in triage. Triage exists so unowned arrivals "+
						"wait outside the backlog until somebody accepts them",
					surface,
				)

				return errIssueRollback
			}
		}

		return errIssueRollback
	})

	if !errors.Is(err, errIssueRollback) {
		t.Fatalf("fixture rollback: %v", err)
	}

	if failure != nil {
		t.Fatal(failure)
	}
}
