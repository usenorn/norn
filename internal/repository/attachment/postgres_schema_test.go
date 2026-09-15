package attachment

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	migrations "github.com/usenorn/norn/db"
)

const (
	lastMigrationBeforeRecount = 20260913091500
	recountMigration           = 20260915180000
)

func postgresDSN(t *testing.T) string {
	t.Helper()

	dsn := strings.TrimSpace(os.Getenv("NORN_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("NORN_POSTGRES_DSN is unset, so there is no database to run this against")
	}

	return dsn
}

func openDatabase(t *testing.T, dsn string) *sql.DB {
	t.Helper()

	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open the database: %v", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	if err := database.Ping(); err != nil {
		t.Skipf("no database to run this against: %v", err)
	}

	return database
}

func scratchDatabase(t *testing.T) *sql.DB {
	t.Helper()

	dsn := postgresDSN(t)
	server := openDatabase(t, dsn)
	name := fmt.Sprintf("norn_recount_%d", time.Now().UnixNano())

	if _, err := server.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatalf("create a scratch database: %v", err)
	}

	t.Cleanup(func() { _, _ = server.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)") })

	scratch, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse the database address: %v", err)
	}

	scratch.Path = "/" + name

	return openDatabase(t, scratch.String())
}

func migrateTo(t *testing.T, database *sql.DB, version int64) {
	t.Helper()

	fsys, err := migrations.PostgresMigrations()
	if err != nil {
		t.Fatalf("read the migrations: %v", err)
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, database, fsys, goose.WithAllowOutofOrder(true))
	if err != nil {
		t.Fatalf("build the migration provider: %v", err)
	}

	if _, err := provider.UpTo(context.Background(), version); err != nil {
		t.Fatalf("migrate to %d: %v", version, err)
	}
}

func TestEveryAttachmentStatementMatchesTheSchema(t *testing.T) {
	database := openDatabase(t, postgresDSN(t))

	for name, statement := range statements {
		t.Run(name, func(t *testing.T) {
			prepared, err := database.Prepare(statement)
			if err != nil {
				t.Fatalf(
					"%s does not match the schema: %v. The package reaches the database through raw "+
						"SQL, so a column that does not exist leaves the build and every other test green.",
					name, err,
				)
			}

			_ = prepared.Close()
		})
	}
}

const legacyIssueFixture = `
WITH workspace AS (
    INSERT INTO workspaces (slug, name) VALUES ('recount', 'Recount')
    RETURNING id
), team AS (
    INSERT INTO workspace_teams (workspace_id, key, name)
    SELECT id, 'REC', 'Recount' FROM workspace
    RETURNING id, workspace_id
), state AS (
    INSERT INTO workspace_workflow_states (workspace_id, team_id, name, category, position)
    SELECT workspace_id, id, 'Todo', 'not_started', 1 FROM team
    RETURNING id
), issue AS (
    INSERT INTO workspace_issues (workspace_id, team_id, number, title, state_id, reference_key, rank)
    SELECT team.workspace_id, team.id, 1, 'files', state.id, 'REC', 'n'
    FROM team, state
    RETURNING id, workspace_id
)
SELECT workspace_id, id FROM issue`

const legacyAttachmentFixture = `
INSERT INTO workspace_issue_attachments
    (id, workspace_id, issue_id, object_key, file_name, content_type, size_bytes, status,
     reclaim_after, created_at, updated_at)
VALUES
    (gen_random_uuid(), $1, nullif($2, '')::uuid, $3, 'file.bin', 'application/octet-stream', $4, $5,
     CASE WHEN $6 THEN now() ELSE NULL END, now(), now())
RETURNING id`

type legacyFile struct {
	name     string
	onIssue  bool
	bytes    int64
	status   string
	deadline bool
}

