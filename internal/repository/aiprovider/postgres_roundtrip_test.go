package aiprovider

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

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/crypter"
	"github.com/usenorn/norn/internal/pkg/postgres"
)

const plaintextKey = "sk-proj-roundtrip-abcd1234"

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

func fixtureWorkspace(ctx context.Context, t *testing.T, db *postgres.Client) uuid.UUID {
	t.Helper()

	id := uuid.New()
	now := time.Now().UTC()
	model := &dbpostgres.Workspace{
		ID:           id.String(),
		Slug:         "ai-" + id.String()[:8],
		Name:         "AI provider",
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

func TestAKeyIsSealedAtRestAndOnlyItsOwnWorkspaceReadsItBack(t *testing.T) {
	db := reach(t)
	providers := New(db, sealer(t))

	err := db.WithTx(context.Background(), func(ctx context.Context) error {
		owner := fixtureWorkspace(ctx, t, db)
		neighbour := fixtureWorkspace(ctx, t, db)

		saved, err := providers.Save(ctx, entity.AIProviderConnection{
			WorkspaceID:  owner,
			Provider:     entity.AIProviderOpenAI,
			DefaultModel: "gpt-6-luna",
		}, plaintextKey)
		if err != nil {
			return err
		}

		if saved.KeyHint != "1234" || saved.Status() != entity.AIProviderUnverified {
			t.Errorf("saved hint %q status %q, want 1234 and unverified", saved.KeyHint, saved.Status())
		}

		row, err := dbpostgres.FindWorkspaceAiProvider(ctx, db.Querier(ctx), owner.String())
		if err != nil {
			return err
		}

		if bytes.Contains(row.APIKeySealed, []byte(plaintextKey)) {
			t.Error("the stored column holds the key in the clear")
		}

		opened, err := providers.APIKey(ctx, owner)
		if err != nil {
			return err
		}

		if opened != plaintextKey {
			t.Errorf("opened key %q, want the key that was saved", opened)
		}

		if _, err := providers.APIKey(ctx, neighbour); !errors.Is(err, entity.ErrAIProviderNotConfigured) {
			t.Errorf("neighbour read: err = %v, want ErrAIProviderNotConfigured", err)
		}

		if err := providers.Delete(ctx, neighbour); !errors.Is(err, entity.ErrAIProviderNotConfigured) {
			t.Errorf("neighbour delete: err = %v, want ErrAIProviderNotConfigured", err)
		}

		if _, err := providers.Get(ctx, owner); err != nil {
			t.Errorf("the owner's provider went missing after the neighbour's delete: %v", err)
		}

		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("round trip: %v", err)
	}
}

func TestTheRecordedOutcomeFollowsTheLatestTestAndClearsOnChange(t *testing.T) {
	db := reach(t)
	providers := New(db, sealer(t))

	err := db.WithTx(context.Background(), func(ctx context.Context) error {
		workspaceID := fixtureWorkspace(ctx, t, db)

		if _, err := providers.Save(ctx, entity.AIProviderConnection{
			WorkspaceID: workspaceID, Provider: entity.AIProviderOpenAI, DefaultModel: "gpt-6-luna",
		}, plaintextKey); err != nil {
			return err
		}

		now := time.Now().UTC()

		if err := providers.MarkFailed(ctx, workspaceID, entity.AIProviderFailureModelUnavailable, now); err != nil {
			return err
		}

		failed, err := providers.Get(ctx, workspaceID)
		if err != nil {
			return err
		}

		if failed.Status() != entity.AIProviderFailed || failed.Failure != entity.AIProviderFailureModelUnavailable {
			t.Errorf("after a failed test: status %q failure %q", failed.Status(), failed.Failure)
		}

		if err := providers.MarkVerified(ctx, workspaceID, now); err != nil {
			return err
		}

		verified, err := providers.Get(ctx, workspaceID)
		if err != nil {
			return err
		}

		if verified.Status() != entity.AIProviderVerified || verified.Failure != "" {
			t.Errorf("after a passing test: status %q failure %q", verified.Status(), verified.Failure)
		}

		gateway := entity.AIProviderEndpoint{BaseURL: "http://10.0.4.12:8000/v1", AllowPrivateAddress: true}

		changed, err := providers.Update(ctx, entity.AIProviderConnection{
			WorkspaceID:  workspaceID,
			Provider:     entity.AIProviderOpenAI,
			Endpoint:     gateway,
			DefaultModel: "gpt-6-sol",
		})
		if err != nil {
			return err
		}

		if changed.DefaultModel != "gpt-6-sol" || changed.Endpoint != gateway ||
			changed.Status() != entity.AIProviderUnverified {
			t.Errorf(
				"after a change: model %q endpoint %v status %q",
				changed.DefaultModel, changed.Endpoint, changed.Status(),
			)
		}

		if key, err := providers.APIKey(ctx, workspaceID); err != nil || key != plaintextKey {
			t.Errorf("a model change disturbed the key: %q, %v", key, err)
		}

		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("round trip: %v", err)
	}
}

func TestWithoutAnEncryptionKeyNothingIsStored(t *testing.T) {
	db := reach(t)

	unkeyed, err := crypter.New(config.Security{})
	if err != nil {
		t.Fatalf("build crypter: %v", err)
	}

	providers := New(db, unkeyed)

	err = db.WithTx(context.Background(), func(ctx context.Context) error {
		workspaceID := fixtureWorkspace(ctx, t, db)

		_, err := providers.Save(ctx, entity.AIProviderConnection{
			WorkspaceID: workspaceID, Provider: entity.AIProviderOpenAI,
		}, plaintextKey)
		if !errors.Is(err, entity.ErrAIProviderEncryptionKeyMissing) {
			t.Errorf("err = %v, want ErrAIProviderEncryptionKeyMissing", err)
		}

		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("round trip: %v", err)
	}
}
