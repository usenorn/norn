package agentmcpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/aarondl/sqlboiler/v4/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/crypter"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const (
	uniqueViolationCode = "23505"
	agentNameIndex      = "workspace_agent_mcp_servers_agent_name_key"
	libraryNameIndex    = "workspace_agent_mcp_servers_library_name_key"
	attachmentKey       = "workspace_agent_mcp_server_attachments_pkey"
	orderByName         = "lower(name)"
)

type sealedSecrets struct {
	Env               map[string]string `json:"env,omitempty"`
	Headers           map[string]string `json:"headers,omitempty"`
	OAuthClientSecret string            `json:"oauth_client_secret,omitempty"`
}

type serverRepository struct {
	db      *postgres.Client
	crypter *crypter.Crypter
}

func New(db *postgres.Client, sealer *crypter.Crypter) repository.AgentMCPServer {
	return &serverRepository{db: db, crypter: sealer}
}

func toEntity(model *dbpostgres.WorkspaceAgentMCPServer) (entity.AgentMCPServer, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return entity.AgentMCPServer{}, fmt.Errorf("parse mcp server id: %w", err)
	}

	workspaceID, err := uuid.Parse(model.WorkspaceID)
	if err != nil {
		return entity.AgentMCPServer{}, fmt.Errorf("parse mcp server workspace id: %w", err)
	}

	createdBy, err := uuid.Parse(model.CreatedByAccountID)
	if err != nil {
		return entity.AgentMCPServer{}, fmt.Errorf("parse mcp server author id: %w", err)
	}

	server := entity.AgentMCPServer{
		ID:            id,
		WorkspaceID:   workspaceID,
		Name:          model.Name,
		Transport:     entity.AgentMCPTransport(model.Transport),
		Command:       model.Command,
		Args:          []string(model.Args),
		URL:           model.URL,
		Auth:          entity.AgentMCPAuth(model.Auth),
		EnvKeys:       []string(model.EnvKeys),
		HeaderKeys:    []string(model.HeaderKeys),
		OAuthClientID: model.OauthClientID,
		Registry:      entity.AgentMCPRegistryRef{Name: model.RegistryName, Version: model.RegistryVersion},
		CreatedBy:     createdBy,
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
	}

	if model.AgentID.Valid {
		agentID, err := uuid.Parse(model.AgentID.String)
		if err != nil {
			return entity.AgentMCPServer{}, fmt.Errorf("parse mcp server agent id: %w", err)
		}

		server.AgentID = &agentID
	}

	return server, nil
}

func toEntities(models dbpostgres.WorkspaceAgentMCPServerSlice) ([]entity.AgentMCPServer, error) {
	servers := make([]entity.AgentMCPServer, 0, len(models))

	for _, model := range models {
		server, err := toEntity(model)
		if err != nil {
			return nil, err
		}

		servers = append(servers, server)
	}

	return servers, nil
}

func agentColumn(agentID *uuid.UUID) null.String {
	if agentID == nil {
		return null.String{}
	}

	return null.StringFrom(agentID.String())
}

func nameTaken(err error) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) &&
		pgErr.Code == uniqueViolationCode &&
		(pgErr.ConstraintName == agentNameIndex || pgErr.ConstraintName == libraryNameIndex)
}

func (r *serverRepository) seal(secrets entity.AgentMCPSecrets) (null.Bytes, error) {
	if secrets.Empty() {
		return null.Bytes{}, nil
	}

	payload, err := json.Marshal(sealedSecrets{
		Env:               secrets.Env,
		Headers:           secrets.Headers,
		OAuthClientSecret: secrets.OAuthClientSecret,
	})
	if err != nil {
		return null.Bytes{}, fmt.Errorf("encode mcp server secrets: %w", err)
	}

	sealed, err := r.crypter.Seal(payload)
	if err != nil {
		if errors.Is(err, crypter.ErrKeyMissing) {
			return null.Bytes{}, entity.ErrAgentCapabilityEncryptionKeyMissing
		}

		return null.Bytes{}, fmt.Errorf("seal mcp server secrets: %w", err)
	}

	return null.BytesFrom(sealed), nil
}

