package doclink

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

var errLinkRollback = errors.New("roll the doc link fixture back")

const linkFixtureQuery = `
WITH workspace AS (
    INSERT INTO workspaces (id, slug, name) VALUES ($1, $2, 'Doc link fixture')
    RETURNING id
), team AS (
    INSERT INTO workspace_teams (workspace_id, key, name)
    SELECT id, 'LNK', 'Links' FROM workspace
    RETURNING id, workspace_id
), state AS (
    INSERT INTO workspace_workflow_states (workspace_id, team_id, name, category, position)
    SELECT workspace_id, id, 'Todo', 'not_started', 1 FROM team
    RETURNING id
), project AS (
    INSERT INTO workspace_projects (workspace_id, slug, name)
    SELECT id, 'handbook', 'Handbook' FROM workspace
    RETURNING id
), issue AS (
    INSERT INTO workspace_issues (workspace_id, team_id, number, title, state_id, reference_key, rank)
    SELECT team.workspace_id, team.id, 7, 'Ship the handbook', state.id, 'LNK', 'n'
    FROM team, state
    RETURNING id, team_id
), doc AS (
    INSERT INTO workspace_docs (workspace_id, title, content, visibility)
    SELECT id, 'Handbook plan', '{"type":"doc"}', 'workspace' FROM workspace
    RETURNING id
)
SELECT issue.id, issue.team_id, project.id, doc.id FROM issue, project, doc`

type linkFixture struct {
	workspaceID, issueID, teamID, projectID, docID uuid.UUID
}

func withLinkFixture(t *testing.T, run func(ctx context.Context, links *linkRepository, fixture linkFixture) error) {
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
	defer cleanup()

	var failure error

	err = client.WithTx(context.Background(), func(ctx context.Context) error {
		fixture := linkFixture{workspaceID: uuid.New()}

		if err := client.Querier(ctx).QueryRowContext(
			ctx, linkFixtureQuery, fixture.workspaceID, "links-"+fixture.workspaceID.String()[:8],
		).Scan(&fixture.issueID, &fixture.teamID, &fixture.projectID, &fixture.docID); err != nil {
			failure = fmt.Errorf("lay doc link fixture: %w", err)

			return errLinkRollback
		}

		failure = run(ctx, &linkRepository{db: client}, fixture)

		return errLinkRollback
	})
	if !errors.Is(err, errLinkRollback) {
		t.Fatalf("transaction: %v", err)
	}

	if failure != nil {
		t.Fatal(failure)
	}
}

func beside(ctx context.Context, links *linkRepository, write func() error) error {
	querier := links.db.Querier(ctx)

	if _, err := querier.ExecContext(ctx, "SAVEPOINT doc_link_write"); err != nil {
		return fmt.Errorf("open savepoint: %w", err)
	}

	failure := write()

	if _, err := querier.ExecContext(ctx, "ROLLBACK TO SAVEPOINT doc_link_write"); err != nil {
		return fmt.Errorf("roll back to savepoint: %w", err)
	}

	return failure
}

func TestADocLinksOnceToEachIssueAndProjectAndHidesIssuesOutsideTheScope(t *testing.T) {
	withLinkFixture(t, func(ctx context.Context, links *linkRepository, fixture linkFixture) error {
		at := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)
		toIssue := entity.StoredDocLink{
			ID: uuid.New(), WorkspaceID: fixture.workspaceID, DocID: fixture.docID, IssueID: fixture.issueID, CreatedAt: at,
		}
		toProject := entity.StoredDocLink{
			ID: uuid.New(), WorkspaceID: fixture.workspaceID, DocID: fixture.docID, ProjectID: fixture.projectID,
			CreatedAt: at.Add(time.Minute),
		}

		for _, link := range []entity.StoredDocLink{toIssue, toProject} {
			if err := links.Create(ctx, link); err != nil {
				return fmt.Errorf("create %s link: %w", link.Target(), err)
			}

			again := link
			again.ID = uuid.New()

			if err := beside(ctx, links, func() error { return links.Create(ctx, again) }); !errors.Is(err, entity.ErrDocLinkExists) {
				return fmt.Errorf("a second %s link returned %v, want ErrDocLinkExists", link.Target(), err)
			}
		}

		if err := links.Delete(ctx, fixture.workspaceID, uuid.New(), toIssue.ID); !errors.Is(err, entity.ErrDocLinkNotFound) {
			return fmt.Errorf("deleting a link through another doc returned %v", err)
		}

		everyTeam := entity.TeamScope{WorkspaceID: fixture.workspaceID, AllTeams: true}

		listed, err := links.ListForDoc(ctx, fixture.workspaceID, fixture.docID, everyTeam)
		if err != nil {
			return fmt.Errorf("list links: %w", err)
		}

		if len(listed) != 2 ||
			listed[0].Target != entity.DocLinkTargetIssue || listed[0].Issue.Reference() != "LNK-7" ||
			listed[0].Issue.Title != "Ship the handbook" || listed[0].Issue.State.Name != "Todo" ||
			listed[1].Target != entity.DocLinkTargetProject || listed[1].Project.Slug != "handbook" ||
			listed[1].Project.Name != "Handbook" {
			return fmt.Errorf("listed %+v", listed)
		}

		otherTeam := entity.TeamScope{WorkspaceID: fixture.workspaceID, TeamIDs: []uuid.UUID{uuid.New()}}

		listed, err = links.ListForDoc(ctx, fixture.workspaceID, fixture.docID, otherTeam)
		if err != nil {
			return fmt.Errorf("list links outside the team: %w", err)
		}

		if len(listed) != 1 || listed[0].Target != entity.DocLinkTargetProject {
			return fmt.Errorf("a reader outside the issue's team saw %+v", listed)
		}

		if err := links.Delete(ctx, fixture.workspaceID, fixture.docID, toIssue.ID); err != nil {
			return fmt.Errorf("delete issue link: %w", err)
		}

		if err := links.Delete(ctx, fixture.workspaceID, fixture.docID, toIssue.ID); !errors.Is(err, entity.ErrDocLinkNotFound) {
			return fmt.Errorf("deleting a removed link returned %v", err)
		}

		return nil
	})
}
