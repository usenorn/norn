package cycle

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const cadenceColumns = `
       team_id,
       workspace_id,
       length_weeks,
       to_char(anchor_on, 'YYYY-MM-DD'),
       created_at,
       updated_at`

const cadenceByTeamQuery = `
SELECT` + cadenceColumns + `
FROM workspace_team_cycle_cadences
WHERE team_id = $1`

const lockCadenceQuery = cadenceByTeamQuery + `
FOR UPDATE`

const upsertCadenceQuery = `
INSERT INTO workspace_team_cycle_cadences (team_id, workspace_id, length_weeks, anchor_on)
VALUES ($1, $2, $3, $4::date)
ON CONFLICT (team_id) DO UPDATE
    SET length_weeks = excluded.length_weeks,
        anchor_on    = excluded.anchor_on,
        updated_at   = $5
RETURNING` + cadenceColumns

const deleteCadenceQuery = `
DELETE FROM workspace_team_cycle_cadences WHERE team_id = $1`

const cadencesByWorkspaceQuery = `
SELECT` + cadenceColumns + `
FROM workspace_team_cycle_cadences
WHERE workspace_id = $1`

const allCadencesQuery = `
SELECT c.team_id,
       c.workspace_id,
       c.length_weeks,
       to_char(c.anchor_on, 'YYYY-MM-DD'),
       c.created_at,
       c.updated_at,
       w.timezone
FROM workspace_team_cycle_cadences c
JOIN workspaces w ON w.id = c.workspace_id
JOIN workspace_teams t ON t.id = c.team_id
WHERE w.status = 'active' AND t.status = 'active'
ORDER BY c.team_id`

type cadenceRepository struct {
	db *postgres.Client
}

func NewCadence(db *postgres.Client) repository.CycleCadence {
	return &cadenceRepository{db: db}
}

func scanCadence(row scanner) (entity.CycleCadence, error) {
	var (
		cadence   entity.CycleCadence
		team      string
		workspace string
	)

	if err := row.Scan(
		&team,
		&workspace,
		&cadence.LengthWeeks,
		&cadence.AnchorOn,
		&cadence.CreatedAt,
		&cadence.UpdatedAt,
	); err != nil {
		return entity.CycleCadence{}, err
	}

	parsed, err := uuid.Parse(team)
	if err != nil {
		return entity.CycleCadence{}, fmt.Errorf("parse cadence team id: %w", err)
	}

	cadence.TeamID = parsed

	if cadence.WorkspaceID, err = uuid.Parse(workspace); err != nil {
		return entity.CycleCadence{}, fmt.Errorf("parse cadence workspace id: %w", err)
	}

	return cadence, nil
}

func (r *cadenceRepository) find(ctx context.Context, query string, teamID uuid.UUID) (entity.CycleCadence, error) {
	cadence, err := scanCadence(r.db.Querier(ctx).QueryRowContext(ctx, query, teamID.String()))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.CycleCadence{}, entity.ErrCycleCadenceNotFound
		}

		return entity.CycleCadence{}, fmt.Errorf("find cycle cadence: %w", err)
	}

	return cadence, nil
}

func (r *cadenceRepository) Get(ctx context.Context, teamID uuid.UUID) (entity.CycleCadence, error) {
	return r.find(ctx, cadenceByTeamQuery, teamID)
}

func (r *cadenceRepository) Lock(ctx context.Context, teamID uuid.UUID) (entity.CycleCadence, error) {
	return r.find(ctx, lockCadenceQuery, teamID)
}

func (r *cadenceRepository) Upsert(
	ctx context.Context,
	cadence entity.CycleCadence,
) (entity.CycleCadence, error) {
	stored, err := scanCadence(r.db.Querier(ctx).QueryRowContext(
		ctx,
		upsertCadenceQuery,
		cadence.TeamID.String(),
		cadence.WorkspaceID.String(),
		cadence.LengthWeeks,
		cadence.AnchorOn,
		time.Now().UTC(),
	))
	if err != nil {
		return entity.CycleCadence{}, fmt.Errorf("upsert cycle cadence: %w", err)
	}

	return stored, nil
}

func (r *cadenceRepository) Delete(ctx context.Context, teamID uuid.UUID) error {
	result, err := r.db.Querier(ctx).ExecContext(ctx, deleteCadenceQuery, teamID.String())
	if err != nil {
		return fmt.Errorf("delete cycle cadence: %w", err)
	}

	removed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read removed cadence rows: %w", err)
	}

	if removed == 0 {
		return entity.ErrCycleCadenceNotFound
	}

	return nil
}

func (r *cadenceRepository) ListByWorkspaceID(
	ctx context.Context,
	workspaceID uuid.UUID,
) ([]entity.CycleCadence, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, cadencesByWorkspaceQuery, workspaceID.String())
	if err != nil {
		return nil, fmt.Errorf("list cycle cadences: %w", err)
	}

	defer func() { _ = rows.Close() }()

	cadences := make([]entity.CycleCadence, 0)

	for rows.Next() {
		cadence, err := scanCadence(rows)
		if err != nil {
			return nil, fmt.Errorf("scan cycle cadence: %w", err)
		}

		cadences = append(cadences, cadence)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cycle cadences: %w", err)
	}

	return cadences, nil
}

func (r *cadenceRepository) ListAll(ctx context.Context) ([]repository.CadenceListing, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, allCadencesQuery)
	if err != nil {
		return nil, fmt.Errorf("list every cycle cadence: %w", err)
	}

	defer func() { _ = rows.Close() }()

	listings := make([]repository.CadenceListing, 0)

	for rows.Next() {
		var (
			listing   repository.CadenceListing
			team      string
			workspace string
		)

		if err := rows.Scan(
			&team,
			&workspace,
			&listing.Cadence.LengthWeeks,
			&listing.Cadence.AnchorOn,
			&listing.Cadence.CreatedAt,
			&listing.Cadence.UpdatedAt,
			&listing.Timezone,
		); err != nil {
			return nil, fmt.Errorf("scan cycle cadence listing: %w", err)
		}

		parsed, err := uuid.Parse(team)
		if err != nil {
			return nil, fmt.Errorf("parse cadence team id: %w", err)
		}

		listing.Cadence.TeamID = parsed

		if listing.Cadence.WorkspaceID, err = uuid.Parse(workspace); err != nil {
			return nil, fmt.Errorf("parse cadence workspace id: %w", err)
		}

		listings = append(listings, listing)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cycle cadence listings: %w", err)
	}

	return listings, nil
}
