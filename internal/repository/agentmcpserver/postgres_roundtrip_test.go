package agentmcpserver

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
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
	"github.com/usenorn/norn/internal/pkg/crypter"
	"github.com/usenorn/norn/internal/pkg/postgres"
)

const liveToken = "sntrys_live_roundtrip_0123"

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

func sealer(t *testing.T) *crypter.Crypter {
	t.Helper()

	key := make([]byte, crypter.KeyBytes)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate key: %v", err)
	}

	sealing, err := crypter.New(config.Security{EncryptionKey: base64.StdEncoding.EncodeToString(key)})
	if err != nil {
		t.Fatalf("build crypter: %v", err)
	}

	return sealing
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
		Slug:         "mcp-" + f.workspaceID.String()[:8],
		Name:         "MCP",
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

func (f fixture) server(agentID *uuid.UUID, name string) entity.AgentMCPServer {
	return entity.AgentMCPServer{
		ID:          uuid.New(),
		WorkspaceID: f.workspaceID,
		AgentID:     agentID,
		Name:        name,
		Transport:   entity.AgentMCPStdio,
		Command:     "npx",
		Args:        []string{"-y", "@sentry/mcp-server"},
		Auth:        entity.AgentMCPAuthNone,
		CreatedBy:   f.accountID,
	}
}

func TestSecretsAreSealedAtRestAndOnlyTheirNamesAreListed(t *testing.T) {
	db := reach(t)
	servers := New(db, sealer(t))

	err := db.WithTx(context.Background(), func(ctx context.Context) error {
		f := seed(ctx, t, db, 1)

		created, err := servers.Create(ctx, f.server(&f.agents[0], "sentry"), entity.AgentMCPSecrets{
			Env: map[string]string{"SENTRY_TOKEN": liveToken, "SENTRY_HOST": "sentry.test"},
		})
		if err != nil {
			return err
		}

		if !slices.Equal(created.EnvKeys, []string{"SENTRY_HOST", "SENTRY_TOKEN"}) {
			t.Errorf("env keys = %v, want both names sorted", created.EnvKeys)
		}

		row, err := dbpostgres.FindWorkspaceAgentMCPServer(ctx, db.Querier(ctx), created.ID.String())
		if err != nil {
			return err
		}

		if bytes.Contains(row.SecretsSealed.Bytes, []byte(liveToken)) {
			t.Error("the stored column holds the token in the clear")
		}

		opened, err := servers.Secrets(ctx, f.workspaceID, created.ID)
		if err != nil {
			return err
		}

		if opened.Env["SENTRY_TOKEN"] != liveToken {
			t.Errorf("opened env = %v", opened.Env)
		}

		if _, err := servers.Secrets(ctx, uuid.New(), created.ID); !errors.Is(err, entity.ErrAgentMCPServerNotFound) {
			t.Errorf("another workspace read the secrets: err = %v", err)
		}

		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("round trip: %v", err)
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

func TestAnAgentSeesItsOwnServersAndTheLibraryServersAttachedToIt(t *testing.T) {
	db := reach(t)
	servers := New(db, sealer(t))

	rolledBack(t, db, func(ctx context.Context) error {
		f := seed(ctx, t, db, 2)
		first, second := f.agents[0], f.agents[1]

		if _, err := servers.Create(ctx, f.server(&first, "sentry"), entity.AgentMCPSecrets{}); err != nil {
			return err
		}

		if _, err := servers.Create(ctx, f.server(&second, "sentry"), entity.AgentMCPSecrets{}); err != nil {
			t.Errorf("two agents could not each have a server named sentry: %v", err)
		}

		library, err := servers.Create(ctx, f.server(nil, "linear"), entity.AgentMCPSecrets{})
		if err != nil {
			return err
		}

		if err := servers.Attach(ctx, entity.AgentCapabilityAttachment{
			WorkspaceID: f.workspaceID, AgentID: first, CapabilityID: library.ID, AttachedBy: f.accountID,
		}); err != nil {
			return err
		}

		listed, err := servers.ListByAgent(ctx, f.workspaceID, first)
		if err != nil {
			return err
		}

		names := []string{}
		for _, server := range listed {
			names = append(names, server.Name)
		}

		if !slices.Equal(names, []string{"linear", "sentry"}) || listed[0].AgentID != nil {
			t.Errorf("first agent lists %v, want the attached library server and its own", names)
		}

		attached, err := servers.AttachedAgents(ctx, f.workspaceID)
		if err != nil {
			return err
		}

		if !slices.Equal(attached[library.ID], []uuid.UUID{first}) {
			t.Errorf("attached = %v", attached)
		}

		if err := servers.Detach(ctx, f.workspaceID, second, library.ID); !errors.Is(err, entity.ErrAgentCapabilityNotAttached) {
			t.Errorf("detaching what was never attached: err = %v", err)
		}

		return nil
	})
}

func TestOneAgentCannotHoldTwoServersWithOneName(t *testing.T) {
	db := reach(t)
	servers := New(db, sealer(t))

	rolledBack(t, db, func(ctx context.Context) error {
		f := seed(ctx, t, db, 1)

		if _, err := servers.Create(ctx, f.server(&f.agents[0], "sentry"), entity.AgentMCPSecrets{}); err != nil {
			return err
		}

		if _, err := servers.Create(ctx, f.server(&f.agents[0], "Sentry"), entity.AgentMCPSecrets{}); !errors.Is(err, entity.ErrAgentMCPServerNameTaken) {
			t.Errorf("a second sentry on one agent: err = %v", err)
		}

		return nil
	})
}

func TestALibraryServerIsAttachedToAnAgentOnce(t *testing.T) {
	db := reach(t)
	servers := New(db, sealer(t))

	rolledBack(t, db, func(ctx context.Context) error {
		f := seed(ctx, t, db, 1)

		library, err := servers.Create(ctx, f.server(nil, "linear"), entity.AgentMCPSecrets{})
		if err != nil {
			return err
		}

		attachment := entity.AgentCapabilityAttachment{
			WorkspaceID: f.workspaceID, AgentID: f.agents[0], CapabilityID: library.ID, AttachedBy: f.accountID,
		}
		if err := servers.Attach(ctx, attachment); err != nil {
			return err
		}

		if err := servers.Attach(ctx, attachment); !errors.Is(err, entity.ErrAgentCapabilityAttached) {
			t.Errorf("attaching twice: err = %v", err)
		}

		return nil
	})
}
