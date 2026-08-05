package directory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const connectionColumns = `
SELECT workspace_id, token_hash, enabled, on_unknown, on_absent, admin_group,
       last_sync_at, created_at, updated_at
FROM workspace_directory_connections`

const saveConnectionQuery = `
INSERT INTO workspace_directory_connections (
    workspace_id, token_hash, enabled, on_unknown, on_absent, admin_group, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, now())
ON CONFLICT (workspace_id) DO UPDATE SET
    token_hash = excluded.token_hash,
    enabled = excluded.enabled,
    on_unknown = excluded.on_unknown,
    on_absent = excluded.on_absent,
    admin_group = excluded.admin_group,
    updated_at = now()`

const userColumns = `
SELECT id, workspace_id, account_id, external_id, user_name, display_name, active,
       created_at, updated_at
FROM directory_users`

const saveUserQuery = `
INSERT INTO directory_users (
    id, workspace_id, account_id, external_id, user_name, display_name, active, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, now())
ON CONFLICT (workspace_id, account_id) DO UPDATE SET
    external_id = excluded.external_id,
    user_name = excluded.user_name,
    display_name = excluded.display_name,
    active = excluded.active,
    updated_at = now()
RETURNING id`

const groupColumns = `
SELECT id, workspace_id, external_id, display_name, team_id, created_at, updated_at
FROM directory_groups`

const saveGroupQuery = `
INSERT INTO directory_groups (
    id, workspace_id, external_id, display_name, team_id, updated_at
)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (workspace_id, lower(display_name)) DO UPDATE SET
    external_id = excluded.external_id,
    display_name = excluded.display_name,
    team_id = excluded.team_id,
    updated_at = now()
RETURNING id`

type directoryRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.Directory {
	return &directoryRepository{db: db}
}

func NewSync(db *postgres.Client) repository.DirectorySync {
	return &directoryRepository{db: db}
}

func (r *directoryRepository) GetConnection(
	ctx context.Context,
	workspaceID uuid.UUID,
) (entity.DirectoryConnection, error) {
	return r.connection(ctx, connectionColumns+"\nWHERE workspace_id = $1", workspaceID.String())
}

func (r *directoryRepository) GetConnectionByToken(
	ctx context.Context,
	tokenHash []byte,
) (entity.DirectoryConnection, error) {
	return r.connection(ctx, connectionColumns+"\nWHERE token_hash = $1", tokenHash)
}

func (r *directoryRepository) connection(
	ctx context.Context,
	query string,
	arg any,
) (entity.DirectoryConnection, error) {
	var (
		connection entity.DirectoryConnection
		rawID      string
		lastSyncAt sql.NullTime
		onUnknown  string
		onAbsent   string
	)

	err := r.db.Querier(ctx).QueryRowContext(ctx, query, arg).Scan(
		&rawID,
		&connection.TokenHash,
		&connection.Enabled,
		&onUnknown,
		&onAbsent,
		&connection.AdminGroup,
		&lastSyncAt,
		&connection.CreatedAt,
		&connection.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.DirectoryConnection{}, entity.ErrDirectoryNotConnected
		}

		return entity.DirectoryConnection{}, fmt.Errorf("find directory connection: %w", err)
	}

	workspaceID, err := uuid.Parse(rawID)
	if err != nil {
		return entity.DirectoryConnection{}, fmt.Errorf("parse directory workspace id: %w", err)
	}

	connection.WorkspaceID = workspaceID
	connection.OnUnknown = entity.DirectoryUnknownPolicy(onUnknown)
	connection.OnAbsent = entity.DirectoryAbsentPolicy(onAbsent)

	if lastSyncAt.Valid {
		syncedAt := lastSyncAt.Time
		connection.LastSyncAt = &syncedAt
	}

	return connection, nil
}

func (r *directoryRepository) SaveConnection(
	ctx context.Context,
	connection entity.DirectoryConnection,
) (entity.DirectoryConnection, error) {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		saveConnectionQuery,
		connection.WorkspaceID.String(),
		connection.TokenHash,
		connection.Enabled,
		string(connection.OnUnknown),
		string(connection.OnAbsent),
		connection.AdminGroup,
	); err != nil {
		return entity.DirectoryConnection{}, fmt.Errorf("save directory connection: %w", err)
	}

	return r.GetConnection(ctx, connection.WorkspaceID)
}

