package scm

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
	"github.com/usenorn/norn/internal/pkg/crypter"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

var errRollback = errors.New("roll the fixture back")

const roundtripKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

type ground struct {
	workspaceID uuid.UUID
	accountID   uuid.UUID
	appID       uuid.UUID
	connections repository.SCMConnection
	repository  repository.SCMRepository
	apps        repository.SCMApp
}

func lay(ctx context.Context, db *postgres.Client, sealer *crypter.Crypter) (ground, error) {
	on := ground{
		workspaceID: uuid.New(),
		accountID:   uuid.New(),
		appID:       uuid.New(),
		connections: NewSCMConnection(db, sealer),
		repository:  NewSCMRepository(db, sealer),
		apps:        NewSCMApp(db, sealer),
	}

	statements := []struct {
		what string
		sql  string
		args []any
	}{
		{
			what: "an account",
			sql: `INSERT INTO accounts (id, status, kind, email, display_name, timezone)
                  VALUES ($1, 'active', 'person', $2, 'Roundtrip', 'UTC')`,
			args: []any{on.accountID, on.accountID.String() + "@example.test"},
		},
		{
			what: "a workspace",
			sql:  `INSERT INTO workspaces (id, slug, name) VALUES ($1, $2, $2)`,
			args: []any{on.workspaceID, "roundtrip-" + on.workspaceID.String()[:8]},
		},
		{
			what: "an application",
			sql: `INSERT INTO scm_apps
                      (id, provider, external_app_id, private_key_sealed, webhook_secret_sealed)
                  VALUES ($1, 'github', '4711', '\x01'::bytea, '\x02'::bytea)`,
			args: []any{on.appID},
		},
	}

	for _, statement := range statements {
		if _, err := db.Querier(ctx).ExecContext(ctx, statement.sql, statement.args...); err != nil {
			return ground{}, fmt.Errorf("lay %s: %w", statement.what, err)
		}
	}

	return on, nil
}