func TestTheRecountLeavesOutBytesTheOldCodeAlreadyGaveBackAndKeepsOrphansThatStillTakeRoom(t *testing.T) {
	database := scratchDatabase(t)

	migrateTo(t, database, lastMigrationBeforeRecount)

	var workspaceID, issueID string

	if err := database.QueryRow(legacyIssueFixture).Scan(&workspaceID, &issueID); err != nil {
		t.Fatalf("build a workspace with an issue: %v", err)
	}

	files := []legacyFile{
		{name: "stored", onIssue: true, bytes: 100, status: "stored"},
		{name: "reserved", onIssue: true, bytes: 50, status: "pending", deadline: true},
		{name: "removed", onIssue: true, bytes: 200, status: "discarded", deadline: true},
		{name: "orphan marked by the sweep", bytes: 70, status: "discarded", deadline: true},
		{name: "orphan the sweep has not reached", bytes: 30, status: "stored"},
	}

	ids := map[string]string{}

	for _, file := range files {
		issue := ""
		if file.onIssue {
			issue = issueID
		}

		key := "attachments/" + workspaceID + "/" + strings.ReplaceAll(file.name, " ", "-")

		var id string

		if err := database.QueryRow(
			legacyAttachmentFixture, workspaceID, issue, key, file.bytes, file.status, file.deadline,
		).Scan(&id); err != nil {
			t.Fatalf("store the %s file: %v", file.name, err)
		}

		ids[file.name] = id
	}

	if _, err := database.Exec(
		`INSERT INTO workspace_storage_ledger (workspace_id, stored_bytes) VALUES ($1, 250)`, workspaceID,
	); err != nil {
		t.Fatalf("record what the old code counted: %v", err)
	}

	var driftedID string

	if err := database.QueryRow(
		`INSERT INTO workspaces (slug, name) VALUES ('drifted', 'Drifted') RETURNING id`,
	).Scan(&driftedID); err != nil {
		t.Fatalf("build a second workspace: %v", err)
	}

	if _, err := database.Exec(
		`INSERT INTO workspace_storage_ledger (workspace_id, stored_bytes) VALUES ($1, 999)`, driftedID,
	); err != nil {
		t.Fatalf("drift the second workspace's ledger: %v", err)
	}

	migrateTo(t, database, recountMigration)

	ledger := func(workspace string) int64 {
		t.Helper()

		var stored int64

		if err := database.QueryRow(
			`SELECT stored_bytes FROM workspace_storage_ledger WHERE workspace_id = $1`, workspace,
		).Scan(&stored); err != nil {
			t.Fatalf("read the ledger: %v", err)
		}

		return stored
	}

	sizeOf := func(name string) int64 {
		t.Helper()

		var size int64

		if err := database.QueryRow(
			`SELECT size_bytes FROM workspace_issue_attachments WHERE id = $1`, ids[name],
		).Scan(&size); err != nil {
			t.Fatalf("read the %s file: %v", name, err)
		}

		return size
	}

	if stored := ledger(workspaceID); stored != 250 {
		t.Fatalf(
			"the recount put the workspace at %d bytes, want 250. The removed file's 200 bytes were "+
				"already taken off when it was discarded, and the orphans' 100 bytes are still in "+
				"storage waiting for the sweep.",
			stored,
		)
	}

	if size := sizeOf("removed"); size != 0 {
		t.Fatalf(
			"the removed file still carries %d bytes. The sweep subtracts what the row carries, so "+
				"it would take those bytes off a second time.",
			size,
		)
	}

	if size := sizeOf("orphan marked by the sweep"); size != 70 {
		t.Fatalf(
			"the orphan the sweep marked carries %d bytes, want 70. Its bytes were never given back, "+
				"so the sweep has to subtract them when it deletes the object.",
			size,
		)
	}

	if stored := ledger(driftedID); stored != 0 {
		t.Fatalf("a workspace with no files but a ledger of 999 was recounted to %d", stored)
	}

	for _, name := range []string{"removed", "orphan marked by the sweep"} {
		if _, err := database.Exec(reclaimAttachmentQuery, ids[name]); err != nil {
			t.Fatalf("sweep the %s file: %v", name, err)
		}
	}

	var rows int64

	if err := database.QueryRow(
		`SELECT coalesce(sum(size_bytes), 0) FROM workspace_issue_attachments WHERE workspace_id = $1`,
		workspaceID,
	).Scan(&rows); err != nil {
		t.Fatalf("add up the remaining files: %v", err)
	}

	if stored := ledger(workspaceID); stored != 180 || rows != 180 {
		t.Fatalf(
			"after the sweep the ledger reads %d and the remaining files add up to %d, want 180 for "+
				"both: the stored file, the reservation and the orphan the sweep has not reached.",
			stored, rows,
		)
	}
}