func (r *directoryRepository) DeleteConnection(ctx context.Context, workspaceID uuid.UUID) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		"DELETE FROM workspace_directory_connections WHERE workspace_id = $1",
		workspaceID.String(),
	)
	if err != nil {
		return fmt.Errorf("delete directory connection: %w", err)
	}

	removed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete directory connection: %w", err)
	}

	if removed == 0 {
		return entity.ErrDirectoryNotConnected
	}

	return nil
}

func (r *directoryRepository) StampSync(ctx context.Context, workspaceID uuid.UUID) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		"UPDATE workspace_directory_connections SET last_sync_at = now() WHERE workspace_id = $1",
		workspaceID.String(),
	); err != nil {
		return fmt.Errorf("stamp directory sync: %w", err)
	}

	return nil
}

func (r *directoryRepository) SaveUser(
	ctx context.Context,
	user entity.DirectoryUser,
) (entity.DirectoryUser, error) {
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}

	var storedID string

	if err := r.db.Querier(ctx).QueryRowContext(
		ctx,
		saveUserQuery,
		user.ID.String(),
		user.WorkspaceID.String(),
		user.AccountID.String(),
		user.ExternalID,
		user.UserName,
		user.DisplayName,
		user.Active,
	).Scan(&storedID); err != nil {
		return entity.DirectoryUser{}, fmt.Errorf("save directory user: %w", err)
	}

	id, err := uuid.Parse(storedID)
	if err != nil {
		return entity.DirectoryUser{}, fmt.Errorf("parse directory user id: %w", err)
	}

	return r.GetUser(ctx, user.WorkspaceID, id)
}

func (r *directoryRepository) GetUser(
	ctx context.Context,
	workspaceID, id uuid.UUID,
) (entity.DirectoryUser, error) {
	return r.user(
		ctx,
		userColumns+"\nWHERE workspace_id = $1 AND id = $2",
		workspaceID.String(),
		id.String(),
	)
}

func (r *directoryRepository) GetUserByName(
	ctx context.Context,
	workspaceID uuid.UUID,
	userName string,
) (entity.DirectoryUser, error) {
	return r.user(
		ctx,
		userColumns+"\nWHERE workspace_id = $1 AND lower(user_name) = $2",
		workspaceID.String(),
		entity.NormalizeDirectoryUserName(userName),
	)
}

func (r *directoryRepository) GetUserByAccount(
	ctx context.Context,
	workspaceID, accountID uuid.UUID,
) (entity.DirectoryUser, error) {
	return r.user(
		ctx,
		userColumns+"\nWHERE workspace_id = $1 AND account_id = $2",
		workspaceID.String(),
		accountID.String(),
	)
}

func (r *directoryRepository) user(
	ctx context.Context,
	query string,
	args ...any,
) (entity.DirectoryUser, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return entity.DirectoryUser{}, fmt.Errorf("find directory user: %w", err)
	}

	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return entity.DirectoryUser{}, fmt.Errorf("find directory user: %w", err)
		}

		return entity.DirectoryUser{}, entity.ErrDirectoryUserNotFound
	}

	return scanUser(rows)
}

func (r *directoryRepository) ListUsers(
	ctx context.Context,
	workspaceID uuid.UUID,
	userName string,
	limit int,
) ([]entity.DirectoryUser, error) {
	query := userColumns + "\nWHERE workspace_id = $1"
	args := []any{workspaceID.String()}

	if trimmed := entity.NormalizeDirectoryUserName(userName); trimmed != "" {
		query += "\n  AND lower(user_name) = $2"
		args = append(args, trimmed)
	}

	query += fmt.Sprintf("\nORDER BY lower(user_name)\nLIMIT $%d", len(args)+1)
	args = append(args, limit)

	rows, err := r.db.Querier(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list directory users: %w", err)
	}

	defer func() { _ = rows.Close() }()

	users := make([]entity.DirectoryUser, 0, limit)

	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}

		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read directory users: %w", err)
	}

	return users, nil
}

func (r *directoryRepository) DeleteUser(ctx context.Context, workspaceID, id uuid.UUID) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		"DELETE FROM directory_users WHERE workspace_id = $1 AND id = $2",
		workspaceID.String(),
		id.String(),
	)
	if err != nil {
		return fmt.Errorf("delete directory user: %w", err)
	}

	removed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete directory user: %w", err)
	}

	if removed == 0 {
		return entity.ErrDirectoryUserNotFound
	}

	return nil
}

