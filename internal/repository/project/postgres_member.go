package project

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const (
	foreignKeyViolationCode = "23503"
	memberUniqueIndex       = "workspace_project_members_project_account_key"
	membershipForeignKey    = "workspace_project_members_membership_fkey"
)

const memberColumns = `
       id,
       workspace_id,
       project_id,
       account_id,
       created_at,
       updated_at`

const insertMemberQuery = `
INSERT INTO workspace_project_members (workspace_id, project_id, account_id)
VALUES ($1, $2, $3)
RETURNING` + memberColumns

const memberQuery = `
SELECT` + memberColumns + `
FROM workspace_project_members
WHERE project_id = $1 AND account_id = $2`

const membersQuery = `
SELECT` + memberColumns + `
FROM workspace_project_members
WHERE project_id = $1
ORDER BY created_at, id`

const deleteMemberQuery = `
DELETE FROM workspace_project_members WHERE project_id = $1 AND account_id = $2`

type memberRepository struct {
	db *postgres.Client
}

func NewMember(db *postgres.Client) repository.ProjectMember {
	return &memberRepository{db: db}
}

func scanMembership(row scanner) (entity.ProjectMembership, error) {
	var (
		membership entity.ProjectMembership
		id         string
		workspace  string
		project    string
		account    string
	)

	if err := row.Scan(
		&id,
		&workspace,
		&project,
		&account,
		&membership.CreatedAt,
		&membership.UpdatedAt,
	); err != nil {
		return entity.ProjectMembership{}, err
	}

	parsed, err := uuid.Parse(id)
	if err != nil {
		return entity.ProjectMembership{}, fmt.Errorf("parse project membership id: %w", err)
	}

	membership.ID = parsed

	if membership.WorkspaceID, err = uuid.Parse(workspace); err != nil {
		return entity.ProjectMembership{}, fmt.Errorf("parse membership workspace id: %w", err)
	}

	if membership.ProjectID, err = uuid.Parse(project); err != nil {
		return entity.ProjectMembership{}, fmt.Errorf("parse membership project id: %w", err)
	}

	if membership.AccountID, err = uuid.Parse(account); err != nil {
		return entity.ProjectMembership{}, fmt.Errorf("parse membership account id: %w", err)
	}

	return membership, nil
}

func (r *memberRepository) Create(
	ctx context.Context,
	membership entity.ProjectMembership,
) (entity.ProjectMembership, error) {
	created, err := scanMembership(r.db.Querier(ctx).QueryRowContext(
		ctx,
		insertMemberQuery,
		membership.WorkspaceID.String(),
		membership.ProjectID.String(),
		membership.AccountID.String(),
	))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch {
			case pgErr.Code == uniqueViolationCode && pgErr.ConstraintName == memberUniqueIndex:
				return entity.ProjectMembership{}, entity.ErrProjectMemberExists
			case pgErr.Code == foreignKeyViolationCode && pgErr.ConstraintName == membershipForeignKey:
				return entity.ProjectMembership{}, entity.ErrMembershipNotFound
			}
		}

		return entity.ProjectMembership{}, fmt.Errorf("insert project member: %w", err)
	}

	return created, nil
}

func (r *memberRepository) Get(
	ctx context.Context,
	projectID, accountID uuid.UUID,
) (entity.ProjectMembership, error) {
	membership, err := scanMembership(r.db.Querier(ctx).QueryRowContext(
		ctx,
		memberQuery,
		projectID.String(),
		accountID.String(),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.ProjectMembership{}, entity.ErrProjectMembershipNotFound
		}

		return entity.ProjectMembership{}, fmt.Errorf("find project member: %w", err)
	}

	return membership, nil
}

func (r *memberRepository) ListByProjectID(
	ctx context.Context,
	projectID uuid.UUID,
) ([]entity.ProjectMembership, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, membersQuery, projectID.String())
	if err != nil {
		return nil, fmt.Errorf("list project members: %w", err)
	}

	defer func() { _ = rows.Close() }()

	memberships := make([]entity.ProjectMembership, 0)

	for rows.Next() {
		membership, err := scanMembership(rows)
		if err != nil {
			return nil, fmt.Errorf("scan project member: %w", err)
		}

		memberships = append(memberships, membership)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project members: %w", err)
	}

	return memberships, nil
}

func (r *memberRepository) Delete(ctx context.Context, projectID, accountID uuid.UUID) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		deleteMemberQuery,
		projectID.String(),
		accountID.String(),
	)
	if err != nil {
		return fmt.Errorf("delete project member: %w", err)
	}

	removed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read removed project member rows: %w", err)
	}

	if removed == 0 {
		return entity.ErrProjectMembershipNotFound
	}

	return nil
}
