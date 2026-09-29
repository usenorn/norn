package executionreview

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
		"insertCommentQuery":         insertCommentQuery,
		"commentByIDQuery":           commentByIDQuery,
		"commentsByExecutionQuery":   commentsByExecutionQuery,
		"countCommentsQuery":         countCommentsQuery,
		"editCommentQuery":           editCommentQuery,
		"deleteCommentQuery":         deleteCommentQuery,
		"resolveCommentQuery":        resolveCommentQuery,
		"insertReviewQuery":          insertReviewQuery,
		"attachPendingCommentsQuery": attachPendingCommentsQuery,
		"reviewsByExecutionQuery":    reviewsByExecutionQuery,
	}
}

func TestSubmittingAReviewPublishesOnlyItsAuthorsDrafts(t *testing.T) {
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
		t.Fatalf("build a delegation to review: %v", err)
	}

	if _, err := tx.Exec(`
INSERT INTO workspace_executions (id, workspace_id, issue_id, delegation_id, agent_id, attempt)
VALUES ('exec-reviewed', $1, $2, $3, $4, 1)`,
		workspaceID, issueID, delegationID, agentID,
	); err != nil {
		t.Fatalf("open a run: %v", err)
	}

	var reviewer, colleague string

	for _, name := range []string{"reviewer", "colleague"} {
		var id string
		if err := tx.QueryRow(`
INSERT INTO accounts (status, kind, email, display_name, timezone)
VALUES ('active', 'person', $1 || '@review-check.test', $1, 'UTC') RETURNING id`, name).Scan(&id); err != nil {
			t.Fatalf("add %s: %v", name, err)
		}

		if name == "reviewer" {
			reviewer = id
		} else {
			colleague = id
		}
	}

	draft := func(author string) string {
		t.Helper()

		comment, err := scanComment(tx.QueryRow(insertCommentQuery,
			"exec-reviewed", workspaceID, "", "", "api", "main.go", "new", 3, "abc", "", "why", author,
			time.Now().UTC(),
		))
		if err != nil {
			t.Fatalf("draft a comment: %v", err)
		}

		return comment.ID.String()
	}

	mine, theirs := draft(reviewer), draft(colleague)

	review, err := scanReview(tx.QueryRow(insertReviewQuery,
		"exec-reviewed", workspaceID, "comment", "", []byte(`{"api":"abc"}`), reviewer, time.Now().UTC(),
	))
	if err != nil {
		t.Fatalf("submit the review: %v", err)
	}

	if _, err := tx.Exec(attachPendingCommentsQuery, "exec-reviewed", reviewer, review.ID.String()); err != nil {
		t.Fatalf("attach the drafts: %v", err)
	}

	attached := func(id string) bool {
		t.Helper()

		comment, err := scanComment(tx.QueryRow(commentByIDQuery, "exec-reviewed", id))
		if err != nil {
			t.Fatalf("read comment %s: %v", id, err)
		}

		return !comment.Pending()
	}

	if !attached(mine) {
		t.Fatal("the reviewer's own draft stayed a draft after they submitted")
	}

	if attached(theirs) {
		t.Fatal("somebody else's unfinished draft was published by another person's review")
	}

	if review.Heads["api"] != "abc" {
		t.Fatalf("the review remembered heads %v", review.Heads)
	}
}

const delegationFixture = `
WITH workspace AS (
    INSERT INTO workspaces (slug, name) VALUES ('review-check', 'Review check')
    RETURNING id
), team AS (
    INSERT INTO workspace_teams (workspace_id, key, name)
    SELECT id, 'REV', 'Reviewing' FROM workspace
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
    SELECT team.workspace_id, team.id, 1, 'run me', state.id, 'REV', 'n'
    FROM team, state
    RETURNING id, workspace_id, team_id
), delegation AS (
    INSERT INTO workspace_issue_delegations (workspace_id, issue_id, agent_id)
    SELECT issue.workspace_id, issue.id, agent.id FROM issue, agent
    RETURNING id
)
SELECT issue.workspace_id, issue.id, agent.id, delegation.id
FROM issue, agent, delegation`
