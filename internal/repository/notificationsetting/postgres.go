package notificationsetting

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const preferenceColumns = `inbox_assigned, inbox_mentioned, inbox_commented,
       inbox_state_changed, inbox_membership, inbox_checks, inbox_approvals, inbox_agents,
       email_assigned, email_mentioned, email_commented,
       email_state_changed, email_membership, email_checks, email_approvals, email_agents`

const listQuery = `
SELECT '' AS team_id, ` + preferenceColumns + `
FROM workspace_notification_settings
WHERE workspace_id = $1 AND account_id = $2
UNION ALL
SELECT team_id::text, ` + preferenceColumns + `
FROM workspace_team_notification_settings
WHERE workspace_id = $1 AND account_id = $2`

const listForQuery = `
SELECT account_id::text, '' AS team_id, ` + preferenceColumns + `
FROM workspace_notification_settings
WHERE workspace_id = $1 AND account_id = ANY($3::uuid[])
UNION ALL
SELECT account_id::text, team_id::text, ` + preferenceColumns + `
FROM workspace_team_notification_settings
WHERE workspace_id = $1 AND team_id = $2::uuid AND account_id = ANY($3::uuid[])`

const saveGlobalQuery = `
INSERT INTO workspace_notification_settings (
    workspace_id, account_id,
    inbox_assigned, inbox_mentioned, inbox_commented,
    inbox_state_changed, inbox_membership, inbox_checks, inbox_approvals, inbox_agents,
    email_assigned, email_mentioned, email_commented,
    email_state_changed, email_membership, email_checks, email_approvals, email_agents
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
ON CONFLICT (workspace_id, account_id) DO UPDATE SET
    inbox_assigned = excluded.inbox_assigned,
    inbox_mentioned = excluded.inbox_mentioned,
    inbox_commented = excluded.inbox_commented,
    inbox_state_changed = excluded.inbox_state_changed,
    inbox_membership = excluded.inbox_membership,
    inbox_checks = excluded.inbox_checks,
    inbox_approvals = excluded.inbox_approvals,
    inbox_agents = excluded.inbox_agents,
    email_assigned = excluded.email_assigned,
    email_mentioned = excluded.email_mentioned,
    email_commented = excluded.email_commented,
    email_state_changed = excluded.email_state_changed,
    email_membership = excluded.email_membership,
    email_checks = excluded.email_checks,
    email_approvals = excluded.email_approvals,
    email_agents = excluded.email_agents,
    updated_at = now()`

const saveTeamQuery = `
INSERT INTO workspace_team_notification_settings (
    workspace_id, account_id, team_id,
    inbox_assigned, inbox_mentioned, inbox_commented,
    inbox_state_changed, inbox_membership, inbox_checks, inbox_approvals, inbox_agents,
    email_assigned, email_mentioned, email_commented,
    email_state_changed, email_membership, email_checks, email_approvals, email_agents
)
VALUES ($1, $2, $19, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
ON CONFLICT (workspace_id, account_id, team_id) DO UPDATE SET
    inbox_assigned = excluded.inbox_assigned,
    inbox_mentioned = excluded.inbox_mentioned,
    inbox_commented = excluded.inbox_commented,
    inbox_state_changed = excluded.inbox_state_changed,
    inbox_membership = excluded.inbox_membership,
    inbox_checks = excluded.inbox_checks,
    inbox_approvals = excluded.inbox_approvals,
    inbox_agents = excluded.inbox_agents,
    email_assigned = excluded.email_assigned,
    email_mentioned = excluded.email_mentioned,
    email_commented = excluded.email_commented,
    email_state_changed = excluded.email_state_changed,
    email_membership = excluded.email_membership,
    email_checks = excluded.email_checks,
    email_approvals = excluded.email_approvals,
    email_agents = excluded.email_agents,
    updated_at = now()`

const clearTeamQuery = `
DELETE FROM workspace_team_notification_settings
WHERE workspace_id = $1 AND account_id = $2 AND team_id = $3`

const clearGlobalQuery = `
DELETE FROM workspace_notification_settings
WHERE workspace_id = $1 AND account_id = $2`

type settingRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.NotificationSetting {
	return &settingRepository{db: db}
}

func identifiers(ids []uuid.UUID) []string {
	raw := make([]string, 0, len(ids))
	for _, id := range ids {
		raw = append(raw, id.String())
	}

	return raw
}

