package execution

import (
	"database/sql"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func declaredStatements() ([]string, error) {
	entries, err := os.ReadDir(".")
	if err != nil {
		return nil, err
	}

	set := token.NewFileSet()
	names := make([]string, 0, 16)

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(set, name, nil, 0)
		if err != nil {
			return nil, err
		}

		names = append(names, queryConstants(file)...)
	}

	return names, nil
}

func queryConstants(file *ast.File) []string {
	found := make([]string, 0, 8)

	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.CONST {
			continue
		}

		for _, spec := range general.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}

			for _, declared := range value.Names {
				if strings.HasSuffix(declared.Name, "Query") {
					found = append(found, declared.Name)
				}
			}
		}
	}

	return found
}

func redacted(dsn string) string {
	scheme, rest, found := strings.Cut(dsn, "://")
	if !found {
		return "the configured database"
	}

	_, host, found := strings.Cut(rest, "@")
	if !found {
		return scheme + "://" + rest
	}

	return scheme + "://" + host
}

func schemaDatabase(t *testing.T) *sql.DB {
	t.Helper()

	dsn := strings.TrimSpace(os.Getenv("NORN_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("NORN_POSTGRES_DSN is unset, so there is no schema to check the statements against")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open %s: %v", redacted(dsn), err)
	}

	t.Cleanup(func() { _ = db.Close() })

	if err := db.Ping(); err != nil {
		t.Skipf("no database at %s: %v", redacted(dsn), err)
	}

	return db
}

func TestEveryStatementMatchesTheSchemaItRunsAgainst(t *testing.T) {
	db := schemaDatabase(t)

	for name, statement := range statements() {
		t.Run(name, func(t *testing.T) {
			prepared, err := db.Prepare(statement)
			if err != nil {
				t.Fatalf(
					"%s does not match the schema: %v\n\nNothing else catches this. The package "+
						"reaches the database through raw SQL, so a column that stops existing "+
						"leaves the build and every other test green.",
					name, err,
				)
			}

			_ = prepared.Close()
		})
	}
}

func TestEveryStatementInThePackageIsChecked(t *testing.T) {
	declared, err := declaredStatements()
	if err != nil {
		t.Fatalf("read the package source: %v", err)
	}

	checked := statements()

	for _, name := range declared {
		if _, ok := checked[name]; !ok {
			t.Errorf(
				"%s is declared in the package but not listed in statements(), so nothing "+
					"verifies it against the schema",
				name,
			)
		}
	}

	for name := range checked {
		if !contains(declared, name) {
			t.Errorf("statements() lists %s, which no longer exists in the package", name)
		}
	}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}

	return false
}

func statements() map[string]string {
	return map[string]string{
		"insertExecutionQuery":               insertExecutionQuery,
		"executionByIDQuery":                 executionByIDQuery,
		"executionsByIssueQuery":             executionsByIssueQuery,
		"visibleExecutionsQuery":             visibleExecutionsQuery,
		"keepExecutionQuery":                 keepExecutionQuery,
		"liveExecutionsByRunnerQuery":        liveExecutionsByRunnerQuery,
		"queuedExecutionsByAgentQuery":       queuedExecutionsByAgentQuery,
		"executionsSharingRepositoriesQuery": executionsSharingRepositoriesQuery,
		"runnerHeldSlotsQuery":               runnerHeldSlotsQuery,
		"nextExecutionAttemptQuery":          nextExecutionAttemptQuery,
		"bindExecutionQuery":                 bindExecutionQuery,
		"moveExecutionQuery":                 moveExecutionQuery,
		"expiredExecutionLeasesQuery":        expiredExecutionLeasesQuery,
	}
}

func TestOneDelegationCannotHaveTwoRunsInFlightAtOnce(t *testing.T) {
	db := schemaDatabase(t)

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	t.Cleanup(func() { _ = tx.Rollback() })

	var delegationID, workspaceID, issueID, agentID string

	if err := tx.QueryRow(delegationFixture).Scan(
		&workspaceID, &issueID, &agentID, &delegationID,
	); err != nil {
		t.Fatalf("build a delegation to run against: %v", err)
	}

	insert := `
INSERT INTO workspace_executions (id, workspace_id, issue_id, delegation_id, agent_id, attempt)
VALUES ($1, $2, $3, $4, $5, $6)`

	if _, err := tx.Exec(insert, "exec-first", workspaceID, issueID, delegationID, agentID, 1); err != nil {
		t.Fatalf("open the first run: %v", err)
	}

	if _, err := tx.Exec(`SAVEPOINT before_second`); err != nil {
		t.Fatalf("savepoint: %v", err)
	}

	if _, err := tx.Exec(insert, "exec-second", workspaceID, issueID, delegationID, agentID, 2); err == nil {
		t.Fatal(
			"a second run opened against a delegation whose first run is still going. Two " +
				"machines would then work the same issue from the same brief, and whichever " +
				"finished last would overwrite the other's branch",
		)
	}

	if _, err := tx.Exec(`ROLLBACK TO SAVEPOINT before_second`); err != nil {
		t.Fatalf("roll back to savepoint: %v", err)
	}

	if _, err := tx.Exec(
		`UPDATE workspace_executions SET state = 'failed', finished_at = now() WHERE id = $1`,
		"exec-first",
	); err != nil {
		t.Fatalf("finish the first run: %v", err)
	}

	if _, err := tx.Exec(insert, "exec-second", workspaceID, issueID, delegationID, agentID, 2); err != nil {
		t.Fatalf(
			"restarting a finished run was refused: %v. A re-run is a new attempt against the "+
				"same delegation, so the rule has to be about runs in flight and not about the "+
				"delegation ever having been run",
			err,
		)
	}
}

