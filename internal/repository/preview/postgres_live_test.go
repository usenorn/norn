package preview

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

const (
	reportedEarly = "2026-08-23T10:00:00Z"
	reportedLate  = "2026-08-23T11:00:00Z"
)

func live(t *testing.T) (*postgres.Client, repository.Preview) {
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
		t.Skipf(
			"this database holds no execution to hang a preview off: %v. Migrate and seed a run "+
				"first",
			err,
		)
	}

	parsed, err := uuid.Parse(workspaceID)
	if err != nil {
		t.Fatalf("parse the workspace id: %v", err)
	}

	return executionID, parsed
}

func at(stamp string) time.Time {
	parsed, err := time.Parse(time.RFC3339, stamp)
	if err != nil {
		panic(err)
	}

	return parsed
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

func opened(executionID string, workspaceID uuid.UUID, name, stamp string) entity.PreviewSession {
	return entity.PreviewSession{
		ExecutionID: executionID,
		WorkspaceID: workspaceID,
		Name:        name,
		Service:     name,
		Mode:        entity.PreviewBySubdomain,
		Host:        name + "-" + executionID + ".preview.example.test",
		State:       entity.PreviewOpen,
		OpenedAt:    at(stamp),
		ReportedAt:  at(stamp),
	}
}

func TestAReportThatArrivedLateNeverReopensAPreviewThatWasAlreadyClosed(t *testing.T) {
	client, previews := live(t)
	executionID, workspaceID := heldRun(t, client)

	rolledBack(t, client, func(ctx context.Context) error {
		if _, err := previews.Save(ctx, opened(executionID, workspaceID, "web", reportedEarly)); err != nil {
			return fmt.Errorf("open the preview: %w", err)
		}

		closing := opened(executionID, workspaceID, "web", reportedLate)
		closing.State = entity.PreviewClosed
		closing.ClosedAt = at(reportedLate)

		if _, err := previews.Save(ctx, closing); err != nil {
			return fmt.Errorf("close the preview: %w", err)
		}

		late := opened(executionID, workspaceID, "web", reportedEarly)

		stored, err := previews.Save(ctx, late)
		if err != nil {
			return fmt.Errorf("replay the opening report: %w", err)
		}

		if stored.State != entity.PreviewClosed {
			return fmt.Errorf(
				"a replayed opening report put the preview back to %s. Delivery is at-least-once, "+
					"so a reconnect would reopen a route to a service that is already gone",
				stored.State,
			)
		}

		return nil
	})
}

func TestClosingARunClosesEveryPreviewItStillHasOpen(t *testing.T) {
	client, previews := live(t)
	executionID, workspaceID := heldRun(t, client)

	rolledBack(t, client, func(ctx context.Context) error {
		for _, name := range []string{"web", "docs"} {
			if _, err := previews.Save(ctx, opened(executionID, workspaceID, name, reportedEarly)); err != nil {
				return fmt.Errorf("open %s: %w", name, err)
			}
		}

		if err := previews.CloseByExecution(ctx, executionID, at(reportedLate)); err != nil {
			return fmt.Errorf("close the run's previews: %w", err)
		}

		held, err := previews.ByExecution(ctx, executionID)
		if err != nil {
			return fmt.Errorf("read the previews back: %w", err)
		}

		for _, preview := range held {
			if preview.Open() {
				return fmt.Errorf(
					"%s is still open after the run stopped holding it. The machine that would "+
						"have said so is gone, so nothing else closes it and the address goes on "+
						"resolving",
					preview.Name,
				)
			}
		}

		return nil
	})
}

func TestAHostIsClaimedByOnePreviewAndNoOther(t *testing.T) {
	client, previews := live(t)
	executionID, workspaceID := heldRun(t, client)

	rolledBack(t, client, func(ctx context.Context) error {
		first := opened(executionID, workspaceID, "web", reportedEarly)

		if _, err := previews.Save(ctx, first); err != nil {
			return fmt.Errorf("open the first preview: %w", err)
		}

		second := opened(executionID, workspaceID, "docs", reportedEarly)
		second.Host = first.Host

		if _, err := previews.Save(ctx, second); err == nil {
			return errors.New(
				"two previews took the same host. The gateway routes by host alone, so the " +
					"second would answer for the first",
			)
		}

		return nil
	})
}
