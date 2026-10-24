package search

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

var errSearchRollback = errors.New("roll the search fixture back")

const searchFixtureQuery = `
WITH workspace AS (
    INSERT INTO workspaces (id, slug, name) VALUES ($1, $2, 'Search fixture')
    RETURNING id
), people AS (
    INSERT INTO accounts (id, status, kind, email, display_name, timezone)
    VALUES ($3, 'active', 'person', $4, 'Rae', 'UTC'),
           ($5, 'active', 'person', $6, 'Ode', 'UTC')
    RETURNING id
), memberships AS (
    INSERT INTO workspace_memberships (workspace_id, account_id, role)
    SELECT workspace.id, people.id, 'member' FROM workspace, people
    RETURNING id
), project AS (
    INSERT INTO workspace_projects (workspace_id, slug, name)
    SELECT id, 'platform', 'Platform' FROM workspace
    RETURNING id
)
SELECT project.id FROM project, (SELECT count(*) FROM memberships) AS laid`

const insertDocQuery = `
INSERT INTO workspace_docs (
    workspace_id, title, content, markdown, visibility, project_id, author_account_id,
    status, archived_at, updated_at
) VALUES ($1, $2, '{}'::jsonb, $3, $4, $5, $6, $7, $8, $9)`

type searchFixture struct {
	workspaceID, authorID, outsiderID, projectID uuid.UUID
}

type fixtureDoc struct {
	title, markdown string
	visibility      entity.KnowledgeVisibility
	archived        bool
	age             time.Duration
}

func withSearchFixture(t *testing.T, run func(ctx context.Context, client *postgres.Client, fixture searchFixture) error) {
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
		fixture := searchFixture{workspaceID: uuid.New(), authorID: uuid.New(), outsiderID: uuid.New()}

		if err := client.Querier(ctx).QueryRowContext(
			ctx,
			searchFixtureQuery,
			fixture.workspaceID,
			"search-"+fixture.workspaceID.String()[:8],
			fixture.authorID,
			fixture.authorID.String()+"@example.test",
			fixture.outsiderID,
			fixture.outsiderID.String()+"@example.test",
		).Scan(&fixture.projectID); err != nil {
			failure = fmt.Errorf("lay search fixture: %w", err)

			return errSearchRollback
		}

		failure = run(ctx, client, fixture)

		return errSearchRollback
	})
	if !errors.Is(err, errSearchRollback) {
		t.Fatalf("transaction: %v", err)
	}

	if failure != nil {
		t.Fatal(failure)
	}
}

func writeDocs(ctx context.Context, client *postgres.Client, fixture searchFixture, docs ...fixtureDoc) error {
	at := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)

	for _, doc := range docs {
		var projectID *uuid.UUID
		if doc.visibility == entity.KnowledgeVisibilityProject {
			projectID = &fixture.projectID
		}

		status, archivedAt := string(entity.DocStatusActive), (*time.Time)(nil)
		if doc.archived {
			status, archivedAt = string(entity.DocStatusArchived), &at
		}

		if _, err := client.Querier(ctx).ExecContext(
			ctx, insertDocQuery,
			fixture.workspaceID, doc.title, doc.markdown, string(doc.visibility), projectID,
			fixture.authorID, status, archivedAt, at.Add(-doc.age),
		); err != nil {
			return fmt.Errorf("write doc %q: %w", doc.title, err)
		}
	}

	return nil
}

func docSearch(fixture searchFixture, accountID uuid.UUID, query string, limit int) repository.SearchRequest {
	return repository.SearchRequest{
		WorkspaceID: fixture.workspaceID,
		AccountID:   accountID,
		Query:       entity.ParseSearchQuery(query),
		Kinds:       []entity.SearchKind{entity.SearchKindDoc},
		Limit:       limit,
		Audience:    repository.SearchAudience{ProjectIDs: []uuid.UUID{}, PersonalAuthorID: accountID},
	}
}

func titles(groups []entity.SearchGroup) []string {
	found := []string{}

	for _, group := range groups {
		for _, result := range group.Results {
			found = append(found, result.Title)
		}
	}

	return found
}