func (r *directoryRepository) SaveGroup(
	ctx context.Context,
	group entity.DirectoryGroup,
) (entity.DirectoryGroup, error) {
	if group.ID == uuid.Nil {
		group.ID = uuid.New()
	}

	var (
		storedID string
		teamID   any
	)

	if group.TeamID != nil && *group.TeamID != uuid.Nil {
		teamID = group.TeamID.String()
	}

	if err := r.db.Querier(ctx).QueryRowContext(
		ctx,
		saveGroupQuery,
		group.ID.String(),
		group.WorkspaceID.String(),
		group.ExternalID,
		group.DisplayName,
		teamID,
	).Scan(&storedID); err != nil {
		return entity.DirectoryGroup{}, fmt.Errorf("save directory group: %w", err)
	}

	id, err := uuid.Parse(storedID)
	if err != nil {
		return entity.DirectoryGroup{}, fmt.Errorf("parse directory group id: %w", err)
	}

	return r.GetGroup(ctx, group.WorkspaceID, id)
}

func (r *directoryRepository) GetGroup(
	ctx context.Context,
	workspaceID, id uuid.UUID,
) (entity.DirectoryGroup, error) {
	return r.group(
		ctx,
		groupColumns+"\nWHERE workspace_id = $1 AND id = $2",
		workspaceID.String(),
		id.String(),
	)
}

func (r *directoryRepository) GetGroupByName(
	ctx context.Context,
	workspaceID uuid.UUID,
	displayName string,
) (entity.DirectoryGroup, error) {
	return r.group(
		ctx,
		groupColumns+"\nWHERE workspace_id = $1 AND lower(display_name) = $2",
		workspaceID.String(),
		strings.ToLower(strings.TrimSpace(displayName)),
	)
}

func (r *directoryRepository) group(
	ctx context.Context,
	query string,
	args ...any,
) (entity.DirectoryGroup, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return entity.DirectoryGroup{}, fmt.Errorf("find directory group: %w", err)
	}

	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return entity.DirectoryGroup{}, fmt.Errorf("find directory group: %w", err)
		}

		return entity.DirectoryGroup{}, entity.ErrDirectoryGroupNotFound
	}

	return scanGroup(rows)
}

func (r *directoryRepository) ListGroups(
	ctx context.Context,
	workspaceID uuid.UUID,
	displayName string,
	limit int,
) ([]entity.DirectoryGroup, error) {
	query := groupColumns + "\nWHERE workspace_id = $1"
	args := []any{workspaceID.String()}

	if trimmed := strings.ToLower(strings.TrimSpace(displayName)); trimmed != "" {
		query += "\n  AND lower(display_name) = $2"
		args = append(args, trimmed)
	}

	query += fmt.Sprintf("\nORDER BY lower(display_name)\nLIMIT $%d", len(args)+1)
	args = append(args, limit)

	rows, err := r.db.Querier(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list directory groups: %w", err)
	}

	defer func() { _ = rows.Close() }()

	groups := make([]entity.DirectoryGroup, 0, limit)

	for rows.Next() {
		group, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}

		groups = append(groups, group)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read directory groups: %w", err)
	}

	return groups, nil
}

func (r *directoryRepository) DeleteGroup(ctx context.Context, workspaceID, id uuid.UUID) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		"DELETE FROM directory_groups WHERE workspace_id = $1 AND id = $2",
		workspaceID.String(),
		id.String(),
	)
	if err != nil {
		return fmt.Errorf("delete directory group: %w", err)
	}

	removed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete directory group: %w", err)
	}

	if removed == 0 {
		return entity.ErrDirectoryGroupNotFound
	}

	return nil
}

func (r *directoryRepository) AddGroupMember(ctx context.Context, groupID, directoryUserID uuid.UUID) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		`INSERT INTO directory_group_members (group_id, directory_user_id)
		 VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		groupID.String(),
		directoryUserID.String(),
	); err != nil {
		return fmt.Errorf("add directory group member: %w", err)
	}

	return nil
}

func (r *directoryRepository) RemoveGroupMember(ctx context.Context, groupID, directoryUserID uuid.UUID) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		"DELETE FROM directory_group_members WHERE group_id = $1 AND directory_user_id = $2",
		groupID.String(),
		directoryUserID.String(),
	); err != nil {
		return fmt.Errorf("remove directory group member: %w", err)
	}

	return nil
}

func (r *directoryRepository) ListGroupMembers(
	ctx context.Context,
	groupID uuid.UUID,
) ([]entity.DirectoryUser, error) {
	rows, err := r.db.Querier(ctx).QueryContext(
		ctx,
		userColumns+`
