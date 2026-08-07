package scm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

type settingRepository struct {
	db *postgres.Client
}

func NewSCMTeamSetting(db *postgres.Client) repository.SCMTeamSetting {
	return &settingRepository{db: db}
}

const settingColumns = `
    team_id, workspace_id, advance_on_merge,
    coalesce(merged_state_id, '00000000-0000-0000-0000-000000000000'::uuid),
    created_at, updated_at`

func scanSetting(row interface{ Scan(...any) error }) (entity.SCMTeamSettings, error) {
	var settings entity.SCMTeamSettings

	err := row.Scan(
		&settings.TeamID,
		&settings.WorkspaceID,
		&settings.AdvanceOnMerge,
		&settings.MergedStateID,
		&settings.CreatedAt,
		&settings.UpdatedAt,
	)
	if err != nil {
		return entity.SCMTeamSettings{}, err
	}

	return settings, nil
}

const settingsQuery = `
SELECT` + settingColumns + `
FROM workspace_team_scm_settings
WHERE workspace_id = $1 AND team_id = $2`

// Settings reports a missing row as a sentinel rather than a zero value, so a caller has to
// decide what an unconfigured team means instead of silently reading it as "off" the way a
// zero-valued struct would.
func (r *settingRepository) Settings(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
) (entity.SCMTeamSettings, error) {
	settings, err := scanSetting(
		r.db.Querier(ctx).QueryRowContext(ctx, settingsQuery, workspaceID, teamID),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.SCMTeamSettings{}, entity.ErrSCMTeamSettingsNotFound
	}

	if err != nil {
		return entity.SCMTeamSettings{}, fmt.Errorf("read team source control settings: %w", err)
	}

	return settings, nil
}

const upsertSettingQuery = `
INSERT INTO workspace_team_scm_settings (team_id, workspace_id, advance_on_merge, merged_state_id)
VALUES ($1, $2, $3, $4)
ON CONFLICT (team_id) DO UPDATE
SET advance_on_merge = excluded.advance_on_merge,
    merged_state_id = excluded.merged_state_id,
    updated_at = now()
RETURNING` + settingColumns

func (r *settingRepository) Upsert(
	ctx context.Context,
	settings entity.SCMTeamSettings,
) (entity.SCMTeamSettings, error) {
	stored, err := scanSetting(r.db.Querier(ctx).QueryRowContext(
		ctx,
		upsertSettingQuery,
		settings.TeamID,
		settings.WorkspaceID,
		settings.AdvanceOnMerge,
		stateOrNil(settings.MergedStateID),
	))
	if err != nil {
		return entity.SCMTeamSettings{}, fmt.Errorf("save team source control settings: %w", err)
	}

	return stored, nil
}

func stateOrNil(stateID uuid.UUID) any {
	if stateID == uuid.Nil {
		return nil
	}

	return stateID
}

const disableSettingQuery = `
DELETE FROM workspace_team_scm_settings WHERE workspace_id = $1 AND team_id = $2`

func (r *settingRepository) Disable(ctx context.Context, workspaceID, teamID uuid.UUID) error {
	result, err := r.db.Querier(ctx).
		ExecContext(ctx, disableSettingQuery, workspaceID, teamID)
	if err != nil {
		return fmt.Errorf("clear team source control settings: %w", err)
	}

	return expectOne(result, entity.ErrSCMTeamSettingsNotFound)
}