func TestARunbookIsFoundByItsExactTitleFirstAndByAPhraseFromItsBody(t *testing.T) {
	withSearchFixture(t, func(ctx context.Context, client *postgres.Client, fixture searchFixture) error {
		search := New(client)

		if err := writeDocs(ctx, client, fixture,
			fixtureDoc{
				title:      "Database failover runbook",
				markdown:   "Promote the replica first, then repoint pgbouncer at the new primary and watch replication lag.",
				visibility: entity.KnowledgeVisibilityWorkspace,
				age:        48 * time.Hour,
			},
			fixtureDoc{
				title:      "Runbook index for the database failover drills",
				markdown:   "Every database failover runbook we keep, newest first.",
				visibility: entity.KnowledgeVisibilityWorkspace,
			},
		); err != nil {
			return err
		}

		groups, err := search.Search(ctx, docSearch(fixture, fixture.authorID, "Database failover runbook", 10))
		if err != nil {
			return fmt.Errorf("search the exact title: %w", err)
		}

		if found := titles(groups); len(found) != 2 || found[0] != "Database failover runbook" {
			return fmt.Errorf(
				"an exact title search ranked %v; the runbook named exactly that has to come first, "+
					"ahead of a newer doc whose title merely contains the same words", found,
			)
		}

		for _, phrase := range []string{"repoint pgbouncer", "pgbou"} {
			groups, err := search.Search(ctx, docSearch(fixture, fixture.authorID, phrase, 10))
			if err != nil {
				return fmt.Errorf("search %q: %w", phrase, err)
			}

			if found := titles(groups); len(found) != 1 || found[0] != "Database failover runbook" {
				return fmt.Errorf("searching %q found %v, want only the runbook whose body says it", phrase, found)
			}

			excerpt := groups[0].Results[0].Excerpt
			if !strings.Contains(excerpt, "pgbouncer") || strings.ContainsAny(excerpt, "<>*") {
				return fmt.Errorf("searching %q excerpted %q, want the plain body passage around the match", phrase, excerpt)
			}
		}

		return nil
	})
}

func TestAnOutsiderGetsIdenticalResultsWhetherOrNotADocHiddenFromThemExists(t *testing.T) {
	withSearchFixture(t, func(ctx context.Context, client *postgres.Client, fixture searchFixture) error {
		search := New(client)

		if err := writeDocs(ctx, client, fixture,
			fixtureDoc{title: "Incident review template", markdown: "How we write up an incident.", visibility: entity.KnowledgeVisibilityWorkspace, age: time.Hour},
			fixtureDoc{title: "Incident severity levels", markdown: "What makes an incident a sev one.", visibility: entity.KnowledgeVisibilityWorkspace, age: 2 * time.Hour},
		); err != nil {
			return err
		}

		outsider := docSearch(fixture, fixture.outsiderID, "incident", 1)

		before, err := search.Search(ctx, outsider)
		if err != nil {
			return fmt.Errorf("search before the hidden docs exist: %w", err)
		}

		if err := writeDocs(ctx, client, fixture,
			fixtureDoc{title: "Incident", markdown: "Incident incident incident: the payroll outage.", visibility: entity.KnowledgeVisibilityPersonal},
			fixtureDoc{title: "Incident", markdown: "Incident incident incident: the platform postmortem.", visibility: entity.KnowledgeVisibilityProject},
			fixtureDoc{title: "Incident", markdown: "Incident incident incident: retired.", visibility: entity.KnowledgeVisibilityWorkspace, archived: true},
		); err != nil {
			return err
		}

		after, err := search.Search(ctx, outsider)
		if err != nil {
			return fmt.Errorf("search after the hidden docs exist: %w", err)
		}

		if !reflect.DeepEqual(before, after) {
			return fmt.Errorf(
				"the outsider's results changed when docs they cannot see were added: before %+v, after %+v. "+
					"Any difference in rows, order, excerpt or the more flag tells them those docs exist.",
				before, after,
			)
		}

		author, err := search.Search(ctx, docSearch(fixture, fixture.authorID, "incident", 1))
		if err != nil {
			return fmt.Errorf("search as the author: %w", err)
		}

		if found := titles(author); len(found) != 1 || found[0] != "Incident" {
			return fmt.Errorf("the author searched for their own exact title and found %v", found)
		}

		return nil
	})
}
