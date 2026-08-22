package execution

import (
	"context"
	"database/sql"
	"encoding/json"
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
	sourceUniqueIndex   = "workspace_execution_events_source_key"
	attemptUniqueIndex  = "workspace_executions_attempt_key"
)

const terminalStates = `('completed', 'failed', 'cancelled', 'interrupted')`

const executionColumns = `
       e.id,
       e.workspace_id,
       e.issue_id,
       i.reference_key,
       i.number,
       i.team_id,
       e.delegation_id,
       e.agent_id,
       a.name,
       coalesce(e.runner_id::text, ''),
       coalesce(e.codebase_id::text, ''),
       e.attempt,
       e.state,
       e.reason,
       e.params,
       e.lease_expires_at,
       e.queued_at,
       e.started_at,
       e.finished_at,
       e.updated_at`

const executionJoins = `
FROM workspace_executions e
JOIN workspace_issues i ON i.id = e.issue_id
JOIN workspace_agents a ON a.id = e.agent_id`

const insertExecutionQuery = `
WITH inserted AS (
    INSERT INTO workspace_executions
        (id, workspace_id, issue_id, delegation_id, agent_id, runner_id, codebase_id,
         attempt, state, params, queued_at, updated_at)
    VALUES ($1, $2, $3, $4, $5, nullif($6, '')::uuid, nullif($7, '')::uuid,
            $8, $9, $10::jsonb, $11, $11)
    RETURNING *
)
SELECT` + executionColumns + `
FROM inserted e
JOIN workspace_issues i ON i.id = e.issue_id
JOIN workspace_agents a ON a.id = e.agent_id`

const executionByIDQuery = `
SELECT` + executionColumns + executionJoins + `
WHERE e.id = $1`

const executionsByIssueQuery = `
SELECT` + executionColumns + executionJoins + `
WHERE e.workspace_id = $1 AND e.issue_id = $2
ORDER BY e.attempt DESC`

const liveExecutionsByRunnerQuery = `
SELECT` + executionColumns + executionJoins + `
WHERE e.runner_id = $1 AND e.state NOT IN ` + terminalStates + `
ORDER BY e.queued_at, e.id`

const nextExecutionAttemptQuery = `
SELECT coalesce(max(attempt), 0) + 1 FROM workspace_executions WHERE issue_id = $1`

const moveExecutionQuery = `
WITH updated AS (
    UPDATE workspace_executions
    SET state            = $3,
        reason           = $4,
        runner_id        = coalesce(nullif($5, '')::uuid, runner_id),
        lease_expires_at = $6::timestamptz,
        started_at       = coalesce(started_at, CASE WHEN $3 = 'preparing' THEN $7::timestamptz END),
        finished_at      = CASE WHEN $3 IN ` + terminalStates + ` THEN $7::timestamptz END,
        updated_at       = $7::timestamptz
    WHERE id = $1 AND state = $2
    RETURNING *
)
SELECT` + executionColumns + `
FROM updated e
JOIN workspace_issues i ON i.id = e.issue_id
JOIN workspace_agents a ON a.id = e.agent_id`

const renewExecutionLeasesQuery = `
UPDATE workspace_executions
SET lease_expires_at = $2
WHERE runner_id = $1 AND state NOT IN ` + terminalStates + ` AND state <> 'queued'`

const expiredExecutionLeasesQuery = `
SELECT` + executionColumns + executionJoins + `
WHERE e.lease_expires_at IS NOT NULL
  AND e.lease_expires_at < $1
  AND e.state NOT IN ` + terminalStates + `
ORDER BY e.lease_expires_at
LIMIT $2`

const executionEventColumns = `
       id,
       execution_id,
       sequence,
       kind,
       from_state,
       to_state,
       actor_kind,
       coalesce(actor_account_id::text, ''),
       coalesce(actor_agent_id::text, ''),
       coalesce(actor_runner_id::text, ''),
       reason,
       detail,
       source_id,
       occurred_at,
       recorded_at`

