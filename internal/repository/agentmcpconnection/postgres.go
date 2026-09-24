package agentmcpconnection

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/types"
	"github.com/google/uuid"

	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/crypter"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

type connectionRepository struct {
	db      *postgres.Client
	crypter *crypter.Crypter
}

func New(db *postgres.Client, sealer *crypter.Crypter) repository.AgentMCPConnection {
	return &connectionRepository{db: db, crypter: sealer}
}

func toEntity(model *dbpostgres.WorkspaceAgentMCPConnection) (entity.AgentMCPConnection, error) {
	serverID, err := uuid.Parse(model.ServerID)
	if err != nil {
		return entity.AgentMCPConnection{}, fmt.Errorf("parse connection server id: %w", err)
	}

	workspaceID, err := uuid.Parse(model.WorkspaceID)
	if err != nil {
		return entity.AgentMCPConnection{}, fmt.Errorf("parse connection workspace id: %w", err)
	}

	connectedBy, err := uuid.Parse(model.ConnectedByAccountID)
	if err != nil {
		return entity.AgentMCPConnection{}, fmt.Errorf("parse connection account id: %w", err)
	}

	return entity.AgentMCPConnection{
		ServerID:      serverID,
		WorkspaceID:   workspaceID,
		Status:        entity.AgentMCPConnectionStatus(model.Status),
		Issuer:        model.Issuer,
		TokenEndpoint: model.TokenEndpoint,
		ClientID:      model.ClientID,
		Scopes:        []string(model.Scopes),
		ExpiresAt:     model.ExpiresAt.Ptr(),
		Failure:       entity.AgentMCPConnectionFailure(model.Failure),
		ConnectedBy:   connectedBy,
		ConnectedAt:   model.ConnectedAt,
		UpdatedAt:     model.UpdatedAt,
	}, nil
}

func (r *connectionRepository) seal(value string) (null.Bytes, error) {
	if value == "" {
		return null.Bytes{}, nil
	}

	sealed, err := r.crypter.Seal([]byte(value))
	if err != nil {
		if errors.Is(err, crypter.ErrKeyMissing) {
			return null.Bytes{}, entity.ErrAgentCapabilityEncryptionKeyMissing
		}

		return null.Bytes{}, fmt.Errorf("seal mcp credential: %w", err)
	}

	return null.BytesFrom(sealed), nil
}

func (r *connectionRepository) open(sealed null.Bytes) (string, error) {
	if !sealed.Valid {
		return "", nil
	}

	value, err := r.crypter.Open(sealed.Bytes)
	if err != nil {
		if errors.Is(err, crypter.ErrKeyMissing) {
			return "", entity.ErrAgentCapabilityEncryptionKeyMissing
		}

		return "", fmt.Errorf("open mcp credential: %w", err)
	}

	return string(value), nil
}

func (r *connectionRepository) find(
	ctx context.Context,
	workspaceID, serverID uuid.UUID,
) (*dbpostgres.WorkspaceAgentMCPConnection, error) {
	model, err := dbpostgres.WorkspaceAgentMCPConnections(
		dbpostgres.WorkspaceAgentMCPConnectionWhere.WorkspaceID.EQ(workspaceID.String()),
		dbpostgres.WorkspaceAgentMCPConnectionWhere.ServerID.EQ(serverID.String()),
	).One(ctx, r.db.Querier(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, entity.ErrAgentMCPConnectionNotFound
		}

		return nil, fmt.Errorf("read mcp connection: %w", err)
	}

	return model, nil
}

func (r *connectionRepository) Get(
	ctx context.Context,
	workspaceID, serverID uuid.UUID,
) (entity.AgentMCPConnection, error) {
	model, err := r.find(ctx, workspaceID, serverID)
	if err != nil {
		return entity.AgentMCPConnection{}, err
	}

	return toEntity(model)
}

func (r *connectionRepository) List(
	ctx context.Context,
	workspaceID uuid.UUID,
	serverIDs []uuid.UUID,
) ([]entity.AgentMCPConnection, error) {
	if len(serverIDs) == 0 {
		return nil, nil
	}

	ids := make([]string, len(serverIDs))
	for i, id := range serverIDs {
		ids[i] = id.String()
	}

	models, err := dbpostgres.WorkspaceAgentMCPConnections(
		dbpostgres.WorkspaceAgentMCPConnectionWhere.WorkspaceID.EQ(workspaceID.String()),
		dbpostgres.WorkspaceAgentMCPConnectionWhere.ServerID.IN(ids),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list mcp connections: %w", err)
	}

	connections := make([]entity.AgentMCPConnection, 0, len(models))

	for _, model := range models {
		connection, err := toEntity(model)
		if err != nil {
			return nil, err
		}

		connections = append(connections, connection)
	}

	return connections, nil
}

