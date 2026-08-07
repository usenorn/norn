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

type teamSettingRepository struct {
	db *postgres.Client
}

func NewSCMTeamSetting(db *postgres.Client) repository.SCMTeamSetting {
	return &teamSettingRepository{db: db}
}

const teamSettingColumns = `
    team_id, workspace_id, branch_template, created_at, updated_at`

func scanTeamSettings(row interface{ Scan(...any) error }) (entity.SCMTeamSettings, error) {
	var settings entity.SCMTeamSettings

	err := row.Scan(
		&settings.TeamID,
		&settings.WorkspaceID,
		&settings.BranchTemplate,
		&settings.CreatedAt,
		&settings.UpdatedAt,
	)
	if err != nil {
		return entity.SCMTeamSettings{}, err
	}

	return settings, nil
}

const getTeamSettingsQuery = `
SELECT` + teamSettingColumns + `
FROM workspace_team_scm_settings
WHERE workspace_id = $1 AND team_id = $2`

func (r *teamSettingRepository) Get(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
) (entity.SCMTeamSettings, error) {
	settings, err := scanTeamSettings(
		r.db.Querier(ctx).QueryRowContext(ctx, getTeamSettingsQuery, workspaceID, teamID),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.SCMTeamSettings{WorkspaceID: workspaceID, TeamID: teamID}, nil
	}

	if err != nil {
		return entity.SCMTeamSettings{}, fmt.Errorf("read team source control settings: %w", err)
	}

	return settings, nil
}

const upsertTeamSettingsQuery = `
INSERT INTO workspace_team_scm_settings (team_id, workspace_id, branch_template)
VALUES ($1, $2, $3)
ON CONFLICT (team_id) DO UPDATE
SET branch_template = EXCLUDED.branch_template, updated_at = now()
RETURNING` + teamSettingColumns

func (r *teamSettingRepository) Upsert(
	ctx context.Context,
	settings entity.SCMTeamSettings,
) (entity.SCMTeamSettings, error) {
	stored, err := scanTeamSettings(r.db.Querier(ctx).QueryRowContext(
		ctx,
		upsertTeamSettingsQuery,
		settings.TeamID,
		settings.WorkspaceID,
		settings.BranchTemplate,
	))
	if err != nil {
		return entity.SCMTeamSettings{}, fmt.Errorf("save team source control settings: %w", err)
	}

	return stored, nil
}
