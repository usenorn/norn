package cycle

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
	exclusionViolationCode = "23P01"
	overlapConstraint      = "workspace_cycles_no_overlap"
)

const cycleColumns = `
       c.id,
       c.workspace_id,
       c.team_id,
       c.number,
       to_char(c.starts_on, 'YYYY-MM-DD'),
       to_char(c.ends_on, 'YYYY-MM-DD'),
       c.closed_at,
       coalesce(c.closed_by::text, ''),
       c.rollover,
       c.created_at,
       c.updated_at,
       t.key`

const cycleJoins = `
FROM workspace_cycles c
JOIN workspace_teams t ON t.id = c.team_id`

const insertCycleQuery = `
WITH inserted AS (
    INSERT INTO workspace_cycles (workspace_id, team_id, number, starts_on, ends_on,
                                  closed_at, closed_by, rollover, created_at, updated_at)
    VALUES ($1, $2, $3, $4::date, $5::date, $6, $7::uuid, $8, $9, $10)
    RETURNING id, workspace_id, team_id, number, starts_on, ends_on,
              closed_at, closed_by, rollover, created_at, updated_at
)
SELECT` + cycleColumns + `
FROM inserted c
JOIN workspace_teams t ON t.id = c.team_id`

const visibleCycleQuery = `
SELECT` + cycleColumns + cycleJoins + `
WHERE c.id = $1
  AND c.workspace_id = $2
  AND ($3::boolean IS TRUE OR c.team_id = ANY($4::uuid[]))`

const cycleByNumberQuery = `
SELECT` + cycleColumns + cycleJoins + `
WHERE c.workspace_id = $1
  AND c.team_id = $2
  AND c.number = $3
  AND ($4::boolean IS TRUE OR c.team_id = ANY($5::uuid[]))`

const visibleCyclesQuery = `
SELECT` + cycleColumns + cycleJoins + `
WHERE c.workspace_id = $1
  AND ($2::boolean IS TRUE OR c.team_id = ANY($3::uuid[]))
  AND ($4::boolean IS NOT TRUE OR c.team_id = $5::uuid)
ORDER BY c.starts_on, c.number`

const cyclesForTeamQuery = `
SELECT` + cycleColumns + cycleJoins + `
WHERE c.team_id = $1
ORDER BY c.starts_on, c.number`

const lockCycleQuery = `
SELECT` + cycleColumns + cycleJoins + `
WHERE c.id = $1
FOR UPDATE OF c`

const closeCycleQuery = `
WITH closed AS (
    UPDATE workspace_cycles
    SET closed_at  = $2,
        closed_by  = $3::uuid,
        rollover   = $4,
        updated_at = $2
    WHERE id = $1 AND closed_at IS NULL
    RETURNING id, workspace_id, team_id, number, starts_on, ends_on,
              closed_at, closed_by, rollover, created_at, updated_at
)
SELECT` + cycleColumns + `
FROM closed c
JOIN workspace_teams t ON t.id = c.team_id`

const nextCycleAfterQuery = `
SELECT` + cycleColumns + cycleJoins + `
WHERE c.team_id = $1
  AND c.starts_on > $2::date
  AND c.closed_at IS NULL
ORDER BY c.starts_on
LIMIT 1`

const highestCycleNumberQuery = `
SELECT coalesce(max(number), 0) FROM workspace_cycles WHERE team_id = $1`

const deleteCycleQuery = `DELETE FROM workspace_cycles WHERE id = $1`

const deleteVacantCyclesQuery = `
DELETE FROM workspace_cycles c
WHERE c.team_id = $1
  AND c.starts_on >= $2::date
  AND c.closed_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM workspace_issues i WHERE i.cycle_id = c.id)`

func translateWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) &&
		pgErr.Code == exclusionViolationCode &&
		pgErr.ConstraintName == overlapConstraint {
		return entity.ErrCycleOverlaps
	}

	return err
}

type cycleRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.Cycle {
	return &cycleRepository{db: db}
}

type scanner interface {
	Scan(dest ...any) error
}

