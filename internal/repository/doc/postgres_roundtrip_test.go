package doc

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
	"github.com/usenorn/norn/internal/repository"
)

var errDocRollback = errors.New("roll the doc fixture back")

const docFixtureQuery = `
WITH workspace AS (
    INSERT INTO workspaces (id, slug, name) VALUES ($1, $2, 'Doc fixture')
    RETURNING id
), author AS (
    INSERT INTO accounts (id, status, kind, email, display_name, timezone)
    VALUES ($3, 'active', 'person', $4, 'Rae', 'UTC')
    RETURNING id
), project AS (
    INSERT INTO workspace_projects (workspace_id, slug, name)
    SELECT id, 'handbook', 'Handbook' FROM workspace
    RETURNING id
)
SELECT project.id FROM project, author`

type docFixture struct {
	workspaceID, authorID, projectID uuid.UUID
}

func liveDocRepository(t *testing.T) (*postgres.Client, func()) {
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

func withDocFixture(t *testing.T, run func(ctx context.Context, client *postgres.Client, fixture docFixture) error) {
	t.Helper()

	client, cleanup := liveDocRepository(t)
	defer cleanup()

	var failure error

	err := client.WithTx(context.Background(), func(ctx context.Context) error {
		fixture := docFixture{workspaceID: uuid.New(), authorID: uuid.New()}

		if err := client.Querier(ctx).QueryRowContext(
			ctx,
			docFixtureQuery,
			fixture.workspaceID,
			"docs-"+fixture.workspaceID.String()[:8],
			fixture.authorID,
			fixture.authorID.String()+"@example.test",
		).Scan(&fixture.projectID); err != nil {
			failure = fmt.Errorf("lay doc fixture: %w", err)

			return errDocRollback
		}

		failure = run(ctx, client, fixture)

		return errDocRollback
	})
	if !errors.Is(err, errDocRollback) {
		t.Fatalf("transaction: %v", err)
	}

	if failure != nil {
		t.Fatal(failure)
	}
}

func draftDoc(fixture docFixture, title string, audience entity.KnowledgeAudience, parentID *uuid.UUID, at time.Time) entity.Doc {
	content := entity.DocumentFromMarkdown(title + " body")

	return entity.Doc{
		DocSummary: entity.DocSummary{
			ID:          uuid.New(),
			WorkspaceID: fixture.workspaceID,
			ParentID:    parentID,
			Title:       title,
			Audience:    audience,
			AuthorID:    fixture.authorID,
			Status:      entity.DocStatusActive,
			Version:     1,
			CreatedAt:   at,
			UpdatedAt:   at,
		},
		Content:  content,
		Markdown: content.Markdown(),
	}
}

func createDoc(ctx context.Context, docs repository.Doc, draft entity.Doc) (entity.Doc, error) {
	if err := docs.Create(ctx, draft); err != nil {
		return entity.Doc{}, err
	}

	return docs.GetByID(ctx, draft.WorkspaceID, draft.ID)
}

func TestADocReadsBackAsItWasWrittenAndRefusesAStaleWrite(t *testing.T) {
	withDocFixture(t, func(ctx context.Context, client *postgres.Client, fixture docFixture) error {
		docs := New(client)
		at := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)

		created, err := createDoc(ctx, docs, draftDoc(fixture, "Release checklist", entity.ProjectAudience(fixture.projectID), nil, at))
		if err != nil {
			return fmt.Errorf("create doc: %w", err)
		}

		if created.AuthorName != "Rae" || created.Audience.ProjectID != fixture.projectID ||
			created.Markdown != "Release checklist body" || len(created.Content.Content) == 0 {
			return fmt.Errorf("the doc read back differently from how it was written: %+v", created)
		}

		if _, err := client.Querier(ctx).ExecContext(
			ctx,
			`INSERT INTO workspace_doc_revisions
				(workspace_id, doc_id, version, action, title, content, visibility, project_id, account_id, source)
			 VALUES ($1, $2, 1, 'created', 'Release checklist', '{"type":"doc"}', 'project', $3, $4, 'person')`,
			fixture.workspaceID, created.ID, fixture.projectID, fixture.authorID,
		); err != nil {
			return fmt.Errorf("record a revision: %w", err)
		}

		edited := created
		edited.Title = "Release checklist v2"
		edited.Audience = entity.WorkspaceAudience()
		edited.Version = 2
		edited.UpdatedAt = at.Add(time.Minute)

		if err := docs.Update(ctx, edited, created.Version); err != nil {
			return fmt.Errorf("update doc: %w", err)
		}

		written, err := docs.GetByID(ctx, fixture.workspaceID, created.ID)
		if err != nil {
			return fmt.Errorf("read updated doc: %w", err)
		}

		if written.Title != edited.Title || written.Version != 2 || written.Audience != entity.WorkspaceAudience() ||
			written.UpdatedAt.Location() != time.UTC || !written.UpdatedAt.Equal(edited.UpdatedAt) {
			return fmt.Errorf("the update did not land: %+v", written)
		}

		if !slices.Contains(written.ContributorIDs, fixture.authorID) {
			return fmt.Errorf("somebody with a revision is not a contributor: %v", written.ContributorIDs)
		}

		edited.Version = 3
		if err := docs.Update(ctx, edited, created.Version); !errors.Is(err, entity.ErrDocStale) {
			return fmt.Errorf("a write against version 1 of a version-2 doc returned %v, want ErrDocStale", err)
		}

		if _, err := docs.GetByID(ctx, uuid.New(), created.ID); !errors.Is(err, entity.ErrDocNotFound) {
			return fmt.Errorf("a doc read through another workspace returned %v", err)
		}

		return nil
	})
}

