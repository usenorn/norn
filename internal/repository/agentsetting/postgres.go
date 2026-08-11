package agentsetting

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

const settingsColumns = `team_id, workspace_id, hold_comments, hold_state_changes, hold_issue_edits`

const selectSettingsQuery = `
	SELECT ` + settingsColumns + `
	FROM workspace_team_agent_settings
	WHERE workspace_id = $1 AND team_id = $2`

const upsertSettingsQuery = `
	INSERT INTO workspace_team_agent_settings (
	    team_id, workspace_id, hold_comments, hold_state_changes, hold_issue_edits
	)
	VALUES ($1, $2, $3, $4, $5)
	ON CONFLICT (team_id) DO UPDATE SET
	    hold_comments = excluded.hold_comments,
	    hold_state_changes = excluded.hold_state_changes,
	    hold_issue_edits = excluded.hold_issue_edits,
	    updated_at = now()
	RETURNING ` + settingsColumns

type agentSettingRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.AgentSetting {
	return &agentSettingRepository{db: db}
}

func (r *agentSettingRepository) Settings(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
) (entity.AgentSettings, error) {
	settings, err := scan(r.db.Querier(ctx).QueryRowContext(
		ctx, selectSettingsQuery, workspaceID.String(), teamID.String(),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.AgentSettings{
				WorkspaceID: workspaceID,
				TeamID:      teamID,
			}.Normalised(), nil
		}

		return entity.AgentSettings{}, fmt.Errorf("read agent settings: %w", err)
	}

	return settings, nil
}

func (r *agentSettingRepository) Upsert(
	ctx context.Context,
	settings entity.AgentSettings,
) (entity.AgentSettings, error) {
	saved, err := scan(r.db.Querier(ctx).QueryRowContext(
		ctx,
		upsertSettingsQuery,
		settings.TeamID.String(),
		settings.WorkspaceID.String(),
		string(settings.HoldComments),
		string(settings.HoldStateChanges),
		string(settings.HoldIssueEdits),
	))
	if err != nil {
		return entity.AgentSettings{}, fmt.Errorf("save agent settings: %w", err)
	}

	return saved, nil
}

func scan(row *sql.Row) (entity.AgentSettings, error) {
	var (
		rawTeam, rawWorkspace         string
		comments, stateChanges, edits string
		settings                      entity.AgentSettings
	)

	if err := row.Scan(
		&rawTeam, &rawWorkspace,
		&comments, &stateChanges, &edits,
	); err != nil {
		return entity.AgentSettings{}, err
	}

	settings.HoldComments = entity.AgentHold(comments)
	settings.HoldStateChanges = entity.AgentHold(stateChanges)
	settings.HoldIssueEdits = entity.AgentHold(edits)

	teamID, err := uuid.Parse(rawTeam)
	if err != nil {
		return entity.AgentSettings{}, fmt.Errorf("parse agent settings team id: %w", err)
	}

	workspaceID, err := uuid.Parse(rawWorkspace)
	if err != nil {
		return entity.AgentSettings{}, fmt.Errorf("parse agent settings workspace id: %w", err)
	}

	settings.TeamID = teamID
	settings.WorkspaceID = workspaceID

	return settings.Normalised(), nil
}