func rolledBack(t *testing.T, run func(ctx context.Context, on ground) error) {
	t.Helper()

	dsn := strings.TrimSpace(os.Getenv("NORN_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("NORN_POSTGRES_DSN is unset, so there is no schema to write to")
	}

	sealer, err := crypter.New(config.Security{EncryptionKey: roundtripKey})
	if err != nil {
		t.Fatalf("build a crypter: %v", err)
	}

	db, cleanup, err := postgres.New(config.Postgres{
		DSN:             dsn,
		MaxConns:        4,
		MinConns:        1,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
		ConnectTimeout:  10 * time.Second,
	})
	if err != nil {
		t.Fatalf("reach postgres: %v", err)
	}

	defer cleanup()

	var failure error

	err = db.WithTx(context.Background(), func(ctx context.Context) error {
		on, err := lay(ctx, db, sealer)
		if err != nil {
			failure = err

			return errRollback
		}

		failure = run(ctx, on)

		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("the fixture was not rolled back: %v", err)
	}

	if failure != nil {
		t.Fatal(failure)
	}
}

func TestAConnectionOnAPastedTokenComesBackSayingSo(t *testing.T) {
	rolledBack(t, func(ctx context.Context, on ground) error {
		created, err := on.connections.Create(ctx, repository.SCMConnectionInput{
			Connection: entity.SCMConnection{
				WorkspaceID:          on.workspaceID,
				Provider:             entity.SCMProviderGitLab,
				AuthKind:             entity.SCMAuthToken,
				IntegrationAccountID: on.accountID,
				OwnerAccountID:       on.accountID,
				OwnerActorKind:       entity.ActorKindUser,
			},
			Token: "glpat-secret",
		})
		if err != nil {
			return fmt.Errorf("create a token connection: %w", err)
		}

		if created.AuthKind != entity.SCMAuthToken {
			return fmt.Errorf("auth kind came back %q, want %q", created.AuthKind, entity.SCMAuthToken)
		}

		if !created.TokenSet {
			return errors.New("a connection created with a token reports it has none")
		}

		read, err := on.connections.GetByID(ctx, on.workspaceID, created.ID)
		if err != nil {
			return fmt.Errorf("read it back: %w", err)
		}

		if read.UsesApp() {
			return errors.New("a token connection read back claims to be an installation")
		}

		token, err := on.connections.Token(ctx, created.ID)
		if err != nil {
			return fmt.Errorf("open its credential: %w", err)
		}

		if token != "glpat-secret" {
			return fmt.Errorf("the stored credential opened as %q, want the sealed token", token)
		}

		return nil
	})
}

func TestAConnectionOnAnInstallationSurvivesBeingReadBack(t *testing.T) {
	rolledBack(t, func(ctx context.Context, on ground) error {
		created, err := on.connections.Create(ctx, repository.SCMConnectionInput{
			Connection: installationConnection(on),
		})
		if err != nil {
			return fmt.Errorf(
				"create a connection on an installation: %w. It carries no token at all, so the "+
					"write path has to leave the sealed column empty rather than seal nothing",
				err,
			)
		}

		read, err := on.connections.GetByID(ctx, on.workspaceID, created.ID)
		if err != nil {
			return fmt.Errorf(
				"read an installation connection back: %w. Every screen and every sync starts by "+
					"reading the connection, so a row that cannot be scanned is a connection that "+
					"exists and can never be used",
				err,
			)
		}

		if !read.UsesApp() {
			return errors.New(
				"a connection stored as an installation read back as a pasted token, so nothing " +
					"downstream would ever mint an installation token for it",
			)
		}

		if read.AppID != on.appID || read.InstallationID != "884411" {
			return fmt.Errorf(
				"the installation came back as app %s / %q, want %s / %q",
				read.AppID, read.InstallationID, on.appID, "884411",
			)
		}

		if read.AccountLogin != "flagroll" {
			return fmt.Errorf("the account it acts for came back %q, want %q", read.AccountLogin, "flagroll")
		}

		if read.TokenSet {
			return errors.New(
				"an installation connection claims to hold a pasted token, so a screen would " +
					"offer to replace a credential that was never stored",
			)
		}

		listed, err := on.connections.ListByWorkspace(ctx, on.workspaceID)
		if err != nil {
			return fmt.Errorf("list the workspace connections: %w", err)
		}

		for _, found := range listed {
			if found.ID == created.ID && !found.UsesApp() {
				return errors.New(
					"the same connection reads as an installation one way and a pasted token the other",
				)
			}
		}

		return nil
	})
}

func TestARepositoryWithNoWebhookSecretOfItsOwnStillReads(t *testing.T) {
	rolledBack(t, func(ctx context.Context, on ground) error {
		connection, err := on.connections.Create(ctx, repository.SCMConnectionInput{
			Connection: installationConnection(on),
		})
		if err != nil {
			return fmt.Errorf("create the connection it hangs from: %w", err)
		}

		created, err := on.repository.Create(ctx, repository.SCMRepositoryInput{
			Repository: entity.SCMRepository{
				WorkspaceID:   on.workspaceID,
				ConnectionID:  connection.ID,
				Provider:      entity.SCMProviderGitHub,
				FullName:      "flagroll/platform",
				ExternalID:    "9001",
				DefaultBranch: "main",
				MirrorLabel:   "norn",
			},
		})
		if err != nil {
			return fmt.Errorf(
				"create a repository with no secret of its own: %w. An application signs every "+
					"delivery with one secret for all of its repositories",
				err,
			)
		}

		if _, err := on.repository.GetByID(ctx, on.workspaceID, created.ID); err != nil {
			return fmt.Errorf(
				"read it back: %w. The delivery edge reads the repository before it can verify "+
					"anything, so a row it cannot scan drops every webhook that arrives",
				err,
			)
		}

		return nil
	})
}

func installationConnection(on ground) entity.SCMConnection {
	return entity.SCMConnection{
		WorkspaceID:          on.workspaceID,
		Provider:             entity.SCMProviderGitHub,
		AuthKind:             entity.SCMAuthApp,
		AppID:                on.appID,
		InstallationID:       "884411",
		AccountLogin:         "flagroll",
		IntegrationAccountID: on.accountID,
		OwnerAccountID:       on.accountID,
		OwnerActorKind:       entity.ActorKindUser,
	}
}

func TestAnApplicationKeepsItsKeysAndTheTrustItWasGranted(t *testing.T) {
	rolledBack(t, func(ctx context.Context, on ground) error {
		stored, err := on.apps.Upsert(ctx, repository.SCMAppInput{
			App: entity.SCMApp{
				Provider:      entity.SCMProviderGitHub,
				BaseURL:       "https://ghe.northwind.example/api/v3",
				Slug:          "norn-northwind",
				ExternalAppID: "4711",
				ClientID:      "Iv1.deadbeef",
				Trust: entity.SCMTrust{
					AllowPrivateAddress: true,
					CACertificate:       "-----BEGIN CERTIFICATE-----",
				},
			},
			PrivateKey:    "-----BEGIN RSA PRIVATE KEY-----",
			WebhookSecret: "the-hook-secret",
			ClientSecret:  "the-client-secret",
		})
		if err != nil {
			return fmt.Errorf("register an application: %w", err)
		}

		if !stored.Trust.AllowPrivateAddress || stored.Trust.CACertificate == "" {
			return fmt.Errorf(
				"the trust came back as %+v. An enterprise instance on a private network is "+
					"unreachable without it, and the application would be stored unusable",
				stored.Trust,
			)
		}

		read, err := on.apps.Get(ctx, entity.SCMProviderGitHub, "https://ghe.northwind.example/api/v3")
		if err != nil {
			return fmt.Errorf("read it back: %w", err)
		}

		if read.Trust != stored.Trust || read.Slug != "norn-northwind" {
			return fmt.Errorf("the application read back as %+v", read)
		}

		if read.PrivateKey != "" || read.WebhookSecret != "" || read.ClientSecret != "" {
			return errors.New(
				"listing an application carried its secrets. Only the call that needs them should " +
					"open them, so an ordinary read cannot leak a key",
			)
		}

		secrets, err := on.apps.Secrets(ctx, stored.ID)
		if err != nil {
			return fmt.Errorf("open its secrets: %w", err)
		}

		if secrets.PrivateKey == "" || secrets.WebhookSecret == "" || secrets.ClientSecret == "" {
			return fmt.Errorf("a sealed secret did not open: %+v", secrets)
		}

		if secrets.Trust != stored.Trust {
			return fmt.Errorf(
				"the sealed read lost the trust (%+v). Minting an installation token goes through "+
					"it, so the connection would work until the first token expired",
				secrets.Trust,
			)
		}

		return nil
	})
}