func (r *serverRepository) Create(
	ctx context.Context,
	server entity.AgentMCPServer,
	secrets entity.AgentMCPSecrets,
) (entity.AgentMCPServer, error) {
	sealed, err := r.seal(secrets)
	if err != nil {
		return entity.AgentMCPServer{}, err
	}

	now := time.Now().UTC()

	model := &dbpostgres.WorkspaceAgentMCPServer{
		ID:                 server.ID.String(),
		WorkspaceID:        server.WorkspaceID.String(),
		AgentID:            agentColumn(server.AgentID),
		Name:               server.Name,
		Transport:          string(server.Transport),
		Command:            server.Command,
		Args:               types.StringArray(nonNil(server.Args)),
		URL:                server.URL,
		Auth:               string(server.Auth),
		EnvKeys:            types.StringArray(entity.AgentMCPVariableKeys(secrets.Env)),
		HeaderKeys:         types.StringArray(entity.AgentMCPVariableKeys(secrets.Headers)),
		OauthClientID:      server.OAuthClientID,
		SecretsSealed:      sealed,
		RegistryName:       server.Registry.Name,
		RegistryVersion:    server.Registry.Version,
		CreatedByAccountID: server.CreatedBy.String(),
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		if nameTaken(err) {
			return entity.AgentMCPServer{}, entity.ErrAgentMCPServerNameTaken
		}

		return entity.AgentMCPServer{}, fmt.Errorf("insert mcp server: %w", err)
	}

	return toEntity(model)
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}

	return values
}

func (r *serverRepository) Update(
	ctx context.Context,
	server entity.AgentMCPServer,
	secrets entity.AgentMCPSecrets,
) (entity.AgentMCPServer, error) {
	sealed, err := r.seal(secrets)
	if err != nil {
		return entity.AgentMCPServer{}, err
	}

	rows, err := dbpostgres.WorkspaceAgentMCPServers(
		dbpostgres.WorkspaceAgentMCPServerWhere.WorkspaceID.EQ(server.WorkspaceID.String()),
		dbpostgres.WorkspaceAgentMCPServerWhere.ID.EQ(server.ID.String()),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.WorkspaceAgentMCPServerColumns.Name:            server.Name,
		dbpostgres.WorkspaceAgentMCPServerColumns.Transport:       string(server.Transport),
		dbpostgres.WorkspaceAgentMCPServerColumns.Command:         server.Command,
		dbpostgres.WorkspaceAgentMCPServerColumns.Args:            types.StringArray(nonNil(server.Args)),
		dbpostgres.WorkspaceAgentMCPServerColumns.URL:             server.URL,
		dbpostgres.WorkspaceAgentMCPServerColumns.Auth:            string(server.Auth),
		dbpostgres.WorkspaceAgentMCPServerColumns.EnvKeys:         types.StringArray(entity.AgentMCPVariableKeys(secrets.Env)),
		dbpostgres.WorkspaceAgentMCPServerColumns.HeaderKeys:      types.StringArray(entity.AgentMCPVariableKeys(secrets.Headers)),
		dbpostgres.WorkspaceAgentMCPServerColumns.OauthClientID:   server.OAuthClientID,
		dbpostgres.WorkspaceAgentMCPServerColumns.SecretsSealed:   sealed,
		dbpostgres.WorkspaceAgentMCPServerColumns.RegistryName:    server.Registry.Name,
		dbpostgres.WorkspaceAgentMCPServerColumns.RegistryVersion: server.Registry.Version,
		dbpostgres.WorkspaceAgentMCPServerColumns.UpdatedAt:       time.Now().UTC(),
	})
	if err != nil {
		if nameTaken(err) {
			return entity.AgentMCPServer{}, entity.ErrAgentMCPServerNameTaken
		}

		return entity.AgentMCPServer{}, fmt.Errorf("update mcp server: %w", err)
	}

	if rows == 0 {
		return entity.AgentMCPServer{}, entity.ErrAgentMCPServerNotFound
	}

	return r.Get(ctx, server.WorkspaceID, server.ID)
}

