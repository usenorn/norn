package scm

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Every statement this package sends. Source control reaches the database through raw SQL
// rather than the generated models, so nothing here is checked by the compiler: a column
// that stops existing leaves `go build` and `go test` green and breaks only in production.
// This test is the missing check — Postgres itself refuses a statement naming a table,
// column or type it does not have.
func statements() map[string]string {
	return map[string]string{
		"claimDueQuery":                     claimDueQuery,
		"claimTransitionQuery":              claimTransitionQuery,
		"deleteConnectionQuery":             deleteConnectionQuery,
		"deleteLinkQuery":                   deleteLinkQuery,
		"deleteMirrorQuery":                 deleteMirrorQuery,
		"deleteRepositoryQuery":             deleteRepositoryQuery,
		"deleteRouteQuery":                  deleteRouteQuery,
		"deleteRuleQuery":                   deleteRuleQuery,
		"deleteSettledDeliveriesQuery":      deleteSettledDeliveriesQuery,
		"detachLinksQuery":                  detachLinksQuery,
		"detachMirrorsQuery":                detachMirrorsQuery,
		"getCommentMirrorQuery":             getCommentMirrorQuery,
		"getConnectionForDeliveryQuery":     getConnectionForDeliveryQuery,
		"getConnectionQuery":                getConnectionQuery,
		"getDeliveryQuery":                  getDeliveryQuery,
		"getMirrorByExternalQuery":          getMirrorByExternalQuery,
		"getMirrorByIssueQuery":             getMirrorByIssueQuery,
		"getRepositoryForDeliveryQuery":     getRepositoryForDeliveryQuery,
		"getRepositoryQuery":                getRepositoryQuery,
		"insertCommentMirrorQuery":          insertCommentMirrorQuery,
		"insertConnectionQuery":             insertConnectionQuery,
		"insertMirrorQuery":                 insertMirrorQuery,
		"insertRepositoryQuery":             insertRepositoryQuery,
		"insertRouteQuery":                  insertRouteQuery,
		"listCommentMirrorsQuery":           listCommentMirrorsQuery,
		"listConnectionsQuery":              listConnectionsQuery,
		"listDeliveriesQuery":               listDeliveriesQuery,
		"listLinksByExternalQuery":          listLinksByExternalQuery,
		"listLinksByIssueQuery":             listLinksByIssueQuery,
		"listMirrorsByIssueQuery":           listMirrorsByIssueQuery,
		"listMirrorsByRepositoryQuery":      listMirrorsByRepositoryQuery,
		"listPendingDeliveriesQuery":        listPendingDeliveriesQuery,
		"listRepositoriesByConnectionQuery": listRepositoriesByConnectionQuery,
		"listRepositoriesByWorkspaceQuery":  listRepositoriesByWorkspaceQuery,
		"listRoutesByRepositoryQuery":       listRoutesByRepositoryQuery,
		"listRoutesByWorkspaceQuery":        listRoutesByWorkspaceQuery,
		"listRulesQuery":                    listRulesQuery,
		"markBrokenQuery":                   markBrokenQuery,
		"markVerifiedQuery":                 markVerifiedQuery,
		"parkQuery":                         parkQuery,
		"readLinkQuery":                     readLinkQuery,
		"recordDeliveryQuery":               recordDeliveryQuery,
		"recordHookQuery":                   recordHookQuery,
		"recordPullQuery":                   recordPullQuery,
		"recordPushQuery":                   recordPushQuery,
		"recordReconciledQuery":             recordReconciledQuery,
		"recordSeenQuery":                   recordSeenQuery,
		"replaceTokenQuery":                 replaceTokenQuery,
		"rescheduleDeliveryQuery":           rescheduleDeliveryQuery,
		"settleDeliveryQuery":               settleDeliveryQuery,
		"tokenQuery":                        tokenQuery,
		"updateLabelQuery":                  updateLabelQuery,
		"updateRepositorySettingsQuery":     updateRepositorySettingsQuery,
		"upsertLinkQuery":                   upsertLinkQuery,
		"upsertRuleQuery":                   upsertRuleQuery,
		"webhookSecretQuery":                webhookSecretQuery,
	}
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

// redacted keeps the password out of a failure message, which lands in CI output.
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

// TestEveryStatementInThePackageIsChecked keeps the map above honest. A statement added to
// the package and forgotten here would be exactly as unchecked as before this test existed.
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