const appendExecutionEventQuery = `
WITH inserted AS (
    INSERT INTO workspace_execution_events
        (execution_id, kind, from_state, to_state, actor_kind, actor_account_id,
         actor_agent_id, actor_runner_id, reason, detail, source_id, occurred_at, recorded_at)
    VALUES ($1, $2, $3, $4, $5, nullif($6, '')::uuid, nullif($7, '')::uuid,
            nullif($8, '')::uuid, $9, $10::jsonb, $11, $12, $12)
    RETURNING *
)
SELECT` + executionEventColumns + `
FROM inserted`

const executionEventsQuery = `
SELECT` + executionEventColumns + `
FROM workspace_execution_events
WHERE execution_id = $1 AND sequence > $2
ORDER BY sequence
LIMIT $3`

type executionRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.Execution {
	return &executionRepository{db: db}
}

type scanner interface {
	Scan(dest ...any) error
}

type storedParams struct {
	Tool    string `json:"tool"`
	Model   string `json:"model"`
	Runtime string `json:"runtime"`
	Brief   string `json:"brief"`
}

func encodeParams(params entity.ExecutionParams) ([]byte, error) {
	encoded, err := json.Marshal(storedParams{
		Tool:    params.Tool,
		Model:   params.Model,
		Runtime: string(params.Runtime),
		Brief:   params.Brief,
	})
	if err != nil {
		return nil, fmt.Errorf("encode execution params: %w", err)
	}

	return encoded, nil
}

func optionalID(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}

	return id.String()
}

func parseOptionalID(raw string) (uuid.UUID, error) {
	if raw == "" {
		return uuid.Nil, nil
	}

	return uuid.Parse(raw)
}

func scanExecution(row scanner) (entity.Execution, error) {
	var (
		execution    entity.Execution
		workspaceID  string
		issueID      string
		teamID       string
		referenceKey string
		number       int
		delegation   string
		agentID      string
		runnerID     string
		codebaseID   string
		state        string
		params       []byte
	)

	if err := row.Scan(
		&execution.ID,
		&workspaceID,
		&issueID,
		&referenceKey,
		&number,
		&teamID,
		&delegation,
		&agentID,
		&execution.AgentName,
		&runnerID,
		&codebaseID,
		&execution.Attempt,
		&state,
		&execution.Reason,
		&params,
		&execution.LeaseExpiresAt,
		&execution.QueuedAt,
		&execution.StartedAt,
		&execution.FinishedAt,
		&execution.UpdatedAt,
	); err != nil {
		return entity.Execution{}, err
	}

	var err error

	if execution.WorkspaceID, err = uuid.Parse(workspaceID); err != nil {
		return entity.Execution{}, fmt.Errorf("parse execution workspace id: %w", err)
	}

	if execution.IssueID, err = uuid.Parse(issueID); err != nil {
		return entity.Execution{}, fmt.Errorf("parse execution issue id: %w", err)
	}

	if execution.TeamID, err = uuid.Parse(teamID); err != nil {
		return entity.Execution{}, fmt.Errorf("parse execution team id: %w", err)
	}

	if execution.DelegationID, err = uuid.Parse(delegation); err != nil {
		return entity.Execution{}, fmt.Errorf("parse execution delegation id: %w", err)
	}

	if execution.AgentID, err = uuid.Parse(agentID); err != nil {
		return entity.Execution{}, fmt.Errorf("parse execution agent id: %w", err)
	}

	if execution.RunnerID, err = parseOptionalID(runnerID); err != nil {
		return entity.Execution{}, fmt.Errorf("parse execution runner id: %w", err)
	}

	if execution.CodebaseID, err = parseOptionalID(codebaseID); err != nil {
		return entity.Execution{}, fmt.Errorf("parse execution codebase id: %w", err)
	}

	execution.IssueReference = entity.Issue{ReferenceKey: referenceKey, Number: number}.Reference()
	execution.State = entity.ExecutionState(state)

	var stored storedParams
	if err := json.Unmarshal(params, &stored); err != nil {
		return entity.Execution{}, fmt.Errorf("decode execution params: %w", err)
	}

	execution.Params = entity.ExecutionParams{
		Tool:    stored.Tool,
		Model:   stored.Model,
		Runtime: entity.CodebaseRuntime(stored.Runtime),
		Brief:   stored.Brief,
	}

	return execution, nil
}

