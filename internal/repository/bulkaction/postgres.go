package bulkaction

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aarondl/sqlboiler/v4/types"
	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const insertActionQuery = `
INSERT INTO workspace_bulk_actions (
    id, workspace_id, requested_by_account_id, requested_actor_kind, requested_auth_method,
    change, status, expected, created_at, updated_at,
    requested_all_teams, requested_team_ids, requested_scopes
)
VALUES ($1, $2, $3, $7, $8, $4, 'queued', $5, $6, $6, $9, $10, $11)
RETURNING id, workspace_id, coalesce(requested_by_account_id::text, ''),
          requested_actor_kind, requested_auth_method, change, status,
          expected, processed, requested_all_teams, requested_team_ids, requested_scopes,
          started_at, finished_at, created_at, updated_at`

const actionByIDQuery = `
SELECT id, workspace_id, coalesce(requested_by_account_id::text, ''),
       requested_actor_kind, requested_auth_method, change, status,
       expected, processed, requested_all_teams, requested_team_ids, requested_scopes,
       started_at, finished_at, created_at, updated_at
FROM workspace_bulk_actions
WHERE id = $1 AND workspace_id = $2`

const claimActionQuery = `
UPDATE workspace_bulk_actions
SET status = 'running', started_at = $2, updated_at = $2
WHERE id = $1 AND status = 'queued'`

const advanceActionQuery = `
UPDATE workspace_bulk_actions
SET processed = processed + $2, updated_at = now()
WHERE id = $1`

const settleActionQuery = `
UPDATE workspace_bulk_actions
SET status = $2, finished_at = $3, updated_at = $3
WHERE id = $1 AND status = 'running'`

const outcomesQuery = `
SELECT o.issue_id, o.reference, o.outcome
FROM workspace_bulk_action_outcomes o
JOIN workspace_issues i ON i.id = o.issue_id
WHERE o.bulk_action_id = $1
  AND ($2::boolean IS TRUE OR i.team_id = ANY($3::uuid[]))
ORDER BY o.created_at, o.issue_id`

type actionRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.BulkAction {
	return &actionRepository{db: db}
}

type scanner interface {
	Scan(dest ...any) error
}

func scanAction(row scanner) (entity.BulkAction, error) {
	var (
		action    entity.BulkAction
		id        string
		workspace string
		requested string
		kind      string
		method    string
		change    []byte
		status    string
		expected  sql.NullInt64
		allTeams  bool
		teamIDs   types.StringArray
		scopes    types.StringArray
		started   sql.NullTime
		finished  sql.NullTime
	)

	if err := row.Scan(
		&id, &workspace, &requested, &kind, &method, &change, &status,
		&expected, &action.Processed, &allTeams, &teamIDs, &scopes,
		&started, &finished,
		&action.CreatedAt, &action.UpdatedAt,
	); err != nil {
		return entity.BulkAction{}, err
	}

	action.Status = entity.BulkActionStatus(status)
	action.RequestedActorKind = entity.ActorKind(kind)
	action.RequestedAuthMethod = entity.SessionAuthMethod(method)
	action.Authority = entity.NewRequestedAuthority(allTeams, teamIDs, scopes)

	if err := json.Unmarshal(change, &action.Change); err != nil {
		return entity.BulkAction{}, fmt.Errorf("decode bulk change: %w", err)
	}

	if expected.Valid {
		size := int(expected.Int64)
		action.Expected = &size
	}

	if started.Valid {
		at := started.Time
		action.StartedAt = &at
	}

	if finished.Valid {
		at := finished.Time
		action.FinishedAt = &at
	}

	parsed, err := uuid.Parse(id)
	if err != nil {
		return entity.BulkAction{}, fmt.Errorf("parse bulk action id: %w", err)
	}

	action.ID = parsed

	if action.WorkspaceID, err = uuid.Parse(workspace); err != nil {
		return entity.BulkAction{}, fmt.Errorf("parse bulk action workspace id: %w", err)
	}

	if requested != "" {
		if action.RequestedByAccount, err = uuid.Parse(requested); err != nil {
			return entity.BulkAction{}, fmt.Errorf("parse bulk action author id: %w", err)
		}
	}

	return action, nil
}

func (r *actionRepository) Create(
	ctx context.Context,
	action entity.BulkAction,
) (entity.BulkAction, error) {
	if action.ID == uuid.Nil {
		action.ID = uuid.New()
	}

	change, err := json.Marshal(action.Change)
	if err != nil {
		return entity.BulkAction{}, fmt.Errorf("encode bulk change: %w", err)
	}

	var (
		requested any
		expected  any
	)

	if action.RequestedByAccount != uuid.Nil {
		requested = action.RequestedByAccount.String()
	}

	if action.Expected != nil {
		expected = *action.Expected
	}

	created, err := scanAction(r.db.Querier(ctx).QueryRowContext(
		ctx,
		insertActionQuery,
		action.ID.String(),
		action.WorkspaceID.String(),
		requested,
		change,
		expected,
		time.Now().UTC(),
		string(action.RequestedActorKind),
		string(action.RequestedAuthMethod),
		action.Authority.AllTeams,
		types.StringArray(action.Authority.TeamStrings()),
		nullableStrings(action.Authority.ScopeStrings()),
	))
	if err != nil {
		return entity.BulkAction{}, fmt.Errorf("insert bulk action: %w", err)
	}

	return created, nil
}

