package scm

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func statements() map[string]string {
	return map[string]string{
		"claimAnnouncementQuery":            claimAnnouncementQuery,
		"claimDueQuery":                     claimDueQuery,
		"claimTransitionQuery":              claimTransitionQuery,
		"deferTransitionQuery":              deferTransitionQuery,
		"deferredTransitionsQuery":          deferredTransitionsQuery,
		"settleTransitionQuery":             settleTransitionQuery,
		"deleteConnectionQuery":             deleteConnectionQuery,
		"deleteIdentityQuery":               deleteIdentityQuery,
		"deleteLinkQuery":                   deleteLinkQuery,
		"deleteMirrorQuery":                 deleteMirrorQuery,
		"deleteRepositoryQuery":             deleteRepositoryQuery,
		"deleteReviewersQuery":              deleteReviewersQuery,
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
		"getTeamSettingsQuery":              getTeamSettingsQuery,
		"insertCommentMirrorQuery":          insertCommentMirrorQuery,
		"insertConnectionQuery":             insertConnectionQuery,
		"insertIdentityQuery":               insertIdentityQuery,
		"insertMirrorQuery":                 insertMirrorQuery,
		"insertRepositoryQuery":             insertRepositoryQuery,
		"insertReviewerQuery":               insertReviewerQuery,
		"insertRouteQuery":                  insertRouteQuery,
		"insertRuleQuery":                   insertRuleQuery,
		"linkReleaseChangeQuery":            linkReleaseChangeQuery,
		"listCommentMirrorsQuery":           listCommentMirrorsQuery,
		"listConflictsQuery":                listConflictsQuery,
		"listConnectionsQuery":              listConnectionsQuery,
		"listDeliveriesQuery":               listDeliveriesQuery,
		"listDeploymentsQuery":              listDeploymentsQuery,
		"listIdentitiesQuery":               listIdentitiesQuery,
		"listLinksByExternalQuery":          listLinksByExternalQuery,
		"listLinksByIssueQuery":             listLinksByIssueQuery,
		"listLinksByRepositoryQuery":        listLinksByRepositoryQuery,
		"listMirrorsByIssueQuery":           listMirrorsByIssueQuery,
		"listMirrorsByRepositoryQuery":      listMirrorsByRepositoryQuery,
		"listPendingDeliveriesQuery":        listPendingDeliveriesQuery,
		"listReleasesByIssueQuery":          listReleasesByIssueQuery,
		"listReleasesQuery":                 listReleasesQuery,
		"listRepositoriesByConnectionQuery": listRepositoriesByConnectionQuery,
		"listRepositoriesByWorkspaceQuery":  listRepositoriesByWorkspaceQuery,
		"listReviewersQuery":                listReviewersQuery,
		"listRoutesByRepositoryQuery":       listRoutesByRepositoryQuery,
		"listRoutesByWorkspaceQuery":        listRoutesByWorkspaceQuery,
		"listRulesQuery":                    listRulesQuery,
		"markBrokenQuery":                   markBrokenQuery,
		"markVerifiedQuery":                 markVerifiedQuery,
		"parkQuery":                         parkQuery,
		"readLinkQuery":                     readLinkQuery,
		"recordBackfilledQuery":             recordBackfilledQuery,
		"recordConflictQuery":               recordConflictQuery,
		"recordDeliveryQuery":               recordDeliveryQuery,
		"recordHookQuery":                   recordHookQuery,
		"recordPullQuery":                   recordPullQuery,
		"recordPushQuery":                   recordPushQuery,
		"recordReconciledQuery":             recordReconciledQuery,
		"recordSeenQuery":                   recordSeenQuery,
		"releaseAnnouncementQuery":          releaseAnnouncementQuery,
		"replaceTokenQuery":                 replaceTokenQuery,
		"rescheduleDeliveryQuery":           rescheduleDeliveryQuery,
		"setChecksQuery":                    setChecksQuery,
		"settleDeliveryQuery":               settleDeliveryQuery,
		"tokenQuery":                        tokenQuery,
		"upsertAppQuery":                    upsertAppQuery,
		"getAppQuery":                       getAppQuery,
		"getAppByIDQuery":                   getAppByIDQuery,
		"appSecretsQuery":                   appSecretsQuery,
		"getConnectionByInstallationQuery":  getConnectionByInstallationQuery,
		"getRepositoryByFullNameQuery":      getRepositoryByFullNameQuery,
		"listAppsQuery":                     listAppsQuery,
		"updateLabelQuery":                  updateLabelQuery,
		"updateRepositorySettingsQuery":     updateRepositorySettingsQuery,
		"upsertDeploymentQuery":             upsertDeploymentQuery,
		"upsertLinkQuery":                   upsertLinkQuery,
		"upsertReleaseQuery":                upsertReleaseQuery,
		"upsertRuleQuery":                   upsertRuleQuery,
		"upsertTeamSettingsQuery":           upsertTeamSettingsQuery,
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
