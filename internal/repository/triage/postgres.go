package triage

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

type triageRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.Triage {
	return &triageRepository{db: db}
}

const settingsColumns = `
       team_id,
       workspace_id,
       route_agents,
       route_integrations,
       route_non_members`

const settingsByTeamQuery = `
SELECT` + settingsColumns + `
FROM workspace_team_triage_settings
WHERE team_id = $1 AND workspace_id = $2`

const upsertSettingsQuery = `
INSERT INTO workspace_team_triage_settings
    (team_id, workspace_id, route_agents, route_integrations, route_non_members)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (team_id) DO UPDATE
    SET route_agents       = excluded.route_agents,
        route_integrations = excluded.route_integrations,
        route_non_members  = excluded.route_non_members,
        updated_at         = $6
RETURNING` + settingsColumns

const disableSettingsQuery = `
DELETE FROM workspace_team_triage_settings
WHERE team_id = $1 AND workspace_id = $2`

const decideQuery = `
UPDATE workspace_issues
SET triage_state = $2,
    triage_decided_by_account_id = nullif($3, '')::uuid,
    triage_decided_at = $4,
    updated_at = $4
WHERE id = $1 AND triage_state = 'waiting'`

type scanner interface {
	Scan(dest ...any) error
}

func scanSettings(row scanner) (entity.TriageSettings, error) {
	var (
		settings        entity.TriageSettings
		team, workspace string
	)

	if err := row.Scan(
		&team, &workspace,
		&settings.RouteAgents, &settings.RouteIntegrations, &settings.RouteNonMembers,
	); err != nil {
		return entity.TriageSettings{}, err
	}

	parsed, err := uuid.Parse(team)
	if err != nil {
		return entity.TriageSettings{}, fmt.Errorf("parse triage team id: %w", err)
	}

	settings.TeamID = parsed

	if settings.WorkspaceID, err = uuid.Parse(workspace); err != nil {
		return entity.TriageSettings{}, fmt.Errorf("parse triage workspace id: %w", err)
	}

	return settings, nil
}

func (r *triageRepository) Settings(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
) (entity.TriageSettings, error) {
	settings, err := scanSettings(r.db.Querier(ctx).QueryRowContext(
		ctx, settingsByTeamQuery, teamID.String(), workspaceID.String(),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.TriageSettings{}, entity.ErrTriageDisabled
		}

		return entity.TriageSettings{}, fmt.Errorf("find triage settings: %w", err)
	}

	return settings, nil
}

func (r *triageRepository) Upsert(
	ctx context.Context,
	settings entity.TriageSettings,
) (entity.TriageSettings, error) {
	saved, err := scanSettings(r.db.Querier(ctx).QueryRowContext(
		ctx, upsertSettingsQuery,
		settings.TeamID.String(), settings.WorkspaceID.String(),
		settings.RouteAgents, settings.RouteIntegrations, settings.RouteNonMembers,
		time.Now().UTC(),
	))
	if err != nil {
		return entity.TriageSettings{}, fmt.Errorf("save triage settings: %w", err)
	}

	return saved, nil
}

func (r *triageRepository) Disable(ctx context.Context, workspaceID, teamID uuid.UUID) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, disableSettingsQuery, teamID.String(), workspaceID.String(),
	); err != nil {
		return fmt.Errorf("disable triage: %w", err)
	}

	return nil
}

func (r *triageRepository) Decide(
	ctx context.Context,
	issueID uuid.UUID,
	state entity.TriageState,
	decidedBy uuid.UUID,
	at time.Time,
) error {
	decider := ""
	if decidedBy != uuid.Nil {
		decider = decidedBy.String()
	}

	result, err := r.db.Querier(ctx).ExecContext(
		ctx, decideQuery, issueID.String(), string(state), decider, at,
	)
	if err != nil {
		return fmt.Errorf("record triage decision: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("record triage decision: %w", err)
	}

	if affected == 0 {
		return entity.ErrIssueNotWaiting
	}

	return nil
}