func TestDocsListOnlyWhatTheReaderMaySee(t *testing.T) {
	withDocFixture(t, func(ctx context.Context, client *postgres.Client, fixture docFixture) error {
		docs := New(client)
		at := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)
		created := map[string]entity.Doc{}

		for index, draft := range []struct {
			title    string
			audience entity.KnowledgeAudience
			parent   string
		}{
			{title: "workspace", audience: entity.WorkspaceAudience()},
			{title: "project", audience: entity.ProjectAudience(fixture.projectID)},
			{title: "personal", audience: entity.PersonalAudience()},
			{title: "child", audience: entity.WorkspaceAudience(), parent: "workspace"},
			{title: "archived", audience: entity.WorkspaceAudience()},
		} {
			var parentID *uuid.UUID
			if draft.parent != "" {
				id := created[draft.parent].ID
				parentID = &id
			}

			doc, err := createDoc(ctx, docs, draftDoc(fixture, draft.title, draft.audience, parentID, at.Add(time.Duration(index)*time.Minute)))
			if err != nil {
				return fmt.Errorf("create %s doc: %w", draft.title, err)
			}

			created[draft.title] = doc
		}

		archived := created["archived"]
		archivedAt := at.Add(time.Hour)
		archived.Status, archived.ArchivedAt, archived.Version, archived.UpdatedAt = entity.DocStatusArchived, &archivedAt, 2, archivedAt

		if err := docs.Update(ctx, archived, 1); err != nil {
			return fmt.Errorf("archive doc: %w", err)
		}

		titles := func(filter repository.DocFilter, page entity.DocPage) ([]string, error) {
			summaries, err := docs.List(ctx, fixture.workspaceID, filter, page)
			if err != nil {
				return nil, err
			}

			listed := make([]string, 0, len(summaries))
			for _, summary := range summaries {
				listed = append(listed, summary.Title)
			}

			return listed, nil
		}

		page := entity.DocPage{Limit: 10}
		parentID := created["workspace"].ID

		for name, testCase := range map[string]struct {
			filter repository.DocFilter
			want   []string
		}{
			"an outsider to the project": {
				filter: repository.DocFilter{},
				want:   []string{"child", "workspace"},
			},
			"a project member": {
				filter: repository.DocFilter{ProjectIDs: []uuid.UUID{fixture.projectID}},
				want:   []string{"child", "project", "workspace"},
			},
			"the author acting personally": {
				filter: repository.DocFilter{AllProjects: true, PersonalAuthorID: fixture.authorID},
				want:   []string{"child", "personal", "project", "workspace"},
			},
			"another person": {
				filter: repository.DocFilter{AllProjects: true, PersonalAuthorID: uuid.New()},
				want:   []string{"child", "project", "workspace"},
			},
			"roots only": {
				filter: repository.DocFilter{Root: true},
				want:   []string{"workspace"},
			},
			"beneath a parent": {
				filter: repository.DocFilter{ParentID: &parentID},
				want:   []string{"child"},
			},
		} {
			listed, err := titles(testCase.filter, page)
			if err != nil {
				return fmt.Errorf("%s: list docs: %w", name, err)
			}

			if !slices.Equal(listed, testCase.want) {
				return fmt.Errorf("%s: listed %v, want %v", name, listed, testCase.want)
			}
		}

		everything := repository.DocFilter{AllProjects: true, PersonalAuthorID: fixture.authorID}

		first, err := docs.List(ctx, fixture.workspaceID, everything, entity.DocPage{Limit: 2})
		if err != nil || len(first) != 2 {
			return fmt.Errorf("first page: %v, %v", first, err)
		}

		cursor := first[len(first)-1].Cursor()

		rest, err := titles(everything, entity.DocPage{Cursor: &cursor, Limit: 10})
		if err != nil {
			return fmt.Errorf("next page: %w", err)
		}

		if !slices.Equal(rest, []string{"project", "workspace"}) {
			return fmt.Errorf("the page after %q listed %v", first[1].Title, rest)
		}

		return nil
	})
}

