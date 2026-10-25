package docpassage

import (
	"context"
	"errors"
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

var errPassageRollback = errors.New("roll the passage fixture back")

func reach(t *testing.T) *postgres.Client {
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

	t.Cleanup(cleanup)

	return client
}

func TestPassagesFollowTheDocVersionTheyWereCutFrom(t *testing.T) {
	client := reach(t)
	passages := New(client)

	err := client.WithTx(context.Background(), func(ctx context.Context) error {
		workspaceID, docID := uuid.New(), uuid.New()

		if _, err := client.Querier(ctx).ExecContext(ctx, `
WITH workspace AS (INSERT INTO workspaces (id, slug, name) VALUES ($1, $2, 'Passages') RETURNING id)
INSERT INTO workspace_docs (id, workspace_id, title, content, visibility)
SELECT $3, id, 'Runbook', '{"type":"doc"}', 'workspace' FROM workspace`,
			workspaceID, "passages-"+workspaceID.String()[:8], docID); err != nil {
			return err
		}

		outdated, err := passages.Outdated(ctx, workspaceID, 10)
		if err != nil {
			return err
		}

		if !slices.Equal(outdated, []uuid.UUID{docID}) {
			t.Errorf("outdated = %v, want the doc that has no passages yet", outdated)
		}

		doc := entity.Doc{
			DocSummary: entity.DocSummary{ID: docID, WorkspaceID: workspaceID, Title: "Runbook", Version: 1},
			Content:    entity.DocumentFromMarkdown("# Deploys\n\nShip on Tuesdays.\n\n# Alerts\n\nPage the owner.\n"),
		}

		if err := passages.Replace(ctx, doc); err != nil {
			return err
		}

		doc.Version = 2
		doc.Content = entity.DocumentFromMarkdown("# Deploys\n\nShip on Wednesdays.\n")

		if outdated, err := passages.Outdated(ctx, workspaceID, 10); err != nil || len(outdated) != 0 {
			t.Errorf("outdated = %v, %v right after passages were cut", outdated, err)
		}

		if _, err := client.Querier(ctx).ExecContext(ctx,
			`UPDATE workspace_docs SET version = 2 WHERE id = $1`, docID); err != nil {
			return err
		}

		if outdated, err := passages.Outdated(ctx, workspaceID, 10); err != nil || !slices.Equal(outdated, []uuid.UUID{docID}) {
			t.Errorf("outdated = %v, %v; passages cut from version 1 are outdated at version 2", outdated, err)
		}

		if err := passages.Replace(ctx, doc); err != nil {
			return err
		}

		var stored []string

		rows, err := client.Querier(ctx).QueryContext(ctx, `
SELECT doc_version || ':' || heading || ':' || body FROM workspace_doc_passages WHERE doc_id = $1 ORDER BY position`, docID)
		if err != nil {
			return err
		}

		defer func() { _ = rows.Close() }()

		for rows.Next() {
			var row string
			if err := rows.Scan(&row); err != nil {
				return err
			}

			stored = append(stored, row)
		}

		if !slices.Equal(stored, []string{"2:Runbook › Deploys:Ship on Wednesdays."}) {
			t.Errorf("stored = %q, want only the passages of the latest version", stored)
		}

		return errPassageRollback
	})
	if !errors.Is(err, errPassageRollback) {
		t.Fatalf("round trip: %v", err)
	}
}