func (r *settingRepository) List(
	ctx context.Context,
	workspaceID, accountID uuid.UUID,
) ([]entity.NotificationSettings, error) {
	rows, err := r.db.Querier(ctx).QueryContext(
		ctx, listQuery, workspaceID.String(), accountID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("read notification settings: %w", err)
	}

	defer func() { _ = rows.Close() }()

	settings := make([]entity.NotificationSettings, 0)

	for rows.Next() {
		var (
			setting entity.NotificationSettings
			team    string
		)

		if err := rows.Scan(append([]any{&team}, targets(&setting.Preferences)...)...); err != nil {
			return nil, fmt.Errorf("scan notification settings: %w", err)
		}

		setting.WorkspaceID = workspaceID
		setting.AccountID = accountID
		setting.TeamID = parse(team)

		settings = append(settings, setting)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read notification settings: %w", err)
	}

	return settings, nil
}

func (r *settingRepository) ListFor(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
	accountIDs []uuid.UUID,
) ([]entity.NotificationSettings, error) {
	rows, err := r.db.Querier(ctx).QueryContext(
		ctx, listForQuery, workspaceID.String(), teamID.String(), identifiers(accountIDs),
	)
	if err != nil {
		return nil, fmt.Errorf("read notification settings: %w", err)
	}

	defer func() { _ = rows.Close() }()

	settings := make([]entity.NotificationSettings, 0, len(accountIDs))

	for rows.Next() {
		var (
			setting       entity.NotificationSettings
			account, team string
		)

		if err := rows.Scan(
			append([]any{&account, &team}, targets(&setting.Preferences)...)...,
		); err != nil {
			return nil, fmt.Errorf("scan notification settings: %w", err)
		}

		setting.WorkspaceID = workspaceID
		setting.AccountID = parse(account)
		setting.TeamID = parse(team)

		settings = append(settings, setting)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read notification settings: %w", err)
	}

	return settings, nil
}

func (r *settingRepository) Save(ctx context.Context, settings entity.NotificationSettings) error {
	query := saveGlobalQuery
	args := append(
		[]any{settings.WorkspaceID.String(), settings.AccountID.String()},
		values(settings.Preferences)...,
	)

	if settings.TeamID != uuid.Nil {
		query = saveTeamQuery
		args = append(args, settings.TeamID.String())
	}

	if _, err := r.db.Querier(ctx).ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("save notification settings: %w", err)
	}

	return nil
}

func (r *settingRepository) Clear(ctx context.Context, workspaceID, accountID, teamID uuid.UUID) error {
	query := clearGlobalQuery
	args := []any{workspaceID.String(), accountID.String()}

	if teamID != uuid.Nil {
		query = clearTeamQuery
		args = append(args, teamID.String())
	}

	if _, err := r.db.Querier(ctx).ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("clear notification settings: %w", err)
	}

	return nil
}

func parse(raw string) uuid.UUID {
	if raw == "" {
		return uuid.Nil
	}

	parsed, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil
	}

	return parsed
}

func targets(preferences *entity.NotificationPreferences) []any {
	return []any{
		&preferences.Assigned.Inbox,
		&preferences.Mentioned.Inbox,
		&preferences.Commented.Inbox,
		&preferences.StateChanged.Inbox,
		&preferences.Membership.Inbox,
		&preferences.Checks.Inbox,
		&preferences.Approvals.Inbox,
		&preferences.Agents.Inbox,
		&preferences.Assigned.Email,
		&preferences.Mentioned.Email,
		&preferences.Commented.Email,
		&preferences.StateChanged.Email,
		&preferences.Membership.Email,
		&preferences.Checks.Email,
		&preferences.Approvals.Email,
		&preferences.Agents.Email,
	}
}

func values(preferences entity.NotificationPreferences) []any {
	return []any{
		preferences.Assigned.Inbox,
		preferences.Mentioned.Inbox,
		preferences.Commented.Inbox,
		preferences.StateChanged.Inbox,
		preferences.Membership.Inbox,
		preferences.Checks.Inbox,
		preferences.Approvals.Inbox,
		preferences.Agents.Inbox,
		preferences.Assigned.Email,
		preferences.Mentioned.Email,
		preferences.Commented.Email,
		preferences.StateChanged.Email,
		preferences.Membership.Email,
		preferences.Checks.Email,
		preferences.Approvals.Email,
		preferences.Agents.Email,
	}
}
