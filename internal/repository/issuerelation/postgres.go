package issuerelation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const (
	uniqueViolationCode = "23505"
	pairUniqueIndex     = "workspace_issue_relations_pair_key"
)

const insertRelationQuery = `
INSERT INTO workspace_issue_relations (
    id, workspace_id, source_issue_id, target_issue_id, kind, created_by_account_id, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, workspace_id, source_issue_id, target_issue_id, kind,
          coalesce(created_by_account_id::text, ''), created_at`

const relationByIDQuery = `
SELECT id, workspace_id, source_issue_id, target_issue_id, kind,
       coalesce(created_by_account_id::text, ''), created_at
FROM workspace_issue_relations
WHERE id = $1 AND workspace_id = $2`

const deleteRelationQuery = `
DELETE FROM workspace_issue_relations WHERE id = $1`

const relationColumns = `
       r.id,
       r.kind,
       r.source_issue_id = $2 AS subject_is_source,
       r.created_at,
       o.id,
       o.reference_key,
       o.number,
       o.title,
       o.status,
       o.team_id,
       t.key,
       s.id,
       s.name,
       s.category,
       s.position`

const relationJoins = `
FROM workspace_issue_relations r
JOIN workspace_issues o
    ON o.id = CASE WHEN r.source_issue_id = $2 THEN r.target_issue_id ELSE r.source_issue_id END
JOIN workspace_teams t ON t.id = o.team_id
JOIN workspace_workflow_states s ON s.id = o.state_id AND s.team_id = o.team_id`

const relationsForIssueQuery = `
SELECT` + relationColumns + relationJoins + `
WHERE r.workspace_id = $1
  AND (r.source_issue_id = $2 OR r.target_issue_id = $2)
  AND ($3::boolean IS TRUE OR o.team_id = ANY($4::uuid[]))
ORDER BY r.created_at, r.id`

const relationForPairQuery = `
SELECT` + relationColumns + relationJoins + `
WHERE r.workspace_id = $1
  AND ((r.source_issue_id = $2 AND r.target_issue_id = $3)
       OR (r.source_issue_id = $3 AND r.target_issue_id = $2))
  AND ($4::boolean IS TRUE OR o.team_id = ANY($5::uuid[]))`

func translateWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) &&
		pgErr.Code == uniqueViolationCode &&
		pgErr.ConstraintName == pairUniqueIndex {
		return entity.ErrIssueRelationExists
	}

	return err
}

type relationRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.IssueRelation {
	return &relationRepository{db: db}
}

type scanner interface {
	Scan(dest ...any) error
}

func scanStored(row scanner) (entity.StoredIssueRelation, error) {
	var (
		stored    entity.StoredIssueRelation
		id        string
		workspace string
		source    string
		target    string
		kind      string
		createdBy string
	)

	if err := row.Scan(&id, &workspace, &source, &target, &kind, &createdBy, &stored.CreatedAt); err != nil {
		return entity.StoredIssueRelation{}, err
	}

	stored.Kind = entity.IssueRelationKind(kind)

	parsed, err := uuid.Parse(id)
	if err != nil {
		return entity.StoredIssueRelation{}, fmt.Errorf("parse relation id: %w", err)
	}

	stored.ID = parsed

	if stored.WorkspaceID, err = uuid.Parse(workspace); err != nil {
		return entity.StoredIssueRelation{}, fmt.Errorf("parse relation workspace id: %w", err)
	}

	if stored.SourceIssueID, err = uuid.Parse(source); err != nil {
		return entity.StoredIssueRelation{}, fmt.Errorf("parse relation source id: %w", err)
	}

	if stored.TargetIssueID, err = uuid.Parse(target); err != nil {
		return entity.StoredIssueRelation{}, fmt.Errorf("parse relation target id: %w", err)
	}

	if createdBy != "" {
		if stored.CreatedByAccountID, err = uuid.Parse(createdBy); err != nil {
			return entity.StoredIssueRelation{}, fmt.Errorf("parse relation author id: %w", err)
		}
	}

	return stored, nil
}

