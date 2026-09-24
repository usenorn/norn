package agentskill

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aarondl/null/v8"
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

type fixture struct {
	workspaceID uuid.UUID
	accountID   uuid.UUID
	agents      []uuid.UUID
}

func insert(ctx context.Context, t *testing.T, db *postgres.Client, model interface {
	Insert(context.Context, boil.ContextExecutor, boil.Columns) error
}) {
	t.Helper()

	if err := model.Insert(ctx, db.Querier(ctx), boil.Infer()); err != nil {
		t.Fatalf("insert fixture: %v", err)
	}
}

func account(ctx context.Context, t *testing.T, db *postgres.Client, kind string) uuid.UUID {
	t.Helper()

	id := uuid.New()
	now := time.Now().UTC()
	insert(ctx, t, db, &dbpostgres.Account{
		ID:          id.String(),
		Status:      string(entity.AccountStatusActive),
		Email:       null.StringFrom(id.String() + "@roundtrip.test"),
		DisplayName: null.StringFrom("Rae"),
		Timezone:    null.StringFrom("UTC"),
		Kind:        kind,
		CreatedAt:   now,
		UpdatedAt:   now,
	})

	return id
}

func seed(ctx context.Context, t *testing.T, db *postgres.Client, agents int) fixture {
	t.Helper()

	now := time.Now().UTC()
	f := fixture{workspaceID: uuid.New(), accountID: account(ctx, t, db, string(entity.AccountKindPerson))}

	insert(ctx, t, db, &dbpostgres.Workspace{
		ID:           f.workspaceID.String(),
		Slug:         "skl-" + f.workspaceID.String()[:8],
		Name:         "Skills",
		Status:       string(entity.WorkspaceStatusActive),
		Timezone:     "UTC",
		WeekStartsOn: string(entity.WeekDayMonday),
		CreatedAt:    now,
		UpdatedAt:    now,
	})

	for i := range agents {
		id := uuid.New()
		insert(ctx, t, db, &dbpostgres.WorkspaceAgent{
			ID:             id.String(),
			WorkspaceID:    f.workspaceID.String(),
			AccountID:      account(ctx, t, db, string(entity.AccountKindAgent)).String(),
			OwnerAccountID: f.accountID.String(),
			Name:           "agent-" + string(rune('a'+i)),
			Status:         string(entity.AgentStatusActive),
			Icon:           string(entity.AgentIconBot),
			CreatedAt:      now,
			UpdatedAt:      now,
		})
		f.agents = append(f.agents, id)
	}

	return f
}

func (f fixture) skill(agentID *uuid.UUID, name string) entity.AgentSkill {
	id := uuid.New()

	return entity.AgentSkill{
		ID:           id,
		WorkspaceID:  f.workspaceID,
		AgentID:      agentID,
		Name:         name,
		Description:  "Writes release notes.",
		Source:       entity.AgentSkillManual,
		Instructions: "---\nname: " + name + "\n---\n",
		ContentHash:  "4f2a",
		ObjectKey:    entity.AgentSkillObjectKey(f.workspaceID, id, "4f2a"),
		SizeBytes:    64,
		FileCount:    1,
		CreatedBy:    f.accountID,
	}
}

func rolledBack(t *testing.T, db *postgres.Client, check func(ctx context.Context) error) {
	t.Helper()

	if err := db.WithTx(context.Background(), func(ctx context.Context) error {
		if err := check(ctx); err != nil {
			return err
		}

		return errRollback
	}); !errors.Is(err, errRollback) {
		t.Fatalf("round trip: %v", err)
	}
}

func TestAnAgentSeesItsOwnSkillsAndTheLibrarySkillsAttachedToIt(t *testing.T) {
	db := reach(t)
	skills := New(db)

	rolledBack(t, db, func(ctx context.Context) error {
		f := seed(ctx, t, db, 2)
		first, second := f.agents[0], f.agents[1]

		for _, skill := range []entity.AgentSkill{
			f.skill(&first, "release-notes"),
			f.skill(&second, "triage"),
		} {
			if _, err := skills.Create(ctx, skill); err != nil {
				return err
			}
		}

		library, err := skills.Create(ctx, f.skill(nil, "house-style"))
		if err != nil {
			return err
		}

		if err := skills.Attach(ctx, entity.AgentCapabilityAttachment{
			WorkspaceID: f.workspaceID, AgentID: first, CapabilityID: library.ID, AttachedBy: f.accountID,
		}); err != nil {
			return err
		}

		listed, err := skills.ListByAgent(ctx, f.workspaceID, first)
		if err != nil {
			return err
		}

		names := []string{}
		for _, skill := range listed {
			names = append(names, skill.Name)
		}

		if !slices.Equal(names, []string{"house-style", "release-notes"}) {
			t.Errorf("first agent lists %v, want its own skill and the attached library skill", names)
		}

		owned, err := skills.CountOwned(ctx, f.workspaceID, &first)
		if err != nil {
			return err
		}

		inLibrary, err := skills.CountOwned(ctx, f.workspaceID, nil)
		if err != nil {
			return err
		}

		if owned != 1 || inLibrary != 1 {
			t.Errorf("owned %d library %d, want an attachment not to count against the agent's own limit", owned, inLibrary)
		}

		if err := skills.Delete(ctx, f.workspaceID, library.ID); err != nil {
			return err
		}

		after, err := skills.ListByAgent(ctx, f.workspaceID, first)
		if err != nil {
			return err
		}

		if len(after) != 1 {
			t.Errorf("after deleting the library skill the agent lists %d skills, want 1", len(after))
		}

		return nil
	})
}

func TestOneAgentCannotHoldTwoSkillsWithOneName(t *testing.T) {
	db := reach(t)
	skills := New(db)

	rolledBack(t, db, func(ctx context.Context) error {
		f := seed(ctx, t, db, 1)

		if _, err := skills.Create(ctx, f.skill(&f.agents[0], "release-notes")); err != nil {
			return err
		}

		if _, err := skills.Create(ctx, f.skill(&f.agents[0], "Release-Notes")); !errors.Is(err, entity.ErrAgentSkillNameTaken) {
			t.Errorf("err = %v, want ErrAgentSkillNameTaken", err)
		}

		return nil
	})
}