func TestADocTreeKnowsItsAncestryAndItsHeight(t *testing.T) {
	withDocFixture(t, func(ctx context.Context, client *postgres.Client, fixture docFixture) error {
		docs := New(client)
		at := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)

		root, err := createDoc(ctx, docs, draftDoc(fixture, "root", entity.WorkspaceAudience(), nil, at))
		if err != nil {
			return fmt.Errorf("create root: %w", err)
		}

		middle, err := createDoc(ctx, docs, draftDoc(fixture, "middle", entity.WorkspaceAudience(), &root.ID, at))
		if err != nil {
			return fmt.Errorf("create middle: %w", err)
		}

		leaf, err := createDoc(ctx, docs, draftDoc(fixture, "leaf", entity.WorkspaceAudience(), &middle.ID, at))
		if err != nil {
			return fmt.Errorf("create leaf: %w", err)
		}

		ancestry, err := docs.Ancestry(ctx, fixture.workspaceID, leaf.ID)
		if err != nil {
			return fmt.Errorf("ancestry: %w", err)
		}

		if !slices.Equal(ancestry, []uuid.UUID{leaf.ID, middle.ID, root.ID}) {
			return fmt.Errorf("ancestry of the leaf is %v, want leaf, middle, root", ancestry)
		}

		for doc, want := range map[uuid.UUID]int{root.ID: 2, middle.ID: 1, leaf.ID: 0} {
			height, err := docs.SubtreeHeight(ctx, fixture.workspaceID, doc)
			if err != nil || height != want {
				return fmt.Errorf("subtree height of %s is %d (%v), want %d", doc, height, err, want)
			}
		}

		held, err := docs.HasActiveChildren(ctx, fixture.workspaceID, middle.ID)
		if err != nil || !held {
			return fmt.Errorf("the middle doc reports no children: %v, %v", held, err)
		}

		archivedAt := at.Add(time.Hour)
		leaf.Status, leaf.ArchivedAt, leaf.Version, leaf.UpdatedAt = entity.DocStatusArchived, &archivedAt, 2, archivedAt

		if err := docs.Update(ctx, leaf, 1); err != nil {
			return fmt.Errorf("archive leaf: %w", err)
		}

		if held, err := docs.HasActiveChildren(ctx, fixture.workspaceID, middle.ID); err != nil || held {
			return fmt.Errorf("an archived child still holds its parent: %v, %v", held, err)
		}

		if _, err := docs.Ancestry(ctx, fixture.workspaceID, uuid.New()); !errors.Is(err, entity.ErrDocNotFound) {
			return fmt.Errorf("ancestry of a missing doc returned %v", err)
		}

		return nil
	})
}