func (r *connectionRepository) Tokens(
	ctx context.Context,
	workspaceID, serverID uuid.UUID,
) (entity.AgentMCPTokens, error) {
	model, err := r.find(ctx, workspaceID, serverID)
	if err != nil {
		return entity.AgentMCPTokens{}, err
	}

	clientSecret, err := r.open(model.ClientSecretSealed)
	if err != nil {
		return entity.AgentMCPTokens{}, err
	}

	accessToken, err := r.open(null.BytesFrom(model.AccessTokenSealed))
	if err != nil {
		return entity.AgentMCPTokens{}, err
	}

	refreshToken, err := r.open(model.RefreshTokenSealed)
	if err != nil {
		return entity.AgentMCPTokens{}, err
	}

	return entity.AgentMCPTokens{
		ClientSecret: clientSecret,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    model.ExpiresAt.Ptr(),
	}, nil
}

func (r *connectionRepository) Save(
	ctx context.Context,
	connection entity.AgentMCPConnection,
	tokens entity.AgentMCPTokens,
) (entity.AgentMCPConnection, error) {
	clientSecret, err := r.seal(tokens.ClientSecret)
	if err != nil {
		return entity.AgentMCPConnection{}, err
	}

	accessToken, err := r.seal(tokens.AccessToken)
	if err != nil {
		return entity.AgentMCPConnection{}, err
	}

	refreshToken, err := r.seal(tokens.RefreshToken)
	if err != nil {
		return entity.AgentMCPConnection{}, err
	}

	model := &dbpostgres.WorkspaceAgentMCPConnection{
		ServerID:             connection.ServerID.String(),
		WorkspaceID:          connection.WorkspaceID.String(),
		Status:               string(entity.AgentMCPConnected),
		Issuer:               connection.Issuer,
		TokenEndpoint:        connection.TokenEndpoint,
		ClientID:             connection.ClientID,
		ClientSecretSealed:   clientSecret,
		Scopes:               types.StringArray(connection.Scopes),
		AccessTokenSealed:    accessToken.Bytes,
		RefreshTokenSealed:   refreshToken,
		ExpiresAt:            null.TimeFromPtr(tokens.ExpiresAt),
		Failure:              "",
		ConnectedByAccountID: connection.ConnectedBy.String(),
		ConnectedAt:          connection.ConnectedAt,
		UpdatedAt:            time.Now().UTC(),
	}

	if model.Scopes == nil {
		model.Scopes = types.StringArray{}
	}

	if err := model.Upsert(
		ctx,
		r.db.Querier(ctx),
		true,
		[]string{dbpostgres.WorkspaceAgentMCPConnectionColumns.ServerID},
		boil.Blacklist(
			dbpostgres.WorkspaceAgentMCPConnectionColumns.ServerID,
			dbpostgres.WorkspaceAgentMCPConnectionColumns.WorkspaceID,
		),
		boil.Infer(),
	); err != nil {
		return entity.AgentMCPConnection{}, fmt.Errorf("save mcp connection: %w", err)
	}

	return toEntity(model)
}

func (r *connectionRepository) MarkFailed(
	ctx context.Context,
	workspaceID, serverID uuid.UUID,
	failure entity.AgentMCPConnectionFailure,
	at time.Time,
) error {
	rows, err := dbpostgres.WorkspaceAgentMCPConnections(
		dbpostgres.WorkspaceAgentMCPConnectionWhere.WorkspaceID.EQ(workspaceID.String()),
		dbpostgres.WorkspaceAgentMCPConnectionWhere.ServerID.EQ(serverID.String()),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.WorkspaceAgentMCPConnectionColumns.Status:    string(entity.AgentMCPFailed),
		dbpostgres.WorkspaceAgentMCPConnectionColumns.Failure:   string(failure),
		dbpostgres.WorkspaceAgentMCPConnectionColumns.UpdatedAt: at,
	})
	if err != nil {
		return fmt.Errorf("mark mcp connection failed: %w", err)
	}

	if rows == 0 {
		return entity.ErrAgentMCPConnectionNotFound
	}

	return nil
}

func (r *connectionRepository) Delete(ctx context.Context, workspaceID, serverID uuid.UUID) error {
	rows, err := dbpostgres.WorkspaceAgentMCPConnections(
		dbpostgres.WorkspaceAgentMCPConnectionWhere.WorkspaceID.EQ(workspaceID.String()),
		dbpostgres.WorkspaceAgentMCPConnectionWhere.ServerID.EQ(serverID.String()),
	).DeleteAll(ctx, r.db.Querier(ctx))
	if err != nil {
		return fmt.Errorf("delete mcp connection: %w", err)
	}

	if rows == 0 {
		return entity.ErrAgentMCPConnectionNotFound
	}

	return nil
}
