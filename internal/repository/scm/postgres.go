package scm

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	uniqueViolationCode = "23505"

	connectionEndpointUniqueIndex = "workspace_scm_connections_endpoint_key"
	repositoryNameUniqueIndex     = "workspace_scm_repositories_name_key"
	routePrefixUniqueIndex        = "workspace_scm_routes_prefix_key"
	deliveryUniqueIndex           = "workspace_scm_deliveries_external_key"
	mirrorPairUniqueIndex         = "workspace_issue_mirrors_pair_key"
	mirrorExternalUniqueIndex     = "workspace_issue_mirrors_external_key"
	identityLoginUniqueIndex      = "workspace_scm_identities_login_key"
	identityAccountUniqueIndex    = "workspace_scm_identities_account_key"
)

func violates(err error, index string) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) &&
		pgErr.Code == uniqueViolationCode &&
		pgErr.ConstraintName == index
}

func expectOne(result sql.Result, missing error) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected rows: %w", err)
	}

	if affected == 0 {
		return missing
	}

	return nil
}