func (r *serverRepository) find(
	ctx context.Context,
	workspaceID, serverID uuid.UUID,
) (*dbpostgres.WorkspaceAgentMCPServer, error) {
	model, err := dbpostgres.WorkspaceAgentMCPServers(
		dbpostgres.WorkspaceAgentMCPServerWhere.WorkspaceID.EQ(workspaceID.String()),
		dbpostgres.WorkspaceAgentMCPServerWhere.ID.EQ(serverID.String()),
	).One(ctx, r.db.Querier(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, entity.ErrAgentMCPServerNotFound
		}

		return nil, fmt.Errorf("read mcp server: %w", err)
	}

	return model, nil
}

func (r *serverRepository) Get(ctx context.Context, workspaceID, serverID uuid.UUID) (entity.AgentMCPServer, error) {
	model, err := r.find(ctx, workspaceID, serverID)
	if err != nil {
		return entity.AgentMCPServer{}, err
	}

	return toEntity(model)
}

func (r *serverRepository) Secrets(
	ctx context.Context,
	workspaceID, serverID uuid.UUID,
) (entity.AgentMCPSecrets, error) {
	model, err := r.find(ctx, workspaceID, serverID)
	if err != nil {
		return entity.AgentMCPSecrets{}, err
	}

	if !model.SecretsSealed.Valid {
		return entity.AgentMCPSecrets{}, nil
	}

	payload, err := r.crypter.Open(model.SecretsSealed.Bytes)
	if err != nil {
		if errors.Is(err, crypter.ErrKeyMissing) {
			return entity.AgentMCPSecrets{}, entity.ErrAgentCapabilityEncryptionKeyMissing
		}

		return entity.AgentMCPSecrets{}, fmt.Errorf("open mcp server secrets: %w", err)
	}

	var stored sealedSecrets
	if err := json.Unmarshal(payload, &stored); err != nil {
		return entity.AgentMCPSecrets{}, fmt.Errorf("decode mcp server secrets: %w", err)
	}

	return entity.AgentMCPSecrets{
		Env:               stored.Env,
		Headers:           stored.Headers,
		OAuthClientSecret: stored.OAuthClientSecret,
	}, nil
}

func (r *serverRepository) ListByAgent(
	ctx context.Context,
	workspaceID, agentID uuid.UUID,
) ([]entity.AgentMCPServer, error) {
	models, err := dbpostgres.WorkspaceAgentMCPServers(
		dbpostgres.WorkspaceAgentMCPServerWhere.WorkspaceID.EQ(workspaceID.String()),
		qm.Expr(
			dbpostgres.WorkspaceAgentMCPServerWhere.AgentID.EQ(null.StringFrom(agentID.String())),
			qm.Or(
				dbpostgres.WorkspaceAgentMCPServerColumns.ID+
					" IN (SELECT server_id FROM workspace_agent_mcp_server_attachments WHERE agent_id = ?)",
				agentID.String(),
			),
		),
		qm.OrderBy(orderByName),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list agent mcp servers: %w", err)
	}

	return toEntities(models)
}

func (r *serverRepository) ListLibrary(ctx context.Context, workspaceID uuid.UUID) ([]entity.AgentMCPServer, error) {
	models, err := dbpostgres.WorkspaceAgentMCPServers(
		dbpostgres.WorkspaceAgentMCPServerWhere.WorkspaceID.EQ(workspaceID.String()),
		dbpostgres.WorkspaceAgentMCPServerWhere.AgentID.IsNull(),
		qm.OrderBy(orderByName),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list library mcp servers: %w", err)
	}

	return toEntities(models)
}

