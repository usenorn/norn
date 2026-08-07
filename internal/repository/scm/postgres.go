package scm

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	uniqueViolationCode = "23505"

	repositoryUniqueIndex     = "workspace_scm_connections_repository_key"
	deliveryUniqueIndex       = "workspace_scm_deliveries_external_key"
	mirrorIssueUniqueIndex    = "workspace_issue_mirrors_issue_key"
	mirrorExternalUniqueIndex = "workspace_issue_mirrors_external_key"
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