func scanExecutionEvent(row scanner) (entity.ExecutionEvent, error) {
	var (
		event     entity.ExecutionEvent
		id        string
		kind      string
		fromState string
		toState   string
		actorKind string
		accountID string
		agentID   string
		runnerID  string
	)

	if err := row.Scan(
		&id,
		&event.ExecutionID,
		&event.Sequence,
		&kind,
		&fromState,
		&toState,
		&actorKind,
		&accountID,
		&agentID,
		&runnerID,
		&event.Reason,
		&event.Detail,
		&event.SourceID,
		&event.OccurredAt,
		&event.RecordedAt,
	); err != nil {
		return entity.ExecutionEvent{}, err
	}

	parsed, err := uuid.Parse(id)
	if err != nil {
		return entity.ExecutionEvent{}, fmt.Errorf("parse execution event id: %w", err)
	}

	event.ID = parsed
	event.Kind = entity.ExecutionEventKind(kind)
	event.FromState = entity.ExecutionState(fromState)
	event.ToState = entity.ExecutionState(toState)
	event.Actor.Kind = entity.ActorKind(actorKind)

	if event.Actor.AccountID, err = parseOptionalID(accountID); err != nil {
		return entity.ExecutionEvent{}, fmt.Errorf("parse execution event account id: %w", err)
	}

	if event.Actor.AgentID, err = parseOptionalID(agentID); err != nil {
		return entity.ExecutionEvent{}, fmt.Errorf("parse execution event agent id: %w", err)
	}

	if event.Actor.RunnerID, err = parseOptionalID(runnerID); err != nil {
		return entity.ExecutionEvent{}, fmt.Errorf("parse execution event runner id: %w", err)
	}

	return event, nil
}

func (r *executionRepository) find(
	ctx context.Context,
	query string,
	args ...any,
) (entity.Execution, error) {
	execution, err := scanExecution(r.db.Querier(ctx).QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Execution{}, entity.ErrExecutionNotFound
		}

		return entity.Execution{}, fmt.Errorf("find execution: %w", err)
	}

	return execution, nil
}

func (r *executionRepository) list(
	ctx context.Context,
	query string,
	args ...any,
) ([]entity.Execution, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list executions: %w", err)
	}

	defer func() { _ = rows.Close() }()

	executions := make([]entity.Execution, 0)

	for rows.Next() {
		execution, err := scanExecution(rows)
		if err != nil {
			return nil, fmt.Errorf("scan execution: %w", err)
		}

		executions = append(executions, execution)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate executions: %w", err)
	}

	return executions, nil
}

func (r *executionRepository) Create(
	ctx context.Context,
	execution repository.NewExecution,
) (entity.Execution, error) {
	params, err := encodeParams(execution.Params)
	if err != nil {
		return entity.Execution{}, err
	}

	created, err := scanExecution(r.db.Querier(ctx).QueryRowContext(
		ctx,
		insertExecutionQuery,
		execution.ID,
		execution.WorkspaceID.String(),
		execution.IssueID.String(),
		execution.DelegationID.String(),
		execution.AgentID.String(),
		optionalID(execution.RunnerID),
		optionalID(execution.CodebaseID),
		execution.Attempt,
		string(entity.ExecutionQueued),
		params,
		execution.QueuedAt,
	))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == uniqueViolationCode &&
			pgErr.ConstraintName == attemptUniqueIndex {
			return entity.Execution{}, entity.ErrExecutionTransition
		}

		return entity.Execution{}, fmt.Errorf("insert execution: %w", err)
	}

	return created, nil
}

