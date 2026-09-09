package intake_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal"
	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
	intakerepo "github.com/usenorn/norn/internal/repository/intake"
)

type live struct {
	client    *postgres.Client
	intake    repository.Intake
	workspace uuid.UUID
	team      uuid.UUID
}

func liveIntake(t *testing.T) *live {
	t.Helper()

	dsn := os.Getenv("NORN_TEST_POSTGRES_DSN")
	if os.Getenv("NORN_TEST_INTEGRATION") != "true" || dsn == "" {
		t.Skip("set NORN_TEST_INTEGRATION=true and NORN_TEST_POSTGRES_DSN to run this")
	}

	name := fmt.Sprintf("norn_intake_%d", time.Now().UnixNano())

	scratch, err := scratchDSN(dsn, name)
	if err != nil {
		t.Fatalf("%v", err)
	}

	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open the server: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+name); err != nil {
		_ = admin.Close()

		t.Fatalf("create the scratch database: %v", err)
	}

	t.Cleanup(func() {
		done, stop := context.WithTimeout(context.Background(), 30*time.Second)
		defer stop()

		if _, err := admin.ExecContext(done, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("the scratch database %s was left behind: %v", name, err)
		}

		if err := admin.Close(); err != nil {
			t.Errorf("close the server connection: %v", err)
		}
	})

	client, cleanup, err := postgres.New(config.Postgres{
		DSN:             scratch,
		MaxConns:        8,
		MinConns:        2,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
		ConnectTimeout:  10 * time.Second,
	})
	if err != nil {
		t.Fatalf("connect to the scratch database: %v", err)
	}

	t.Cleanup(cleanup)

	var reached string

	if err := client.DB.QueryRowContext(ctx, "SELECT current_database()").Scan(&reached); err != nil {
		t.Fatalf("read which database the connection reached: %v", err)
	}

	if reached != name {
		t.Fatalf("the connection reached %q, want the scratch database %q", reached, name)
	}

	migrator, err := internal.NewMigrator(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("build the migrator: %v", err)
	}

	if err := migrator.Run(ctx); err != nil {
		t.Fatalf("migrate the scratch database: %v", err)
	}

	held := &live{client: client, intake: intakerepo.New(client), workspace: uuid.New(), team: uuid.New()}

	if _, err := client.ExecContext(ctx,
		"INSERT INTO workspaces (id, slug, name) VALUES ($1, $2, $3)",
		held.workspace.String(), "intake-"+held.workspace.String()[:8], "Intake",
	); err != nil {
		t.Fatalf("seed the workspace: %v", err)
	}

	if _, err := client.ExecContext(ctx,
		"INSERT INTO workspace_teams (id, workspace_id, key, name) VALUES ($1, $2, $3, $4)",
		held.team.String(), held.workspace.String(), "COR", "Core",
	); err != nil {
		t.Fatalf("seed the team: %v", err)
	}

	return held
}

func scratchDSN(dsn, name string) (string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		return "", errors.New(
			"NORN_TEST_POSTGRES_DSN must be a postgres:// URL so the database name can be replaced " +
				"safely; running against the original would write fixtures into it",
		)
	}

	parsed.Path = "/" + name

	return parsed.String(), nil
}

func (h *live) record(t *testing.T, externalID string) uuid.UUID {
	t.Helper()

	deliveryID, err := h.intake.Record(context.Background(), entity.IntakeDelivery{
		WorkspaceID: h.workspace,
		TeamID:      h.team,
		ExternalID:  externalID,
		Recipient:   "core-649848208d3e@submit.norn.so",
		Sender:      "rae@northwind.co",
		Subject:     "Export does nothing",
		ReceivedAt:  time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("record the delivery: %v", err)
	}

	return deliveryID
}

func TestARedeliveredMessageFindsTheDeliveryAlreadyStored(t *testing.T) {
	h := liveIntake(t)
	ctx := context.Background()

	deliveryID := h.record(t, "inb_live_1")

	_, err := h.intake.Record(ctx, entity.IntakeDelivery{
		WorkspaceID: h.workspace,
		TeamID:      h.team,
		ExternalID:  "inb_live_1",
		Recipient:   "core-649848208d3e@submit.norn.so",
		Sender:      "rae@northwind.co",
		ReceivedAt:  time.Now().UTC(),
	})
	if !errors.Is(err, entity.ErrIntakeDeliveryDuplicate) {
		t.Fatalf(
			"recording the same message twice answered %v, want the duplicate. The queue retry "+
				"leans on that answer to find the delivery it has to hand over again.",
			err,
		)
	}

	stored, err := h.intake.DeliveryOf(ctx, "inb_live_1")
	if err != nil {
		t.Fatalf("the stored delivery could not be found by the provider's id: %v", err)
	}

	if stored.ID != deliveryID || stored.ProcessedAt != nil {
		t.Fatalf(
			"found %v processed=%v, want the pending delivery %v. Mail whose queueing failed is "+
				"recovered through this row alone.",
			stored.ID, stored.ProcessedAt, deliveryID,
		)
	}
}

func TestSettlingIsUndoneWhenTheTransactionFails(t *testing.T) {
	h := liveIntake(t)
	ctx := context.Background()

	deliveryID := h.record(t, "inb_live_2")
	refused := errors.New("the work after settling failed")

	err := h.client.WithTx(ctx, func(ctx context.Context) error {
		if err := h.intake.Settle(
			ctx, deliveryID, entity.IntakeDeliveryFiled, uuid.Nil, "", time.Now().UTC(),
		); err != nil {
			return err
		}

		return refused
	})
	if !errors.Is(err, refused) {
		t.Fatalf("err=%v, want the refusal", err)
	}

	delivery, err := h.intake.Delivery(ctx, deliveryID)
	if err != nil {
		t.Fatalf("read the delivery back: %v", err)
	}

	if delivery.ProcessedAt != nil || delivery.Outcome.Settled() {
		t.Fatalf(
			"the delivery reads as %q after a failed transaction, want it still waiting. A "+
				"settlement that survives its own rollback strands the mail.",
			delivery.Outcome,
		)
	}
}

func TestTwoWorkersOnOneDeliveryFileItOnce(t *testing.T) {
	h := liveIntake(t)
	ctx := context.Background()

	deliveryID := h.record(t, "inb_live_3")

	var (
		wait  sync.WaitGroup
		mutex sync.Mutex
		filed int
	)

	refusals := make(chan error, 2)

	for range 2 {
		wait.Add(1)

		go func() {
			defer wait.Done()

			refusals <- h.client.WithTx(ctx, func(ctx context.Context) error {
				delivery, err := h.intake.LockDelivery(ctx, deliveryID)
				if err != nil {
					return err
				}

				if delivery.ProcessedAt != nil {
					return nil
				}

				// Long enough that the other worker is waiting on the lock rather than racing
				// past it, which is the whole point of taking one.
				time.Sleep(150 * time.Millisecond)

				mutex.Lock()
				filed++
				mutex.Unlock()

				return h.intake.Settle(
					ctx, deliveryID, entity.IntakeDeliveryFiled, uuid.Nil, "", time.Now().UTC(),
				)
			})
		}()
	}

	wait.Wait()
	close(refusals)

	for err := range refusals {
		if err != nil {
			t.Fatalf("a worker failed for a reason other than the race: %v", err)
		}
	}

	if filed != 1 {
		t.Fatalf(
			"%d workers filed the same message, want 1. Two attempts on one delivery must not "+
				"turn one email into two issues.",
			filed,
		)
	}
}