const delegationFixture = `
WITH workspace AS (
    INSERT INTO workspaces (slug, name) VALUES ('scheduling-check', 'Scheduling check')
    RETURNING id
), team AS (
    INSERT INTO workspace_teams (workspace_id, key, name)
    SELECT id, 'SCH', 'Scheduling' FROM workspace
    RETURNING id, workspace_id
), state AS (
    INSERT INTO workspace_workflow_states (workspace_id, team_id, name, category, position)
    SELECT workspace_id, id, 'Todo', 'not_started', 1 FROM team
    RETURNING id
), account AS (
    INSERT INTO accounts (status, kind, display_name, timezone)
    VALUES ('active', 'agent', 'scheduler', 'UTC')
    RETURNING id
), agent AS (
    INSERT INTO workspace_agents (workspace_id, account_id, owner_account_id, name)
    SELECT team.workspace_id, account.id, account.id, 'scheduler'
    FROM team, account
    RETURNING id, workspace_id
), issue AS (
    INSERT INTO workspace_issues
        (workspace_id, team_id, number, title, state_id, reference_key, rank)
    SELECT team.workspace_id, team.id, 1, 'run me', state.id, 'SCH', 'n'
    FROM team, state
    RETURNING id, workspace_id, team_id
), delegation AS (
    INSERT INTO workspace_issue_delegations (workspace_id, issue_id, agent_id)
    SELECT issue.workspace_id, issue.id, agent.id FROM issue, agent
    RETURNING id
)
SELECT issue.workspace_id, issue.id, agent.id, delegation.id
FROM issue, agent, delegation`

func TestTheRunListNeverReachesOutsideTheCallersTeams(t *testing.T) {
	db := schemaDatabase(t)

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	t.Cleanup(func() { _ = tx.Rollback() })

	var delegationID, workspaceID, issueID, agentID string

	if err := tx.QueryRow(delegationFixture).Scan(
		&workspaceID, &issueID, &agentID, &delegationID,
	); err != nil {
		t.Fatalf("build a delegation to run against: %v", err)
	}

	var teamID string

	if err := tx.QueryRow(
		`SELECT team_id::text FROM workspace_issues WHERE id = $1`, issueID,
	).Scan(&teamID); err != nil {
		t.Fatalf("read the team the issue belongs to: %v", err)
	}

	if _, err := tx.Exec(`
INSERT INTO workspace_executions
    (id, workspace_id, issue_id, delegation_id, agent_id, attempt, state)
VALUES ($1, $2, $3, $4, $5, 1, 'awaiting_review')`,
		"exec-scoped", workspaceID, issueID, delegationID, agentID,
	); err != nil {
		t.Fatalf("open a run to look for: %v", err)
	}

	count := func(allTeams bool, teams []string) int {
		t.Helper()

		rows, err := tx.Query(
			visibleExecutionsQuery, workspaceID, allTeams, teams,
			[]string{"awaiting_review"}, 50,
		)
		if err != nil {
			t.Fatalf("list the runs: %v", err)
		}

		defer func() { _ = rows.Close() }()

		found := 0

		for rows.Next() {
			found++
		}

		if err := rows.Err(); err != nil {
			t.Fatalf("read the runs: %v", err)
		}

		return found
	}

	if found := count(false, []string{}); found != 0 {
		t.Fatalf(
			"a caller on no team saw %d runs. The review queue is workspace-wide, so a "+
				"statement that ignores the scope shows somebody every private team's work",
			found,
		)
	}

	if found := count(false, []string{teamID}); found != 1 {
		t.Fatalf("a caller on the run's own team saw %d runs rather than the one that is there", found)
	}

	if found := count(true, []string{}); found != 1 {
		t.Fatalf("a caller who may see every team saw %d runs rather than the one that is there", found)
	}
}

func TestAWorkspaceIsHeldForTheLatestDeadlineAMachineHasNamed(t *testing.T) {
	db := schemaDatabase(t)

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	t.Cleanup(func() { _ = tx.Rollback() })

	var delegationID, workspaceID, issueID, agentID string

	if err := tx.QueryRow(delegationFixture).Scan(
		&workspaceID, &issueID, &agentID, &delegationID,
	); err != nil {
		t.Fatalf("build a delegation to run against: %v", err)
	}

	if _, err := tx.Exec(`
INSERT INTO workspace_executions
    (id, workspace_id, issue_id, delegation_id, agent_id, attempt, state, finished_at)
VALUES ($1, $2, $3, $4, $5, 1, 'completed', now())`,
		"exec-kept", workspaceID, issueID, delegationID, agentID,
	); err != nil {
		t.Fatalf("open a run to hold: %v", err)
	}

	later := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Microsecond)
	sooner := later.Add(-time.Hour)
	now := time.Now().UTC()

	if _, err := tx.Exec(keepExecutionQuery, "exec-kept", later, now); err != nil {
		t.Fatalf("hold the workspace: %v", err)
	}

	if _, err := tx.Exec(keepExecutionQuery, "exec-kept", sooner, now); err != nil {
		t.Fatalf("hold the workspace again: %v", err)
	}

	var held time.Time

	if err := tx.QueryRow(
		`SELECT keep_until FROM workspace_executions WHERE id = $1`, "exec-kept",
	).Scan(&held); err != nil {
		t.Fatalf("read the deadline: %v", err)
	}

	if !held.Equal(later) {
		t.Fatalf(
			"a report that arrived late pulled the deadline back to %s from %s. Delivery is "+
				"at-least-once and out of order, so an older report has to be a no-op rather "+
				"than something that takes back an extension somebody asked for",
			held, later,
		)
	}
}