func (r *executionRepository) GetByID(
	ctx context.Context,
	executionID string,
) (entity.Execution, error) {
	return r.find(ctx, executionByIDQuery, executionID)
}

func (r *executionRepository) ListByIssue(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) ([]entity.Execution, error) {
	return r.list(ctx, executionsByIssueQuery, workspaceID.String(), issueID.String())
}

func (r *executionRepository) ListLiveByRunner(
	ctx context.Context,
	runnerID uuid.UUID,
) ([]entity.Execution, error) {
	return r.list(ctx, liveExecutionsByRunnerQuery, runnerID.String())
}

func (r *executionRepository) NextAttempt(
	ctx context.Context,
	issueID uuid.UUID,
) (int, error) {
	var attempt int

	if err := r.db.Querier(ctx).QueryRowContext(
		ctx, nextExecutionAttemptQuery, issueID.String(),
	).Scan(&attempt); err != nil {
		return 0, fmt.Errorf("count this issue's executions: %w", err)
	}

	return attempt, nil
}

func (r *executionRepository) Move(
	ctx context.Context,
	executionID string,
	move repository.ExecutionMove,
) (entity.Execution, error) {
	moved, err := scanExecution(r.db.Querier(ctx).QueryRowContext(
		ctx,
		moveExecutionQuery,
		executionID,
		string(move.From),
		string(move.To),
		move.Reason,
		optionalID(move.RunnerID),
		move.LeaseExpiresAt,
		move.At,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Execution{}, entity.ErrExecutionTransition
		}

		return entity.Execution{}, fmt.Errorf("move execution: %w", err)
	}

	return moved, nil
}

func (r *executionRepository) RenewLeases(
	ctx context.Context,
	runnerID uuid.UUID,
	expiresAt time.Time,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, renewExecutionLeasesQuery, runnerID.String(), expiresAt,
	); err != nil {
		return fmt.Errorf("renew execution leases: %w", err)
	}

	return nil
}

func (r *executionRepository) ExpiredLeases(
	ctx context.Context,
	now time.Time,
	limit int,
) ([]entity.Execution, error) {
	return r.list(ctx, expiredExecutionLeasesQuery, now, limit)
}

func (r *executionRepository) AppendEvent(
	ctx context.Context,
	event entity.ExecutionEvent,
) (entity.ExecutionEvent, error) {
	detail := event.Detail
	if len(detail) == 0 {
		detail = []byte("{}")
	}

	appended, err := scanExecutionEvent(r.db.Querier(ctx).QueryRowContext(
		ctx,
		appendExecutionEventQuery,
		event.ExecutionID,
		string(event.Kind),
		string(event.FromState),
		string(event.ToState),
		string(event.Actor.Kind),
		optionalID(event.Actor.AccountID),
		optionalID(event.Actor.AgentID),
		optionalID(event.Actor.RunnerID),
		event.Reason,
		detail,
		event.SourceID,
		event.OccurredAt,
	))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == uniqueViolationCode &&
			pgErr.ConstraintName == sourceUniqueIndex {
			return entity.ExecutionEvent{}, entity.ErrExecutionEventRecorded
		}

		return entity.ExecutionEvent{}, fmt.Errorf("append execution event: %w", err)
	}

	return appended, nil
}

func (r *executionRepository) ListEvents(
	ctx context.Context,
	executionID string,
	page entity.ExecutionTimelinePage,
) ([]entity.ExecutionEvent, error) {
	bounded := page.Normalized()

	rows, err := r.db.Querier(ctx).QueryContext(
		ctx, executionEventsQuery, executionID, bounded.After, bounded.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list execution events: %w", err)
	}

	defer func() { _ = rows.Close() }()

	events := make([]entity.ExecutionEvent, 0)

	for rows.Next() {
		event, err := scanExecutionEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan execution event: %w", err)
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate execution events: %w", err)
	}

	return events, nil
}
