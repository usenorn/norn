package executionsnapshot

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
	"github.com/usenorn/norn/internal/repository"
)

var errRollback = errors.New("this test never keeps what it wrote")

func live(t *testing.T) (*postgres.Client, repository.ExecutionSnapshot) {
	t.Helper()

	dsn := strings.TrimSpace(os.Getenv("NORN_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("NORN_POSTGRES_DSN is unset, so there is no database to run these statements against")
	}

	client, closePool, err := postgres.New(config.Postgres{
		DSN:             dsn,
		MaxConns:        2,
		MinConns:        1,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
		ConnectTimeout:  10 * time.Second,
	})
	if err != nil {
		t.Skipf("no database at the configured dsn: %v", err)
	}

	t.Cleanup(closePool)

	return client, New(client)
}

func heldRun(t *testing.T, client *postgres.Client) (string, uuid.UUID) {
	t.Helper()

	ctx := context.Background()

	var executionID, workspaceID string

	err := client.Querier(ctx).
		QueryRowContext(ctx, `SELECT id, workspace_id FROM workspace_executions LIMIT 1`).
		Scan(&executionID, &workspaceID)
	if err != nil {
		t.Skipf("this database holds no execution to hang a snapshot off: %v", err)
	}

	parsed, err := uuid.Parse(workspaceID)
	if err != nil {
		t.Fatalf("parse the workspace id: %v", err)
	}

	return executionID, parsed
}

func rolledBack(t *testing.T, client *postgres.Client, fn func(context.Context) error) {
	t.Helper()

	var failure error

	err := client.WithTx(context.Background(), func(ctx context.Context) error {
		failure = fn(ctx)

		return errRollback
	})

	if !errors.Is(err, errRollback) {
		t.Fatalf("the fixture was not rolled back: %v", err)
	}

	if failure != nil {
		t.Fatal(failure)
	}
}

func snapshotAt(executionID string, workspaceID uuid.UUID, revision int, head string) entity.ExecutionSnapshot {
	return entity.ExecutionSnapshot{
		ExecutionID: executionID,
		WorkspaceID: workspaceID,
		Revision:    revision,
		Summary:     fmt.Sprintf("pass %d", revision),
		ReportedAt:  time.Date(2026, 9, 30, 10, revision, 0, 0, time.UTC),
		Repositories: []entity.SnapshotRepository{{
			Repository: "backend",
			Branch:     "norn/NORN-230/backend",
			BaseSHA:    "base",
			HeadSHA:    head,
			Commits:    []entity.SnapshotCommit{{SHA: head, Subject: "add review snapshot"}},
			Additions:  revision * 10,
			Deletions:  revision,
		}},
		Previews: []entity.SnapshotPreview{
			{Name: "Application", Service: "web", State: entity.SnapshotPreviewReady, Port: 41000},
			{
				Name:    "Database",
				Service: "postgres",
				State:   entity.SnapshotPreviewUnsupported,
				Reason:  "compose services are not supported",
			},
		},
	}
}

func TestARedeliveredSnapshotKeepsTheRevisionAsFirstRecorded(t *testing.T) {
	client, snapshots := live(t)
	executionID, workspaceID := heldRun(t, client)

	rolledBack(t, client, func(ctx context.Context) error {
		if _, err := snapshots.Record(ctx, snapshotAt(executionID, workspaceID, 1, "first")); err != nil {
			return fmt.Errorf("record the first pass: %w", err)
		}

		replayed, err := snapshots.Record(ctx, snapshotAt(executionID, workspaceID, 1, "replayed"))
		if err != nil {
			return fmt.Errorf("record the redelivery: %w", err)
		}

		if got := replayed.Repositories[0].HeadSHA; got != "first" {
			return fmt.Errorf("a redelivered revision rewrote the head to %q; comments on it would move", got)
		}

		if len(replayed.Previews) != 2 || replayed.Previews[1].State != entity.SnapshotPreviewUnsupported {
			return fmt.Errorf("the previews came back as %+v", replayed.Previews)
		}

		if len(replayed.Repositories[0].Commits) != 1 ||
			replayed.Repositories[0].Commits[0].Subject != "add review snapshot" {
			return fmt.Errorf("the commits came back as %+v", replayed.Repositories[0].Commits)
		}

		return nil
	})
}

func TestTheLatestSnapshotIsTheHighestRevision(t *testing.T) {
	client, snapshots := live(t)
	executionID, workspaceID := heldRun(t, client)

	rolledBack(t, client, func(ctx context.Context) error {
		for revision, head := range []string{"first", "second"} {
			if _, err := snapshots.Record(
				ctx, snapshotAt(executionID, workspaceID, revision+1, head),
			); err != nil {
				return fmt.Errorf("record revision %d: %w", revision+1, err)
			}
		}

		latest, err := snapshots.Latest(ctx, executionID)
		if err != nil {
			return fmt.Errorf("read the latest: %w", err)
		}

		if latest.Revision != 2 || latest.Repositories[0].HeadSHA != "second" {
			return fmt.Errorf("the latest is revision %d at %s", latest.Revision, latest.Repositories[0].HeadSHA)
		}

		revisions, err := snapshots.Revisions(ctx, executionID)
		if err != nil {
			return fmt.Errorf("list revisions: %w", err)
		}

		if len(revisions) != 2 || revisions[1].Additions != 20 {
			return fmt.Errorf("the revisions read %+v", revisions)
		}

		if _, err := snapshots.ByRevision(ctx, executionID, 3); !errors.Is(err, entity.ErrExecutionSnapshotNotFound) {
			return fmt.Errorf("an unknown revision answered %v", err)
		}

		return nil
	})
}