func teamIDs(scope entity.TeamScope) []string {
	ids := make([]string, 0, len(scope.TeamIDs))

	for _, id := range scope.TeamIDs {
		ids = append(ids, id.String())
	}

	return ids
}

func scanCycle(row scanner) (entity.Cycle, error) {
	var (
		cycle     entity.Cycle
		id        string
		workspace string
		team      string
		closedBy  string
		rollover  string
	)

	if err := row.Scan(
		&id,
		&workspace,
		&team,
		&cycle.Number,
		&cycle.StartsOn,
		&cycle.EndsOn,
		&cycle.ClosedAt,
		&closedBy,
		&rollover,
		&cycle.CreatedAt,
		&cycle.UpdatedAt,
		&cycle.TeamKey,
	); err != nil {
		return entity.Cycle{}, err
	}

	cycle.Rollover = entity.CycleRollover(rollover)

	parsed, err := uuid.Parse(id)
	if err != nil {
		return entity.Cycle{}, fmt.Errorf("parse cycle id: %w", err)
	}

	cycle.ID = parsed

	if cycle.WorkspaceID, err = uuid.Parse(workspace); err != nil {
		return entity.Cycle{}, fmt.Errorf("parse cycle workspace id: %w", err)
	}

	if cycle.TeamID, err = uuid.Parse(team); err != nil {
		return entity.Cycle{}, fmt.Errorf("parse cycle team id: %w", err)
	}

	if closedBy != "" {
		if cycle.ClosedByAccountID, err = uuid.Parse(closedBy); err != nil {
			return entity.Cycle{}, fmt.Errorf("parse cycle closing account id: %w", err)
		}
	}

	return cycle, nil
}

func collectCycles(rows *sql.Rows) ([]entity.Cycle, error) {
	defer func() { _ = rows.Close() }()

	cycles := make([]entity.Cycle, 0)

	for rows.Next() {
		cycle, err := scanCycle(rows)
		if err != nil {
			return nil, fmt.Errorf("scan cycle: %w", err)
		}

		cycles = append(cycles, cycle)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cycles: %w", err)
	}

	return cycles, nil
}

func (r *cycleRepository) Create(ctx context.Context, cycle entity.Cycle) (entity.Cycle, error) {
	createdAt, updatedAt := entity.OriginStamp(cycle.Origin, time.Now().UTC())

	var (
		closedAt any
		closedBy any
	)

	if cycle.ClosedAt != nil {
		closedAt = *cycle.ClosedAt
	}

	if cycle.ClosedByAccountID != uuid.Nil {
		closedBy = cycle.ClosedByAccountID.String()
	}

	rollover := entity.CycleRolloverNone
	if cycle.Closed() {
		rollover = cycle.Rollover
	}

	created, err := scanCycle(r.db.Querier(ctx).QueryRowContext(
		ctx,
		insertCycleQuery,
		cycle.WorkspaceID.String(),
		cycle.TeamID.String(),
		cycle.Number,
		cycle.StartsOn,
		cycle.EndsOn,
		closedAt,
		closedBy,
		string(rollover),
		createdAt,
		updatedAt,
	))
	if err != nil {
		if translated := translateWriteError(err); !errors.Is(translated, err) {
			return entity.Cycle{}, translated
		}

		return entity.Cycle{}, fmt.Errorf("insert cycle: %w", err)
	}

	return created, nil
}

func (r *cycleRepository) GetVisible(
	ctx context.Context,
	workspaceID, cycleID uuid.UUID,
	scope entity.TeamScope,
) (entity.Cycle, error) {
	cycle, err := scanCycle(r.db.Querier(ctx).QueryRowContext(
		ctx,
		visibleCycleQuery,
		cycleID.String(),
		workspaceID.String(),
		scope.AllTeams,
		teamIDs(scope),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Cycle{}, entity.ErrCycleNotFound
		}

		return entity.Cycle{}, fmt.Errorf("find cycle: %w", err)
	}

	return cycle, nil
}