WHERE id IN (SELECT directory_user_id FROM directory_group_members WHERE group_id = $1)
ORDER BY lower(user_name)`,
		groupID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("list directory group members: %w", err)
	}

	defer func() { _ = rows.Close() }()

	members := make([]entity.DirectoryUser, 0)

	for rows.Next() {
		member, err := scanUser(rows)
		if err != nil {
			return nil, err
		}

		members = append(members, member)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read directory group members: %w", err)
	}

	return members, nil
}

func (r *directoryRepository) ListGroupsForUser(
	ctx context.Context,
	directoryUserID uuid.UUID,
) ([]entity.DirectoryGroup, error) {
	rows, err := r.db.Querier(ctx).QueryContext(
		ctx,
		groupColumns+`
WHERE id IN (SELECT group_id FROM directory_group_members WHERE directory_user_id = $1)
ORDER BY lower(display_name)`,
		directoryUserID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("list directory groups for user: %w", err)
	}

	defer func() { _ = rows.Close() }()

	groups := make([]entity.DirectoryGroup, 0)

	for rows.Next() {
		group, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}

		groups = append(groups, group)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read directory groups for user: %w", err)
	}

	return groups, nil
}

func (r *directoryRepository) RecordRun(
	ctx context.Context,
	run entity.DirectorySyncRun,
) (entity.DirectorySyncRun, error) {
	if run.ID == uuid.Nil {
		run.ID = uuid.New()
	}

	if run.StartedAt.IsZero() {
		run.StartedAt = time.Now().UTC()
	}

	if run.FinishedAt.IsZero() {
		run.FinishedAt = time.Now().UTC()
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		`INSERT INTO directory_sync_runs (
		     id, workspace_id, trigger, outcome, operation, started_at, finished_at, detail
		 ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		run.ID.String(),
		run.WorkspaceID.String(),
		string(run.Trigger),
		string(run.Outcome),
		run.Operation,
		run.StartedAt,
		run.FinishedAt,
		run.Detail,
	); err != nil {
		return entity.DirectorySyncRun{}, fmt.Errorf("record directory sync run: %w", err)
	}

	for i := range run.Changes {
		change := run.Changes[i]

		if change.ID == uuid.Nil {
			change.ID = uuid.New()
		}

		var detail any

		if len(change.Detail) > 0 {
			encoded, err := json.Marshal(change.Detail)
			if err != nil {
				return entity.DirectorySyncRun{}, fmt.Errorf("encode directory change detail: %w", err)
			}

			detail = encoded
		}

		if _, err := r.db.Querier(ctx).ExecContext(
			ctx,
			`INSERT INTO directory_sync_changes (id, run_id, subject, kind, outcome, detail)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			change.ID.String(),
			run.ID.String(),
			change.Subject,
			string(change.Kind),
			string(change.Outcome),
			detail,
		); err != nil {
			return entity.DirectorySyncRun{}, fmt.Errorf("record directory sync change: %w", err)
		}

		run.Changes[i] = change
	}

	return run, nil
}

func (r *directoryRepository) ListRuns(
	ctx context.Context,
	workspaceID uuid.UUID,
	page entity.DirectoryRunPage,
) ([]entity.DirectorySyncRun, error) {
	page = page.Normalized()

	query := `
SELECT id, workspace_id, trigger, outcome, operation, started_at, finished_at, detail
FROM directory_sync_runs
WHERE workspace_id = $1`
	args := []any{workspaceID.String()}

	if page.Cursor != nil {
		query += "\n  AND started_at < $2"
		args = append(args, *page.Cursor)
	}

	query += fmt.Sprintf("\nORDER BY started_at DESC, id DESC\nLIMIT $%d", len(args)+1)
	args = append(args, page.Limit)

	rows, err := r.db.Querier(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list directory sync runs: %w", err)
	}

	defer func() { _ = rows.Close() }()

	runs := make([]entity.DirectorySyncRun, 0, page.Limit)

	for rows.Next() {
		var (
			run              entity.DirectorySyncRun
			rawID, rawSpace  string
			trigger, outcome string
		)

		if err := rows.Scan(
			&rawID, &rawSpace, &trigger, &outcome,
			&run.Operation, &run.StartedAt, &run.FinishedAt, &run.Detail,
		); err != nil {
			return nil, fmt.Errorf("scan directory sync run: %w", err)
		}

		id, err := uuid.Parse(rawID)
		if err != nil {
			return nil, fmt.Errorf("parse directory sync run id: %w", err)
		}

		space, err := uuid.Parse(rawSpace)
		if err != nil {
			return nil, fmt.Errorf("parse directory sync run workspace id: %w", err)
		}

		run.ID = id
		run.WorkspaceID = space
		run.Trigger = entity.DirectorySyncTrigger(trigger)
		run.Outcome = entity.DirectorySyncOutcome(outcome)

		runs = append(runs, run)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read directory sync runs: %w", err)
	}

	return runs, nil
}

func (r *directoryRepository) ListChanges(
	ctx context.Context,
	runID uuid.UUID,
) ([]entity.DirectorySyncChange, error) {
	rows, err := r.db.Querier(ctx).QueryContext(
		ctx,
		`SELECT id, run_id, subject, kind, outcome, coalesce(detail::text, ''), recorded_at
		 FROM directory_sync_changes
		 WHERE run_id = $1
		 ORDER BY recorded_at, id`,
		runID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("list directory sync changes: %w", err)
	}

	defer func() { _ = rows.Close() }()

	changes := make([]entity.DirectorySyncChange, 0)

	for rows.Next() {
		var (
			change              entity.DirectorySyncChange
			rawID, rawRun       string
			kind, outcome, meta string
		)

		if err := rows.Scan(
			&rawID, &rawRun, &change.Subject, &kind, &outcome, &meta, &change.RecordedAt,
		); err != nil {
			return nil, fmt.Errorf("scan directory sync change: %w", err)
		}

		id, err := uuid.Parse(rawID)
		if err != nil {
			return nil, fmt.Errorf("parse directory sync change id: %w", err)
		}

		run, err := uuid.Parse(rawRun)
		if err != nil {
			return nil, fmt.Errorf("parse directory sync change run id: %w", err)
		}

		change.ID = id
		change.RunID = run
		change.Kind = entity.DirectoryChangeKind(kind)
		change.Outcome = entity.DirectorySyncOutcome(outcome)

		if meta != "" {
			if err := json.Unmarshal([]byte(meta), &change.Detail); err != nil {
				return nil, fmt.Errorf("decode directory change detail: %w", err)
			}
		}

		changes = append(changes, change)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read directory sync changes: %w", err)
	}

	return changes, nil
}

func scanUser(rows *sql.Rows) (entity.DirectoryUser, error) {
	var (
		user                        entity.DirectoryUser
		rawID, rawSpace, rawAccount string
	)

	if err := rows.Scan(
		&rawID, &rawSpace, &rawAccount,
		&user.ExternalID, &user.UserName, &user.DisplayName, &user.Active,
		&user.CreatedAt, &user.UpdatedAt,
	); err != nil {
		return entity.DirectoryUser{}, fmt.Errorf("scan directory user: %w", err)
	}

	for target, raw := range map[*uuid.UUID]string{
		&user.ID:          rawID,
		&user.WorkspaceID: rawSpace,
		&user.AccountID:   rawAccount,
	} {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			return entity.DirectoryUser{}, fmt.Errorf("parse directory user identifier: %w", err)
		}

		*target = parsed
	}

	return user, nil
}

func scanGroup(rows *sql.Rows) (entity.DirectoryGroup, error) {
	var (
		group           entity.DirectoryGroup
		rawID, rawSpace string
		rawTeam         sql.NullString
	)

	if err := rows.Scan(
		&rawID, &rawSpace, &group.ExternalID, &group.DisplayName, &rawTeam,
		&group.CreatedAt, &group.UpdatedAt,
	); err != nil {
		return entity.DirectoryGroup{}, fmt.Errorf("scan directory group: %w", err)
	}

	id, err := uuid.Parse(rawID)
	if err != nil {
		return entity.DirectoryGroup{}, fmt.Errorf("parse directory group id: %w", err)
	}

	space, err := uuid.Parse(rawSpace)
	if err != nil {
		return entity.DirectoryGroup{}, fmt.Errorf("parse directory group workspace id: %w", err)
	}

	group.ID = id
	group.WorkspaceID = space

	if rawTeam.Valid {
		teamID, err := uuid.Parse(rawTeam.String)
		if err != nil {
			return entity.DirectoryGroup{}, fmt.Errorf("parse directory group team id: %w", err)
		}

		group.TeamID = &teamID
	}

	return group, nil
}
