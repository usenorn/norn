package mcpconnection

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/aarondl/sqlboiler/v4/types"
	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const connectionColumns = `
	c.id, c.account_id, c.client_id, c.client_name, c.scopes,
	c.revoked_at, c.last_used_at, c.created_at, c.updated_at`

const insertConnectionQuery = `
	INSERT INTO mcp_connections (id, account_id, client_id, client_name, scopes)
	VALUES ($1, $2, $3, $4, $5)`

const insertGrantQuery = `
	INSERT INTO mcp_connection_grants (connection_id, workspace_id, all_teams)
	VALUES ($1, $2, $3)
	RETURNING id`

const insertGrantTeamQuery = `
	INSERT INTO mcp_connection_grant_teams (grant_id, team_id) VALUES ($1, $2)`

const grantsQuery = `
	SELECT g.connection_id, g.workspace_id, g.all_teams, gt.team_id
	FROM mcp_connection_grants g
	LEFT JOIN mcp_connection_grant_teams gt ON gt.grant_id = g.id
	WHERE g.connection_id = ANY($1::uuid[])
	ORDER BY g.workspace_id, gt.team_id`

const reachingWorkspaceQuery = `
	SELECT` + connectionColumns + `
	FROM mcp_connections c
	WHERE c.revoked_at IS NULL
	  AND EXISTS (
	      SELECT 1 FROM workspace_memberships m
	      WHERE m.workspace_id = $1 AND m.account_id = c.account_id
	  )
	  AND (
	      NOT EXISTS (SELECT 1 FROM mcp_connection_grants g WHERE g.connection_id = c.id)
	      OR EXISTS (
	          SELECT 1 FROM mcp_connection_grants g
	          WHERE g.connection_id = c.id AND g.workspace_id = $1
	      )
	  )
	ORDER BY c.created_at DESC`

type mcpConnectionRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.MCPConnection {
	return &mcpConnectionRepository{db: db}
}

func (r *mcpConnectionRepository) Create(
	ctx context.Context,
	connection entity.MCPConnection,
) (entity.MCPConnection, error) {
	if connection.ID == uuid.Nil {
		connection.ID = uuid.New()
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		insertConnectionQuery,
		connection.ID.String(),
		connection.AccountID.String(),
		connection.ClientID.String(),
		connection.ClientName,
		types.StringArray(connection.Scopes.Strings()),
	); err != nil {
		return entity.MCPConnection{}, fmt.Errorf("insert mcp connection: %w", err)
	}

	if err := r.insertGrants(ctx, connection.ID, connection.Grants); err != nil {
		return entity.MCPConnection{}, err
	}

	return r.GetByID(ctx, connection.ID)
}

func (r *mcpConnectionRepository) GetByID(
	ctx context.Context,
	connectionID uuid.UUID,
) (entity.MCPConnection, error) {
	return r.one(
		ctx,
		`SELECT`+connectionColumns+` FROM mcp_connections c WHERE c.id = $1`,
		connectionID.String(),
	)
}

func (r *mcpConnectionRepository) ListByAccount(
	ctx context.Context,
	accountID uuid.UUID,
) ([]entity.MCPConnection, error) {
	return r.many(
		ctx,
		`SELECT`+connectionColumns+`
		FROM mcp_connections c
		WHERE c.account_id = $1 AND c.revoked_at IS NULL
		ORDER BY c.created_at DESC`,
		accountID.String(),
	)
}

func (r *mcpConnectionRepository) ListReachingWorkspace(
	ctx context.Context,
	workspaceID uuid.UUID,
) ([]entity.MCPConnection, error) {
	return r.many(ctx, reachingWorkspaceQuery, workspaceID.String())
}

func (r *mcpConnectionRepository) Revoke(
	ctx context.Context,
	connectionID uuid.UUID,
	revokedAt time.Time,
) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		`UPDATE mcp_connections SET revoked_at = $2, updated_at = now()
		 WHERE id = $1 AND revoked_at IS NULL`,
		connectionID.String(),
		revokedAt,
	)
	if err != nil {
		return fmt.Errorf("revoke mcp connection: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke mcp connection: %w", err)
	}

	if affected == 0 {
		return entity.ErrMCPConnectionNotFound
	}

	return nil
}

func (r *mcpConnectionRepository) RecordUsage(
	ctx context.Context,
	connectionID uuid.UUID,
	usedAt time.Time,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		`UPDATE mcp_connections SET last_used_at = $2 WHERE id = $1`,
		connectionID.String(),
		usedAt,
	); err != nil {
		return fmt.Errorf("record mcp connection usage: %w", err)
	}

	return nil
}

func (r *mcpConnectionRepository) ReplaceGrants(
	ctx context.Context,
	connectionID uuid.UUID,
	grants entity.APITokenGrants,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		`DELETE FROM mcp_connection_grants WHERE connection_id = $1`,
		connectionID.String(),
	); err != nil {
		return fmt.Errorf("clear mcp connection grants: %w", err)
	}

	if err := r.insertGrants(ctx, connectionID, grants); err != nil {
		return err
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		`UPDATE mcp_connections SET updated_at = now() WHERE id = $1`,
		connectionID.String(),
	); err != nil {
		return fmt.Errorf("stamp mcp connection: %w", err)
	}

	return nil
}

func (r *mcpConnectionRepository) SetScopes(
	ctx context.Context,
	connectionID uuid.UUID,
	scopes entity.APIScopeSet,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		`UPDATE mcp_connections SET scopes = $2, updated_at = now() WHERE id = $1`,
		connectionID.String(),
		types.StringArray(scopes.Strings()),
	); err != nil {
		return fmt.Errorf("set mcp connection scopes: %w", err)
	}

	return nil
}