func (r *actionRepository) GetByID(
	ctx context.Context,
	workspaceID, actionID uuid.UUID,
) (entity.BulkAction, error) {
	action, err := scanAction(r.db.Querier(ctx).QueryRowContext(
		ctx,
		actionByIDQuery,
		actionID.String(),
		workspaceID.String(),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.BulkAction{}, entity.ErrBulkActionNotFound
		}

		return entity.BulkAction{}, fmt.Errorf("find bulk action: %w", err)
	}

	return action, nil
}

func (r *actionRepository) Claim(ctx context.Context, actionID uuid.UUID, at time.Time) error {
	result, err := r.db.Querier(ctx).ExecContext(ctx, claimActionQuery, actionID.String(), at)
	if err != nil {
		return fmt.Errorf("claim bulk action: %w", err)
	}

	claimed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read claimed bulk action rows: %w", err)
	}

	if claimed == 0 {
		return entity.ErrBulkActionNotFound
	}

	return nil
}

func (r *actionRepository) Advance(ctx context.Context, actionID uuid.UUID, by int) error {
	if _, err := r.db.Querier(ctx).ExecContext(ctx, advanceActionQuery, actionID.String(), by); err != nil {
		return fmt.Errorf("advance bulk action: %w", err)
	}

	return nil
}

func (r *actionRepository) Settle(
	ctx context.Context,
	actionID uuid.UUID,
	status entity.BulkActionStatus,
	at time.Time,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		settleActionQuery,
		actionID.String(),
		string(status),
		at,
	); err != nil {
		return fmt.Errorf("settle bulk action: %w", err)
	}

	return nil
}

func (r *actionRepository) RecordOutcomes(
	ctx context.Context,
	actionID uuid.UUID,
	outcomes []entity.BulkActionOutcome,
) error {
	if len(outcomes) == 0 {
		return nil
	}

	values := make([]string, 0, len(outcomes))
	args := make([]any, 0, len(outcomes)*4+1)
	args = append(args, actionID.String())

	for i, outcome := range outcomes {
		base := i*3 + 2
		values = append(values, fmt.Sprintf("($1, $%d, $%d, $%d)", base, base+1, base+2))
		args = append(args, outcome.IssueID.String(), outcome.Reference, string(outcome.Outcome))
	}

	query := `
INSERT INTO workspace_bulk_action_outcomes (bulk_action_id, issue_id, reference, outcome)
VALUES ` + strings.Join(values, ", ") + `
ON CONFLICT (bulk_action_id, issue_id) DO UPDATE SET outcome = excluded.outcome`

	if _, err := r.db.Querier(ctx).ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("record bulk action outcomes: %w", err)
	}

	return nil
}

func (r *actionRepository) ListOutcomes(
	ctx context.Context,
	actionID uuid.UUID,
	scope entity.TeamScope,
) ([]entity.BulkActionOutcome, error) {
	rows, err := r.db.Querier(ctx).QueryContext(
		ctx,
		outcomesQuery,
		actionID.String(),
		scope.AllTeams,
		teamIDs(scope),
	)
	if err != nil {
		return nil, fmt.Errorf("list bulk action outcomes: %w", err)
	}

	defer func() { _ = rows.Close() }()

	outcomes := make([]entity.BulkActionOutcome, 0)

	for rows.Next() {
		var (
			outcome entity.BulkActionOutcome
			issueID string
			verdict string
		)

		if err := rows.Scan(&issueID, &outcome.Reference, &verdict); err != nil {
			return nil, fmt.Errorf("scan bulk action outcome: %w", err)
		}

		parsed, err := uuid.Parse(issueID)
		if err != nil {
			return nil, fmt.Errorf("parse outcome issue id: %w", err)
		}

		outcome.IssueID = parsed
		outcome.Outcome = entity.BulkOutcome(verdict)
		outcomes = append(outcomes, outcome)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bulk action outcomes: %w", err)
	}

	return outcomes, nil
}

func nullableStrings(values []string) any {
	if values == nil {
		return nil
	}

	return types.StringArray(values)
}

func teamIDs(scope entity.TeamScope) []string {
	ids := make([]string, 0, len(scope.TeamIDs))

	for _, id := range scope.TeamIDs {
		ids = append(ids, id.String())
	}

	return ids
}