func scanRelation(row scanner) (entity.IssueRelation, error) {
	var (
		relation        entity.IssueRelation
		id              string
		kind            string
		subjectIsSource bool
		issueID         string
		teamID          string
		stateID         string
		status          string
		category        string
	)

	if err := row.Scan(
		&id,
		&kind,
		&subjectIsSource,
		&relation.CreatedAt,
		&issueID,
		&relation.Issue.ReferenceKey,
		&relation.Issue.Number,
		&relation.Issue.Title,
		&status,
		&teamID,
		&relation.Issue.TeamKey,
		&stateID,
		&relation.Issue.State.Name,
		&category,
		&relation.Issue.State.Position,
	); err != nil {
		return entity.IssueRelation{}, err
	}

	relation.Kind = entity.IssueRelationKind(kind).As(subjectIsSource)
	relation.Issue.Status = entity.IssueStatus(status)
	relation.Issue.State.Category = entity.StateCategory(category)

	parsed, err := uuid.Parse(id)
	if err != nil {
		return entity.IssueRelation{}, fmt.Errorf("parse relation id: %w", err)
	}

	relation.ID = parsed

	if relation.Issue.ID, err = uuid.Parse(issueID); err != nil {
		return entity.IssueRelation{}, fmt.Errorf("parse related issue id: %w", err)
	}

	if relation.Issue.TeamID, err = uuid.Parse(teamID); err != nil {
		return entity.IssueRelation{}, fmt.Errorf("parse related issue team id: %w", err)
	}

	if relation.Issue.State.ID, err = uuid.Parse(stateID); err != nil {
		return entity.IssueRelation{}, fmt.Errorf("parse related issue state id: %w", err)
	}

	return relation, nil
}

func teamIDs(scope entity.TeamScope) []string {
	ids := make([]string, 0, len(scope.TeamIDs))

	for _, id := range scope.TeamIDs {
		ids = append(ids, id.String())
	}

	return ids
}

func (r *relationRepository) Create(
	ctx context.Context,
	relation entity.StoredIssueRelation,
) (entity.StoredIssueRelation, error) {
	if relation.ID == uuid.Nil {
		relation.ID = uuid.New()
	}

	var author any

	if relation.CreatedByAccountID != uuid.Nil {
		author = relation.CreatedByAccountID.String()
	}

	created, err := scanStored(r.db.Querier(ctx).QueryRowContext(
		ctx,
		insertRelationQuery,
		relation.ID.String(),
		relation.WorkspaceID.String(),
		relation.SourceIssueID.String(),
		relation.TargetIssueID.String(),
		string(relation.Kind),
		author,
		time.Now().UTC(),
	))
	if err != nil {
		if translated := translateWriteError(err); !errors.Is(translated, err) {
			return entity.StoredIssueRelation{}, translated
		}

		return entity.StoredIssueRelation{}, fmt.Errorf("insert issue relation: %w", err)
	}

	return created, nil
}

func (r *relationRepository) GetByID(
	ctx context.Context,
	workspaceID, relationID uuid.UUID,
) (entity.StoredIssueRelation, error) {
	stored, err := scanStored(r.db.Querier(ctx).QueryRowContext(
		ctx,
		relationByIDQuery,
		relationID.String(),
		workspaceID.String(),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.StoredIssueRelation{}, entity.ErrIssueRelationNotFound
		}

		return entity.StoredIssueRelation{}, fmt.Errorf("find issue relation: %w", err)
	}

	return stored, nil
}

func (r *relationRepository) Delete(ctx context.Context, relationID uuid.UUID) error {
	result, err := r.db.Querier(ctx).ExecContext(ctx, deleteRelationQuery, relationID.String())
	if err != nil {
		return fmt.Errorf("delete issue relation: %w", err)
	}

	removed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read removed relation rows: %w", err)
	}

	if removed == 0 {
		return entity.ErrIssueRelationNotFound
	}

	return nil
}

func (r *relationRepository) ListForIssue(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	scope entity.TeamScope,
) ([]entity.IssueRelation, error) {
	rows, err := r.db.Querier(ctx).QueryContext(
		ctx,
		relationsForIssueQuery,
		workspaceID.String(),
		issueID.String(),
		scope.AllTeams,
		teamIDs(scope),
	)
	if err != nil {
		return nil, fmt.Errorf("list issue relations: %w", err)
	}

	defer func() { _ = rows.Close() }()

	relations := make([]entity.IssueRelation, 0)

	for rows.Next() {
		relation, err := scanRelation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan issue relation: %w", err)
		}

		relations = append(relations, relation)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate issue relations: %w", err)
	}

	return relations, nil
}

func (r *relationRepository) FindPair(
	ctx context.Context,
	workspaceID, issueID, counterpartID uuid.UUID,
	scope entity.TeamScope,
) (entity.IssueRelation, error) {
	relation, err := scanRelation(r.db.Querier(ctx).QueryRowContext(
		ctx,
		relationForPairQuery,
		workspaceID.String(),
		issueID.String(),
		counterpartID.String(),
		scope.AllTeams,
		teamIDs(scope),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.IssueRelation{}, entity.ErrIssueRelationNotFound
		}

		return entity.IssueRelation{}, fmt.Errorf("find issue relation for pair: %w", err)
	}

	return relation, nil
}