func (r *mcpConnectionRepository) insertGrants(
	ctx context.Context,
	connectionID uuid.UUID,
	grants entity.APITokenGrants,
) error {
	executor := r.db.Querier(ctx)

	for _, grant := range grants {
		var grantID string

		if err := executor.QueryRowContext(
			ctx,
			insertGrantQuery,
			connectionID.String(),
			grant.WorkspaceID.String(),
			grant.AllTeams,
		).Scan(&grantID); err != nil {
			return fmt.Errorf("insert mcp connection grant: %w", err)
		}

		for _, teamID := range grant.TeamIDs {
			if _, err := executor.ExecContext(
				ctx, insertGrantTeamQuery, grantID, teamID.String(),
			); err != nil {
				return fmt.Errorf("insert mcp connection grant team: %w", err)
			}
		}
	}

	return nil
}

func (r *mcpConnectionRepository) one(
	ctx context.Context,
	query string,
	args ...any,
) (entity.MCPConnection, error) {
	connections, err := r.many(ctx, query, args...)
	if err != nil {
		return entity.MCPConnection{}, err
	}

	if len(connections) == 0 {
		return entity.MCPConnection{}, entity.ErrMCPConnectionNotFound
	}

	return connections[0], nil
}

func (r *mcpConnectionRepository) many(
	ctx context.Context,
	query string,
	args ...any,
) ([]entity.MCPConnection, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query mcp connections: %w", err)
	}

	defer func() { _ = rows.Close() }()

	connections := make([]entity.MCPConnection, 0)
	index := make(map[uuid.UUID]int)
	ids := make([]string, 0)

	for rows.Next() {
		connection, err := scanConnection(rows)
		if err != nil {
			return nil, err
		}

		index[connection.ID] = len(connections)
		ids = append(ids, connection.ID.String())
		connections = append(connections, connection)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read mcp connections: %w", err)
	}

	if len(connections) == 0 {
		return connections, nil
	}

	if err := r.attachGrants(ctx, connections, index, ids); err != nil {
		return nil, err
	}

	return connections, nil
}

func (r *mcpConnectionRepository) attachGrants(
	ctx context.Context,
	connections []entity.MCPConnection,
	index map[uuid.UUID]int,
	ids []string,
) error {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, grantsQuery, types.StringArray(ids))
	if err != nil {
		return fmt.Errorf("query mcp connection grants: %w", err)
	}

	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			rawConnection, rawWorkspace string
			allTeams                    bool
			rawTeam                     sql.NullString
		)

		if err := rows.Scan(&rawConnection, &rawWorkspace, &allTeams, &rawTeam); err != nil {
			return fmt.Errorf("scan mcp connection grant: %w", err)
		}

		connectionID, err := uuid.Parse(rawConnection)
		if err != nil {
			return fmt.Errorf("parse mcp connection grant connection id: %w", err)
		}

		workspaceID, err := uuid.Parse(rawWorkspace)
		if err != nil {
			return fmt.Errorf("parse mcp connection grant workspace id: %w", err)
		}

		position, ok := index[connectionID]
		if !ok {
			continue
		}

		connection := &connections[position]

		if !connection.Grants.Covers(workspaceID) {
			connection.Grants = append(connection.Grants, entity.APITokenGrant{
				WorkspaceID: workspaceID,
				AllTeams:    allTeams,
			})
		}

		if !rawTeam.Valid {
			continue
		}

		teamID, err := uuid.Parse(rawTeam.String)
		if err != nil {
			return fmt.Errorf("parse mcp connection grant team id: %w", err)
		}

		for position := range connection.Grants {
			if connection.Grants[position].WorkspaceID == workspaceID {
				connection.Grants[position].TeamIDs = append(connection.Grants[position].TeamIDs, teamID)
			}
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("read mcp connection grants: %w", err)
	}

	return nil
}

func scanConnection(rows *sql.Rows) (entity.MCPConnection, error) {
	var (
		rawID, rawAccount, rawClient string
		clientName                   string
		scopes                       types.StringArray
		revokedAt, lastUsedAt        sql.NullTime
		createdAt, updatedAt         time.Time
	)

	if err := rows.Scan(
		&rawID, &rawAccount, &rawClient, &clientName, &scopes,
		&revokedAt, &lastUsedAt, &createdAt, &updatedAt,
	); err != nil {
		return entity.MCPConnection{}, fmt.Errorf("scan mcp connection: %w", err)
	}

	id, err := uuid.Parse(rawID)
	if err != nil {
		return entity.MCPConnection{}, fmt.Errorf("parse mcp connection id: %w", err)
	}

	accountID, err := uuid.Parse(rawAccount)
	if err != nil {
		return entity.MCPConnection{}, fmt.Errorf("parse mcp connection account id: %w", err)
	}

	clientID, err := uuid.Parse(rawClient)
	if err != nil {
		return entity.MCPConnection{}, fmt.Errorf("parse mcp connection client id: %w", err)
	}

	connection := entity.MCPConnection{
		ID:         id,
		AccountID:  accountID,
		ClientID:   clientID,
		ClientName: clientName,
		Scopes:     entity.NewAPIScopeSet(scopes),
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}

	if revokedAt.Valid {
		connection.RevokedAt = &revokedAt.Time
	}

	if lastUsedAt.Valid {
		connection.LastUsedAt = &lastUsedAt.Time
	}

	return connection, nil
}