func (r *serverRepository) CountOwned(
	ctx context.Context,
	workspaceID uuid.UUID,
	agentID *uuid.UUID,
) (int, error) {
	owner := dbpostgres.WorkspaceAgentMCPServerWhere.AgentID.IsNull()
	if agentID != nil {
		owner = dbpostgres.WorkspaceAgentMCPServerWhere.AgentID.EQ(null.StringFrom(agentID.String()))
	}

	count, err := dbpostgres.WorkspaceAgentMCPServers(
		dbpostgres.WorkspaceAgentMCPServerWhere.WorkspaceID.EQ(workspaceID.String()),
		owner,
	).Count(ctx, r.db.Querier(ctx))
	if err != nil {
		return 0, fmt.Errorf("count mcp servers: %w", err)
	}

	return int(count), nil
}

func (r *serverRepository) Delete(ctx context.Context, workspaceID, serverID uuid.UUID) error {
	rows, err := dbpostgres.WorkspaceAgentMCPServers(
		dbpostgres.WorkspaceAgentMCPServerWhere.WorkspaceID.EQ(workspaceID.String()),
		dbpostgres.WorkspaceAgentMCPServerWhere.ID.EQ(serverID.String()),
	).DeleteAll(ctx, r.db.Querier(ctx))
	if err != nil {
		return fmt.Errorf("delete mcp server: %w", err)
	}

	if rows == 0 {
		return entity.ErrAgentMCPServerNotFound
	}

	return nil
}

func (r *serverRepository) Attach(ctx context.Context, attachment entity.AgentCapabilityAttachment) error {
	model := &dbpostgres.WorkspaceAgentMCPServerAttachment{
		AgentID:             attachment.AgentID.String(),
		ServerID:            attachment.CapabilityID.String(),
		WorkspaceID:         attachment.WorkspaceID.String(),
		AttachedByAccountID: attachment.AttachedBy.String(),
		AttachedAt:          time.Now().UTC(),
	}

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode && pgErr.ConstraintName == attachmentKey {
			return entity.ErrAgentCapabilityAttached
		}

		return fmt.Errorf("attach mcp server: %w", err)
	}

	return nil
}

func (r *serverRepository) Detach(ctx context.Context, workspaceID, agentID, serverID uuid.UUID) error {
	rows, err := dbpostgres.WorkspaceAgentMCPServerAttachments(
		dbpostgres.WorkspaceAgentMCPServerAttachmentWhere.WorkspaceID.EQ(workspaceID.String()),
		dbpostgres.WorkspaceAgentMCPServerAttachmentWhere.AgentID.EQ(agentID.String()),
		dbpostgres.WorkspaceAgentMCPServerAttachmentWhere.ServerID.EQ(serverID.String()),
	).DeleteAll(ctx, r.db.Querier(ctx))
	if err != nil {
		return fmt.Errorf("detach mcp server: %w", err)
	}

	if rows == 0 {
		return entity.ErrAgentCapabilityNotAttached
	}

	return nil
}

func (r *serverRepository) AttachedAgents(
	ctx context.Context,
	workspaceID uuid.UUID,
) (map[uuid.UUID][]uuid.UUID, error) {
	models, err := dbpostgres.WorkspaceAgentMCPServerAttachments(
		dbpostgres.WorkspaceAgentMCPServerAttachmentWhere.WorkspaceID.EQ(workspaceID.String()),
		qm.OrderBy(dbpostgres.WorkspaceAgentMCPServerAttachmentColumns.AttachedAt),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list mcp server attachments: %w", err)
	}

	attached := make(map[uuid.UUID][]uuid.UUID, len(models))

	for _, model := range models {
		serverID, err := uuid.Parse(model.ServerID)
		if err != nil {
			return nil, fmt.Errorf("parse attached mcp server id: %w", err)
		}

		agentID, err := uuid.Parse(model.AgentID)
		if err != nil {
			return nil, fmt.Errorf("parse attached agent id: %w", err)
		}

		attached[serverID] = append(attached[serverID], agentID)
	}

	return attached, nil
}
