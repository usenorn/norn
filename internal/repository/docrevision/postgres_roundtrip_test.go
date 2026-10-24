package docrevision

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

var errRevisionRollback = errors.New("roll the doc revision fixture back")

const revisionFixtureQuery = `
WITH workspace AS (
    INSERT INTO workspaces (id, slug, name) VALUES ($1, $2, 'Doc revision fixture')
    RETURNING id
), author AS (
    INSERT INTO accounts (id, status, kind, email, display_name, timezone)
    VALUES ($3, 'active', 'person', $4, 'Rae', 'UTC')
    RETURNING id
), doc AS (
    INSERT INTO workspace_docs (workspace_id, title, content, visibility, author_account_id)
    SELECT workspace.id, 'Runbook', '{"type":"doc"}', 'workspace', author.id FROM workspace, author
    RETURNING id
), other AS (
    INSERT INTO workspace_docs (workspace_id, title, content, visibility)
    SELECT id, 'Elsewhere', '{"type":"doc"}', 'workspace' FROM workspace
    RETURNING id
)
SELECT doc.id, other.id FROM doc, other`

func liveRevisionRepository(t *testing.T) (*postgres.Client, func()) {
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

func TestEveryRevisionIsKeptWithItsAttribution(t *testing.T) {
	client, cleanup := liveRevisionRepository(t)
	defer cleanup()

	var failure error

	err := client.WithTx(context.Background(), func(ctx context.Context) error {
		failure = recordAndReadRevisions(ctx, client)

		return errRevisionRollback
	})
	if !errors.Is(err, errRevisionRollback) {
		t.Fatalf("transaction: %v", err)
	}

	if failure != nil {
		t.Fatal(failure)
	}
}

func recordAndReadRevisions(ctx context.Context, client *postgres.Client) error {
	workspaceID, authorID := uuid.New(), uuid.New()

	var docID, otherID uuid.UUID

	if err := client.Querier(ctx).QueryRowContext(
		ctx,
		revisionFixtureQuery,
		workspaceID,
		"doc-revisions-"+workspaceID.String()[:8],
		authorID,
		authorID.String()+"@example.test",
	).Scan(&docID, &otherID); err != nil {
		return fmt.Errorf("lay doc revision fixture: %w", err)
	}

	revisions := New(client)
	agentID, connectionID := uuid.New(), uuid.New()
	at := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)
	recorded := make([]entity.DocRevision, 0, 3)

	for version := 1; version <= 3; version++ {
		content := entity.DocumentFromMarkdown(fmt.Sprintf("Version %d.", version))
		doc := entity.Doc{
			DocSummary: entity.DocSummary{
				ID:          docID,
				WorkspaceID: workspaceID,
				Title:       "Runbook",
				Audience:    entity.WorkspaceAudience(),
				Version:     version,
				UpdatedAt:   at.Add(time.Duration(version) * time.Second),
			},
			Content:  content,
			Markdown: content.Markdown(),
		}

		revision := doc.Revision(entity.DocRevisionEdited, entity.DocAttribution{
			AccountID:    authorID,
			AgentID:      &agentID,
			ConnectionID: &connectionID,
			Source:       entity.RevisionSourceAgent,
		})

		if err := revisions.Record(ctx, revision); err != nil {
			return fmt.Errorf("record version %d: %w", version, err)
		}

		recorded = append(recorded, revision)
	}

	first, err := revisions.List(ctx, workspaceID, docID, entity.DocRevisionPage{Limit: 2})
	if err != nil {
		return fmt.Errorf("list revisions: %w", err)
	}

	if len(first) != 2 || first[0].Version != 3 || first[1].Version != 2 {
		return fmt.Errorf("the newest revisions are not listed first: %+v", first)
	}

	if first[0].AccountName != "Rae" || first[0].By.AccountID != authorID || first[0].By.Source != entity.RevisionSourceAgent ||
		first[0].By.AgentID == nil || *first[0].By.AgentID != agentID ||
		first[0].By.ConnectionID == nil || *first[0].By.ConnectionID != connectionID {
		return fmt.Errorf("the revision lost who made it: %+v", first[0].By)
	}

	cursor := first[1].Cursor()

	rest, err := revisions.List(ctx, workspaceID, docID, entity.DocRevisionPage{Cursor: &cursor, Limit: 2})
	if err != nil || len(rest) != 1 || rest[0].Version != 1 {
		return fmt.Errorf("the page after version 2 listed %+v (%v)", rest, err)
	}

	read, err := revisions.GetByID(ctx, workspaceID, docID, recorded[0].ID)
	if err != nil {
		return fmt.Errorf("read revision: %w", err)
	}

	if read.Markdown != "Version 1." || len(read.Content.Content) == 0 || read.Action != entity.DocRevisionEdited {
		return fmt.Errorf("the revision read back differently: %+v", read)
	}

	if _, err := revisions.GetByID(ctx, workspaceID, otherID, recorded[0].ID); !errors.Is(err, entity.ErrDocRevisionNotFound) {
		return fmt.Errorf("a revision read through another doc returned %v", err)
	}

	return nil
}
