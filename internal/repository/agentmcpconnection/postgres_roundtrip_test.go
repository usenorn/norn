package agentmcpconnection

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
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
	"github.com/usenorn/norn/internal/pkg/crypter"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository/agentmcpserver"
)

const liveToken = "at-live-roundtrip-0123"

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
		Slug:         "con-" + f.workspaceID.String()[:8],
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

func TestTokensAreSealedAndASecondSignInReplacesTheFirst(t *testing.T) {
	db := reach(t)
	sealing := sealer(t)
	connections := New(db, sealing)
	servers := agentmcpserver.New(db, sealing)

	rolledBack(t, db, func(ctx context.Context) error {
		f := seed(ctx, t, db, 1)

		server, err := servers.Create(ctx, entity.AgentMCPServer{
			ID:          uuid.New(),
			WorkspaceID: f.workspaceID,
			AgentID:     &f.agents[0],
			Name:        "linear",
			Transport:   entity.AgentMCPHTTP,
			URL:         "https://mcp.linear.test/mcp",
			Auth:        entity.AgentMCPAuthOAuth,
			CreatedBy:   f.accountID,
		}, entity.AgentMCPSecrets{})
		if err != nil {
			return err
		}

		expiry := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
		connection := entity.AgentMCPConnection{
			ServerID:      server.ID,
			WorkspaceID:   f.workspaceID,
			Issuer:        "https://auth.linear.test",
			TokenEndpoint: "https://auth.linear.test/token",
			ClientID:      "client-norn",
			ConnectedBy:   f.accountID,
			ConnectedAt:   time.Now().UTC(),
		}

		if _, err := connections.Save(ctx, connection, entity.AgentMCPTokens{
			AccessToken: liveToken, RefreshToken: "rt-1", ExpiresAt: &expiry,
		}); err != nil {
			return err
		}

		row, err := dbpostgres.FindWorkspaceAgentMCPConnection(ctx, db.Querier(ctx), server.ID.String())
		if err != nil {
			return err
		}

		if bytes.Contains(row.AccessTokenSealed, []byte(liveToken)) {
			t.Error("the access token is stored in the clear")
		}

		if err := connections.MarkFailed(ctx, f.workspaceID, server.ID, entity.AgentMCPFailureRefreshRejected, time.Now().UTC()); err != nil {
			return err
		}

		if _, err := connections.Save(ctx, connection, entity.AgentMCPTokens{AccessToken: "at-2"}); err != nil {
			return err
		}

		saved, err := connections.Get(ctx, f.workspaceID, server.ID)
		if err != nil {
			return err
		}

		if saved.Status != entity.AgentMCPConnected || saved.Failure != "" || saved.ExpiresAt != nil {
			t.Errorf("after signing in again: %+v, want a clean connected row", saved)
		}

		tokens, err := connections.Tokens(ctx, f.workspaceID, server.ID)
		if err != nil {
			return err
		}

		if tokens.AccessToken != "at-2" || tokens.RefreshToken != "" {
			t.Errorf("tokens = %+v, want only the second sign-in's", tokens)
		}

		if err := servers.Delete(ctx, f.workspaceID, server.ID); err != nil {
			return err
		}

		if _, err := connections.Get(ctx, f.workspaceID, server.ID); !errors.Is(err, entity.ErrAgentMCPConnectionNotFound) {
			t.Errorf("a deleted server kept its tokens: err = %v", err)
		}

		return nil
	})
}
