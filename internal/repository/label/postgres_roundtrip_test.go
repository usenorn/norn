package label

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
)

var errRollback = errors.New("roll the fixture back")

func reach(t *testing.T) *postgres.Client {
	t.Helper()

	dsn := strings.TrimSpace(os.Getenv("NORN_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("NORN_POSTGRES_DSN is unset, so there is no schema to write to")
	}

	db, cleanup, err := postgres.New(config.Postgres{
		DSN:             dsn,
		MaxConns:        8,
		MinConns:        1,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
		ConnectTimeout:  10 * time.Second,
	})
	if err != nil {
		t.Fatalf("reach postgres: %v", err)
	}

	t.Cleanup(cleanup)

	return db
}

func fixtureWorkspace(ctx context.Context, t *testing.T, db *postgres.Client) uuid.UUID {
	t.Helper()

	id := uuid.New()
	now := time.Now().UTC()
	model := &dbpostgres.Workspace{
		ID:           id.String(),
		Slug:         "labels-" + id.String()[:8],
		Name:         "Labels",
		Status:       string(entity.WorkspaceStatusActive),
		Timezone:     "UTC",
		WeekStartsOn: string(entity.WeekDayMonday),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := model.Insert(ctx, db.Querier(ctx), boil.Infer()); err != nil {
		t.Fatalf("insert workspace fixture: %v", err)
	}

	return id
}

func TestALabelDescriptionSurvivesEveryPathThatReadsItBack(t *testing.T) {
	db := reach(t)
	labels := New(db)

	err := db.WithTx(context.Background(), func(ctx context.Context) error {
		workspaceID := fixtureWorkspace(ctx, t, db)

		created, err := labels.Create(ctx, entity.Label{
			WorkspaceID: workspaceID,
			Name:        "Bug",
			Description: "Something shipped is wrong",
			Color:       entity.LabelColorMagenta,
		})
		if err != nil {
			return err
		}

		if created.Description != "Something shipped is wrong" {
			t.Errorf("create returned description %q, want it carried through", created.Description)
		}

		read, err := labels.GetByID(ctx, workspaceID, created.ID)
		if err != nil {
			return err
		}

		if read.Description != created.Description {
			t.Errorf("GetByID description = %q, want %q", read.Description, created.Description)
		}

		listed, err := labels.ListByWorkspaceID(ctx, workspaceID, entity.TeamScope{AllTeams: true})
		if err != nil {
			return err
		}

		if len(listed) != 1 || listed[0].Description != created.Description {
			t.Errorf("listed %+v, want one label described %q", listed, created.Description)
		}

		updated, err := labels.UpdateSettings(ctx, created.ID, "Bug", "Blocked until product answers", entity.LabelColorOrchid, uuid.Nil)
		if err != nil {
			return err
		}

		if updated.Description != "Blocked until product answers" {
			t.Errorf("update returned description %q, want the new one", updated.Description)
		}

		cleared, err := labels.UpdateSettings(ctx, created.ID, "Bug", "", entity.LabelColorOrchid, uuid.Nil)
		if err != nil {
			return err
		}

		if cleared.Description != "" {
			t.Errorf("description after clearing = %q, want empty", cleared.Description)
		}

		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("the fixture was not rolled back: %v", err)
	}
}
