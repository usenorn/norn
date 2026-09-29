package executionplan

import (
	"database/sql"
	"errors"
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

func TestEveryStatementMatchesTheSchemaItRunsAgainst(t *testing.T) {
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
		"proposePlanQuery":         proposePlanQuery,
		"plansByExecutionQuery":    plansByExecutionQuery,
		"approvePlanQuery":         approvePlanQuery,
		"requestPlanRevisionQuery": requestPlanRevisionQuery,
	}
}

func TestOnlyTheNewestUndecidedRevisionOfAPlanCanBeDecided(t *testing.T) {
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

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	t.Cleanup(func() { _ = tx.Rollback() })

	var delegationID, workspaceID, issueID, agentID string

	if err := tx.QueryRow(delegationFixture).Scan(
		&workspaceID, &issueID, &agentID, &delegationID,
	); err != nil {
		t.Fatalf("build a delegation to plan against: %v", err)
	}

	if _, err := tx.Exec(`
INSERT INTO workspace_executions (id, workspace_id, issue_id, delegation_id, agent_id, attempt)
VALUES ('exec-planned', $1, $2, $3, $4, 1)`,
		workspaceID, issueID, delegationID, agentID,
	); err != nil {
		t.Fatalf("open a run: %v", err)
	}

	propose := func(ref string) (int, bool) {
		t.Helper()

		row := tx.QueryRow(proposePlanQuery, "exec-planned", workspaceID, ref, "do it "+ref, time.Now().UTC())

		plan, err := scanPlan(row)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false
		}

		if err != nil {
			t.Fatalf("propose %s: %v", ref, err)
		}

		return plan.Revision, true
	}

	if revision, _ := propose("first"); revision != 1 {
		t.Fatalf("the first plan is revision %d, want 1", revision)
	}

	if _, recorded := propose("first"); recorded {
		t.Fatal("a replayed proposal became a second revision; the reviewer would see the same plan twice")
	}

	if revision, _ := propose("second"); revision != 2 {
		t.Fatalf("the second plan is revision %d, want 2", revision)
	}

	decide := func(revision int) bool {
		t.Helper()

		_, err := scanPlan(tx.QueryRow(approvePlanQuery, "exec-planned", revision, "", time.Now().UTC()))
		if errors.Is(err, sql.ErrNoRows) {
			return false
		}

		if err != nil {
			t.Fatalf("approve revision %d: %v", revision, err)
		}

		return true
	}

	if decide(1) {
		t.Fatal(
			"revision 1 was approved after revision 2 replaced it; a person approving the plan " +
				"they last looked at would authorise work nobody read",
		)
	}

	if !decide(2) {
		t.Fatal("the newest revision could not be approved")
	}

	if decide(2) {
		t.Fatal("a plan was approved twice")
	}
}

const delegationFixture = `
WITH workspace AS (
    INSERT INTO workspaces (slug, name) VALUES ('plan-check', 'Plan check')
    RETURNING id
), team AS (
    INSERT INTO workspace_teams (workspace_id, key, name)
    SELECT id, 'PLN', 'Planning' FROM workspace
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
    SELECT team.workspace_id, team.id, 1, 'run me', state.id, 'PLN', 'n'
    FROM team, state
    RETURNING id, workspace_id, team_id
), delegation AS (
    INSERT INTO workspace_issue_delegations (workspace_id, issue_id, agent_id)
    SELECT issue.workspace_id, issue.id, agent.id FROM issue, agent
    RETURNING id
)
SELECT issue.workspace_id, issue.id, agent.id, delegation.id
FROM issue, agent, delegation`
