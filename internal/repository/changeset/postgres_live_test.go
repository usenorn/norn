package changeset

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

func live(t *testing.T) (*postgres.Client, repository.ChangeSet) {
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
			"this database holds no execution to hang a changeset off: %v. Migrate and seed a "+
				"run first",
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

func TestAnUpdateNamingOneRepositoryLeavesTheOtherRowsWhereTheyWere(t *testing.T) {
	client, changesets := live(t)
	executionID, workspaceID := heldRun(t, client)

	rolledBack(t, client, func(ctx context.Context) error {
		for _, name := range []string{"backend", "frontend"} {
			if _, err := changesets.SaveChange(ctx, entity.ExecutionChange{
				ExecutionID: executionID,
				WorkspaceID: workspaceID,
				Repository:  name,
				Branch:      "norn/NORN-38/" + name,
				Commits:     2,
				ReportedAt:  at(reportedEarly),
			}); err != nil {
				return fmt.Errorf("record %s: %w", name, err)
			}
		}

		if _, err := changesets.SaveChange(ctx, entity.ExecutionChange{
			ExecutionID: executionID,
			WorkspaceID: workspaceID,
			Repository:  "backend",
			Branch:      "norn/NORN-38/backend",
			Commits:     5,
			ReportedAt:  at(reportedLate),
		}); err != nil {
			return fmt.Errorf("record a later report for the backend: %w", err)
		}

		stored, err := changesets.Get(ctx, executionID)
		if err != nil {
			return fmt.Errorf("read the changeset back: %w", err)
		}

		if len(stored.Changes) != 2 {
			return fmt.Errorf(
				"an update naming only the backend left %d repositories on record, want 2; work "+
					"that already landed would vanish from the result as the run went on",
				len(stored.Changes),
			)
		}

		for _, change := range stored.Changes {
			if change.Repository == "backend" && change.Commits != 5 {
				return fmt.Errorf("the backend still reads %d commits", change.Commits)
			}
		}

		return nil
	})
}

func TestAReportReplayedAfterAReconnectNeverUndoesTheNewerOne(t *testing.T) {
	client, changesets := live(t)
	executionID, workspaceID := heldRun(t, client)

	rolledBack(t, client, func(ctx context.Context) error {
		newer := entity.ExecutionChange{
			ExecutionID: executionID,
			WorkspaceID: workspaceID,
			Repository:  "backend",
			Branch:      "norn/NORN-38/backend",
			Commits:     5,
			ReportedAt:  at(reportedLate),
		}

		if _, err := changesets.SaveChange(ctx, newer); err != nil {
			return fmt.Errorf("record the newer report: %w", err)
		}

		older := newer
		older.Branch = "stale"
		older.Commits = 2
		older.ReportedAt = at(reportedEarly)

		settled, err := changesets.SaveChange(ctx, older)
		if err != nil {
			return fmt.Errorf(
				"a replayed older report answered %w; delivery is at-least-once, so a replay has "+
					"to be a no-op rather than an error that refuses the whole connection",
				err,
			)
		}

		if settled.Commits != 5 || settled.Branch != "norn/NORN-38/backend" {
			return fmt.Errorf(
				"a message replayed after a reconnect rolled the row back to %d commits on %q; "+
					"the newer report has to win, and the caller has to be handed what is "+
					"actually on record",
				settled.Commits, settled.Branch,
			)
		}

		return nil
	})
}

func TestValidationResultsAreKeptPerCheckSoOneNeverOverwritesAnother(t *testing.T) {
	client, changesets := live(t)
	executionID, workspaceID := heldRun(t, client)

	rolledBack(t, client, func(ctx context.Context) error {
		for _, check := range []string{"backend tests", "frontend tests"} {
			if _, err := changesets.SaveValidation(ctx, entity.ExecutionValidation{
				ExecutionID: executionID,
				WorkspaceID: workspaceID,
				Check:       check,
				Status:      entity.ValidationPassed,
				ReportedAt:  at(reportedEarly),
			}); err != nil {
				return fmt.Errorf("record %s: %w", check, err)
			}
		}

		if _, err := changesets.SaveValidation(ctx, entity.ExecutionValidation{
			ExecutionID: executionID,
			WorkspaceID: workspaceID,
			Check:       "backend tests",
			Status:      entity.ValidationFailed,
			Detail:      "one case regressed",
			ReportedAt:  at(reportedLate),
		}); err != nil {
			return fmt.Errorf("record a later verdict: %w", err)
		}

		stored, err := changesets.Get(ctx, executionID)
		if err != nil {
			return fmt.Errorf("read the changeset back: %w", err)
		}

		if len(stored.Validations) != 2 {
			return fmt.Errorf("%d validation results survived, want 2", len(stored.Validations))
		}

		for _, validation := range stored.Validations {
			if validation.Check == "backend tests" && validation.Status != entity.ValidationFailed {
				return fmt.Errorf(
					"the backend check still reads %q; a person reviewing would be told the tests "+
						"passed after they had started failing",
					validation.Status,
				)
			}
		}

		return nil
	})
}

func TestALinkedPullRequestSurvivesTheNextReportOfTheSameRepository(t *testing.T) {
	client, changesets := live(t)
	executionID, workspaceID := heldRun(t, client)

	rolledBack(t, client, func(ctx context.Context) error {
		opened := "https://github.com/usenorn/norn/pull/231"

		if _, err := changesets.SaveChange(ctx, entity.ExecutionChange{
			ExecutionID:    executionID,
			WorkspaceID:    workspaceID,
			Repository:     "backend",
			PullRequestURL: opened,
			ReportedAt:     at(reportedEarly),
		}); err != nil {
			return fmt.Errorf("record a change carrying a pull request: %w", err)
		}

		later, err := changesets.SaveChange(ctx, entity.ExecutionChange{
			ExecutionID: executionID,
			WorkspaceID: workspaceID,
			Repository:  "backend",
			Commits:     9,
			ReportedAt:  at(reportedLate),
		})
		if err != nil {
			return fmt.Errorf("record a later report with no pull request on it: %w", err)
		}

		if later.PullRequestURL != opened {
			return fmt.Errorf(
				"a later report that said nothing about the pull request left %q; the machine "+
					"reports the address once, when it opens it, so every commit after that "+
					"would drop the only link a reviewer has",
				later.PullRequestURL,
			)
		}

		return nil
	})
}
