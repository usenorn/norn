package telegramaudience

import (
	"context"
	"errors"
	"os"
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
	agentID     uuid.UUID
	botID       uuid.UUID
}

func insert(ctx context.Context, t *testing.T, db *postgres.Client, model interface {
	Insert(context.Context, boil.ContextExecutor, boil.Columns) error
}) {
	t.Helper()

	if err := model.Insert(ctx, db.Querier(ctx), boil.Infer()); err != nil {
		t.Fatalf("insert fixture: %v", err)
	}
}

func account(ctx context.Context, t *testing.T, db *postgres.Client, kind entity.AccountKind, name string) uuid.UUID {
	t.Helper()

	id := uuid.New()
	now := time.Now().UTC()
	insert(ctx, t, db, &dbpostgres.Account{
		ID:          id.String(),
		Status:      string(entity.AccountStatusActive),
		Email:       null.StringFrom(id.String() + "@telegram.test"),
		DisplayName: null.StringFrom(name),
		Timezone:    null.StringFrom("UTC"),
		Kind:        string(kind),
		CreatedAt:   now,
		UpdatedAt:   now,
	})

	return id
}

func seed(ctx context.Context, t *testing.T, db *postgres.Client) fixture {
	t.Helper()

	now := time.Now().UTC()
	f := fixture{
		workspaceID: uuid.New(),
		accountID:   account(ctx, t, db, entity.AccountKindPerson, "Rae"),
		agentID:     uuid.New(),
		botID:       uuid.New(),
	}

	insert(ctx, t, db, &dbpostgres.Workspace{
		ID:           f.workspaceID.String(),
		Slug:         "tg-" + f.workspaceID.String()[:8],
		Name:         "Telegram",
		Status:       string(entity.WorkspaceStatusActive),
		Timezone:     "UTC",
		WeekStartsOn: string(entity.WeekDayMonday),
		CreatedAt:    now,
		UpdatedAt:    now,
	})

	insert(ctx, t, db, &dbpostgres.WorkspaceAgent{
		ID:             f.agentID.String(),
		WorkspaceID:    f.workspaceID.String(),
		AccountID:      account(ctx, t, db, entity.AccountKindAgent, "Ada").String(),
		OwnerAccountID: f.accountID.String(),
		Name:           "ada-" + f.agentID.String()[:8],
		Status:         string(entity.AgentStatusActive),
		Icon:           string(entity.AgentIconBot),
		CreatedAt:      now,
		UpdatedAt:      now,
	})

	insert(ctx, t, db, &dbpostgres.WorkspaceTelegramBot{
		ID:                f.botID.String(),
		WorkspaceID:       f.workspaceID.String(),
		AgentID:           f.agentID.String(),
		BotUserID:         int64(f.botID.ID()) + 1<<40,
		BotUsername:       "ada_bot",
		TokenSealed:       []byte("sealed"),
		WebhookSecretHash: []byte("hash"),
		ConnectedAt:       now,
		UpdatedAt:         now,
	})

	return f
}