func (r *cycleRepository) GetByNumber(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
	number int,
	scope entity.TeamScope,
) (entity.Cycle, error) {
	cycle, err := scanCycle(r.db.Querier(ctx).QueryRowContext(
		ctx,
		cycleByNumberQuery,
		workspaceID.String(),
		teamID.String(),
		number,
		scope.AllTeams,
		teamIDs(scope),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Cycle{}, entity.ErrCycleNotFound
		}

		return entity.Cycle{}, fmt.Errorf("find cycle by number: %w", err)
	}

	return cycle, nil
}

func (r *cycleRepository) ListVisible(
	ctx context.Context,
	scope entity.TeamScope,
	filter repository.CycleFilter,
) ([]entity.Cycle, error) {
	team := uuid.Nil
	if filter.TeamID != nil {
		team = *filter.TeamID
	}

	rows, err := r.db.Querier(ctx).QueryContext(
		ctx,
		visibleCyclesQuery,
		scope.WorkspaceID.String(),
		scope.AllTeams,
		teamIDs(scope),
		filter.TeamID != nil,
		team.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("list cycles: %w", err)
	}

	return collectCycles(rows)
}

func (r *cycleRepository) ListByTeamID(
	ctx context.Context,
	teamID uuid.UUID,
) ([]entity.Cycle, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, cyclesForTeamQuery, teamID.String())
	if err != nil {
		return nil, fmt.Errorf("list team cycles: %w", err)
	}

	return collectCycles(rows)
}

func (r *cycleRepository) LockByID(ctx context.Context, cycleID uuid.UUID) (entity.Cycle, error) {
	cycle, err := scanCycle(r.db.Querier(ctx).QueryRowContext(ctx, lockCycleQuery, cycleID.String()))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Cycle{}, entity.ErrCycleNotFound
		}

		return entity.Cycle{}, fmt.Errorf("lock cycle: %w", err)
	}

	return cycle, nil
}

func (r *cycleRepository) Close(
	ctx context.Context,
	cycleID uuid.UUID,
	closedAt time.Time,
	closedBy *uuid.UUID,
	rollover entity.CycleRollover,
) (entity.Cycle, error) {
	var account any

	if closedBy != nil {
		account = closedBy.String()
	}

	cycle, err := scanCycle(r.db.Querier(ctx).QueryRowContext(
		ctx,
		closeCycleQuery,
		cycleID.String(),
		closedAt,
		account,
		string(rollover),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Cycle{}, entity.ErrCycleClosed
		}

		return entity.Cycle{}, fmt.Errorf("close cycle: %w", err)
	}

	return cycle, nil
}

func (r *cycleRepository) NextAfter(
	ctx context.Context,
	teamID uuid.UUID,
	endsOn string,
) (entity.Cycle, error) {
	cycle, err := scanCycle(r.db.Querier(ctx).QueryRowContext(
		ctx,
		nextCycleAfterQuery,
		teamID.String(),
		endsOn,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Cycle{}, entity.ErrCycleNoNextCycle
		}

		return entity.Cycle{}, fmt.Errorf("find next cycle: %w", err)
	}

	return cycle, nil
}

func (r *cycleRepository) HighestNumber(ctx context.Context, teamID uuid.UUID) (int, error) {
	var highest int

	if err := r.db.Querier(ctx).QueryRowContext(
		ctx,
		highestCycleNumberQuery,
		teamID.String(),
	).Scan(&highest); err != nil {
		return 0, fmt.Errorf("read highest cycle number: %w", err)
	}

	return highest, nil
}

func (r *cycleRepository) Delete(ctx context.Context, cycleID uuid.UUID) error {
	result, err := r.db.Querier(ctx).ExecContext(ctx, deleteCycleQuery, cycleID.String())
	if err != nil {
		return fmt.Errorf("delete cycle: %w", err)
	}

	removed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read removed cycle rows: %w", err)
	}

	if removed == 0 {
		return entity.ErrCycleNotFound
	}

	return nil
}

func (r *cycleRepository) DeleteVacantFrom(
	ctx context.Context,
	teamID uuid.UUID,
	from string,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		deleteVacantCyclesQuery,
		teamID.String(),
		from,
	); err != nil {
		return fmt.Errorf("delete vacant cycles: %w", err)
	}

	return nil
}
