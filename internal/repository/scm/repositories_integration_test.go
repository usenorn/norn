package scm_test

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal"
	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/crypter"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
	scmrepo "github.com/usenorn/norn/internal/repository/scm"
)

func scratchDatabase(t *testing.T) *postgres.Client {
	t.Helper()

	dsn := os.Getenv("NORN_TEST_POSTGRES_DSN")
	if os.Getenv("NORN_TEST_INTEGRATION") != "true" || dsn == "" {
		t.Skip("set NORN_TEST_INTEGRATION=true and NORN_TEST_POSTGRES_DSN to run this")
	}

	name := fmt.Sprintf("norn_scm_%d", time.Now().UnixNano())

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
		MaxConns:        4,
		MinConns:        1,
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
		t.Fatalf(
			"the connection reached %q, want the scratch database %q. Migrating anything other "+
				"than the database this test created would write fixtures into somebody's data.",
			reached, name,
		)
	}

	migrator, err := internal.NewMigrator(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("build the migrator: %v", err)
	}

	if err := migrator.Run(ctx); err != nil {
		t.Fatalf("migrate the scratch database: %v", err)
	}

	return client
}

func scratchDSN(dsn, name string) (string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		return "", fmt.Errorf("NORN_TEST_POSTGRES_DSN is not a postgres url: %q", dsn)
	}

	if strings.ContainsAny(name, `"; `) {
		return "", fmt.Errorf("refusing to build a database named %q", name)
	}

	parsed.Path = "/" + name

	return parsed.String(), nil
}

func connectedRepository(
	t *testing.T,
	client *postgres.Client,
) (repository.SCMRepository, entity.SCMRepository) {
	t.Helper()

	ctx := context.Background()

	var (
		workspaceID  = uuid.New()
		connectionID = uuid.New()
		accountID    = uuid.New()
	)

	if _, err := client.ExecContext(
		ctx,
		"INSERT INTO workspaces (id, slug, name) VALUES ($1, $2, $3)",
		workspaceID, "northwind-"+workspaceID.String()[:8], "Northwind",
	); err != nil {
		t.Fatalf("create the workspace: %v", err)
	}

	if _, err := client.ExecContext(
		ctx,
		`INSERT INTO accounts (id, status, email, display_name, timezone)
         VALUES ($1, 'active', $2, 'Rae', 'UTC')`,
		accountID, accountID.String()+"@northwind.example",
	); err != nil {
		t.Fatalf("create the account: %v", err)
	}

	if _, err := client.ExecContext(
		ctx,
		`INSERT INTO workspace_scm_connections
             (id, workspace_id, provider, integration_account_id, owner_account_id)
         VALUES ($1, $2, $3, $4, $4)`,
		connectionID, workspaceID, entity.SCMProviderGitHub, accountID,
	); err != nil {
		t.Fatalf("create the connection: %v", err)
	}

	sealer, err := crypter.New(config.Security{})
	if err != nil {
		t.Fatalf("build the crypter: %v", err)
	}

	repositories := scmrepo.NewSCMRepository(client, sealer)

	stored, err := repositories.Create(ctx, repository.SCMRepositoryInput{
		Repository: entity.SCMRepository{
			ConnectionID: connectionID,
			WorkspaceID:  workspaceID,
			Provider:     entity.SCMProviderGitHub,
			FullName:     "northwind/api",
			MirrorLabel:  "norn",
			PollInterval: 5 * time.Minute,
		},
	})
	if err != nil {
		t.Fatalf("connect the repository: %v", err)
	}

	return repositories, stored
}

func TestRepositorySettingsComeBackFromPostgresAsTheyWereWritten(t *testing.T) {
	client := scratchDatabase(t)
	repositories, stored := connectedRepository(t, client)

	ctx := context.Background()

	if stored.AnnounceDescription {
		t.Fatal("a newly connected repository announces the description, want the title alone")
	}

	settings := repository.SCMRepositorySettings{
		MirrorLabel:         "northwind",
		SyncDirection:       entity.MirrorInbound,
		WebhooksDisabled:    false,
		AnnounceDescription: true,
		PollInterval:        17 * time.Minute,
	}

	written, err := repositories.UpdateSettings(ctx, stored.ID, settings)
	if err != nil {
		t.Fatalf("update the repository settings: %v", err)
	}

	read, err := repositories.GetByID(ctx, stored.WorkspaceID, stored.ID)
	if err != nil {
		t.Fatalf("read the repository back: %v", err)
	}

	for _, got := range []entity.SCMRepository{written, read} {
		if got.MirrorLabel != settings.MirrorLabel {
			t.Errorf("mirror label came back %q, want %q", got.MirrorLabel, settings.MirrorLabel)
		}

		if got.SyncDirection != settings.SyncDirection {
			t.Errorf("direction came back %q, want %q", got.SyncDirection, settings.SyncDirection)
		}

		if got.WebhooksDisabled {
			t.Error("the repository came back polling, want it reachable by webhook — two booleans "+
				"read in the wrong order come back looking alike")
		}

		if !got.AnnounceDescription {
			t.Error("the repository came back announcing the title alone, want the description too")
		}

		if got.PollInterval != settings.PollInterval {
			t.Errorf("interval came back %s, want %s", got.PollInterval, settings.PollInterval)
		}

		if got.FullName != stored.FullName {
			t.Errorf("full name came back %q, want %q", got.FullName, stored.FullName)
		}
	}
}
