package codebase

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/aarondl/sqlboiler/v4/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const (
	uniqueViolationCode = "23505"
	liveRootUniqueIndex = "workspace_codebases_live_root_key"
)

const codebaseColumns = `
       c.id,
       c.runner_id,
       r.workspace_id,
       r.agent_id,
       c.name,
       c.root_path,
       c.state,
       c.shared_files,
       c.runtimes,
       c.tools,
       c.connected_at,
       c.last_seen_at,
       c.disconnected_at,
       c.updated_at`

const codebaseJoins = `
FROM workspace_codebases c
JOIN workspace_runners r ON r.id = c.runner_id`

const insertCodebaseQuery = `
WITH inserted AS (
    INSERT INTO workspace_codebases
        (runner_id, name, root_path, state, shared_files, runtimes, tools, connected_at, updated_at)
    VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $8)
    RETURNING *
)
SELECT` + codebaseColumns + `
FROM inserted c
JOIN workspace_runners r ON r.id = c.runner_id`

const codebaseByIDQuery = `
SELECT` + codebaseColumns + codebaseJoins + `
WHERE c.id = $1`

const liveCodebaseByRootQuery = `
SELECT` + codebaseColumns + codebaseJoins + `
WHERE c.runner_id = $1 AND c.root_path = $2 AND c.state <> 'disconnected'`

const codebasesByRunnerQuery = `
SELECT` + codebaseColumns + codebaseJoins + `
WHERE c.runner_id = $1
ORDER BY c.connected_at DESC, c.id`

const codebasesByAgentQuery = `
SELECT` + codebaseColumns + codebaseJoins + `
WHERE r.agent_id = $1
ORDER BY c.connected_at DESC, c.id`

const replaceCodebaseQuery = `
WITH updated AS (
    UPDATE workspace_codebases
    SET name         = $2,
        shared_files = $3,
        runtimes     = $4,
        tools        = $5::jsonb,
        state        = $6,
        last_seen_at = $7,
        updated_at   = $7
    WHERE id = $1 AND state <> 'disconnected'
    RETURNING *
)
SELECT` + codebaseColumns + `
FROM updated c
JOIN workspace_runners r ON r.id = c.runner_id`

const confirmCodebaseQuery = `
WITH updated AS (
    UPDATE workspace_codebases
    SET state = 'active', last_seen_at = $2, updated_at = $2
    WHERE id = $1 AND state = 'drift'
    RETURNING *
)
SELECT` + codebaseColumns + `
FROM updated c
JOIN workspace_runners r ON r.id = c.runner_id`

const disconnectCodebaseQuery = `
WITH updated AS (
    UPDATE workspace_codebases
    SET state = 'disconnected', disconnected_at = $2, updated_at = $2
    WHERE id = $1 AND state <> 'disconnected'
    RETURNING *
)
SELECT` + codebaseColumns + `
FROM updated c
JOIN workspace_runners r ON r.id = c.runner_id`

const recordCodebaseSeenQuery = `
UPDATE workspace_codebases SET last_seen_at = $2 WHERE id = $1`

const deleteCodebaseRepositoriesQuery = `
DELETE FROM workspace_codebase_repositories WHERE codebase_id = $1`

const insertCodebaseRepositoryQuery = `
INSERT INTO workspace_codebase_repositories
    (codebase_id, ordinal, name, rel_path, default_branch, remote_hash, remote_host, remote_path_tail)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

const codebaseRepositoriesQuery = `
SELECT codebase_id, name, rel_path, default_branch, remote_hash, remote_host, remote_path_tail
FROM workspace_codebase_repositories
WHERE codebase_id = ANY($1::uuid[])
ORDER BY codebase_id, ordinal`

type codebaseRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.Codebase {
	return &codebaseRepository{db: db}
}

type scanner interface {
	Scan(dest ...any) error
}

func scanCodebase(row scanner) (entity.Codebase, error) {
	var (
		codebase  entity.Codebase
		id        string
		runnerID  string
		workspace string
		agentID   string
		state     string
		shared    types.StringArray
		runtimes  types.StringArray
		tools     []byte
	)

	if err := row.Scan(
		&id,
		&runnerID,
		&workspace,
		&agentID,
		&codebase.Name,
		&codebase.RootPath,
		&state,
		&shared,
		&runtimes,
		&tools,
		&codebase.ConnectedAt,
		&codebase.LastSeenAt,
		&codebase.DisconnectedAt,
		&codebase.UpdatedAt,
	); err != nil {
		return entity.Codebase{}, err
	}

	parsed, err := uuid.Parse(id)
	if err != nil {
		return entity.Codebase{}, fmt.Errorf("parse codebase id: %w", err)
	}

	codebase.ID = parsed

	if codebase.RunnerID, err = uuid.Parse(runnerID); err != nil {
		return entity.Codebase{}, fmt.Errorf("parse codebase runner id: %w", err)
	}

	if codebase.WorkspaceID, err = uuid.Parse(workspace); err != nil {
		return entity.Codebase{}, fmt.Errorf("parse codebase workspace id: %w", err)
	}

	if codebase.AgentID, err = uuid.Parse(agentID); err != nil {
		return entity.Codebase{}, fmt.Errorf("parse codebase agent id: %w", err)
	}

	codebase.State = entity.CodebaseState(state)
	codebase.SharedFiles = []string(shared)
	codebase.Repositories = make([]entity.CodebaseRepository, 0)

	codebase.Runtimes = make([]entity.CodebaseRuntime, 0, len(runtimes))
	for _, runtime := range runtimes {
		codebase.Runtimes = append(codebase.Runtimes, entity.CodebaseRuntime(runtime))
	}

	var stored []storedTool
	if err := json.Unmarshal(tools, &stored); err != nil {
		return entity.Codebase{}, fmt.Errorf("decode codebase tools: %w", err)
	}

	codebase.Tools = make([]entity.CodingTool, 0, len(stored))
	for _, tool := range stored {
		codebase.Tools = append(codebase.Tools, entity.CodingTool{Name: tool.Name, Version: tool.Version})
	}

	return codebase, nil
}

type storedTool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func encodeTools(tools []entity.CodingTool) ([]byte, error) {
	stored := make([]storedTool, 0, len(tools))
	for _, tool := range tools {
		stored = append(stored, storedTool{Name: tool.Name, Version: tool.Version})
	}

	encoded, err := json.Marshal(stored)
	if err != nil {
		return nil, fmt.Errorf("encode codebase tools: %w", err)
	}

	return encoded, nil
}

func runtimeStrings(runtimes []entity.CodebaseRuntime) types.StringArray {
	values := make(types.StringArray, 0, len(runtimes))
	for _, runtime := range runtimes {
		values = append(values, string(runtime))
	}

	return values
}

func (r *codebaseRepository) hydrate(ctx context.Context, codebases []entity.Codebase) error {
	if len(codebases) == 0 {
		return nil
	}

	ids := make([]string, 0, len(codebases))
	for _, codebase := range codebases {
		ids = append(ids, codebase.ID.String())
	}

	rows, err := r.db.Querier(ctx).QueryContext(ctx, codebaseRepositoriesQuery, types.StringArray(ids))
	if err != nil {
		return fmt.Errorf("list codebase repositories: %w", err)
	}

	defer func() { _ = rows.Close() }()

	held := make(map[string][]entity.CodebaseRepository, len(codebases))

	for rows.Next() {
		var (
			codebaseID string
			repository entity.CodebaseRepository
		)

		if err := rows.Scan(
			&codebaseID,
			&repository.Name,
			&repository.RelPath,
			&repository.DefaultBranch,
			&repository.Remote.Hash,
			&repository.Remote.Host,
			&repository.Remote.PathTail,
		); err != nil {
			return fmt.Errorf("scan codebase repository: %w", err)
		}

		held[codebaseID] = append(held[codebaseID], repository)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate codebase repositories: %w", err)
	}

	for index := range codebases {
		if found, ok := held[codebases[index].ID.String()]; ok {
			codebases[index].Repositories = found
		}
	}

	return nil
}

func (r *codebaseRepository) writeRepositories(
	ctx context.Context,
	codebaseID uuid.UUID,
	repositories []entity.CodebaseRepository,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, deleteCodebaseRepositoriesQuery, codebaseID.String(),
	); err != nil {
		return fmt.Errorf("clear codebase repositories: %w", err)
	}

	for ordinal, repository := range repositories {
		if _, err := r.db.Querier(ctx).ExecContext(
			ctx,
			insertCodebaseRepositoryQuery,
			codebaseID.String(),
			ordinal,
			repository.Name,
			repository.RelPath,
			repository.DefaultBranch,
			repository.Remote.Hash,
			repository.Remote.Host,
			repository.Remote.PathTail,
		); err != nil {
			return fmt.Errorf("insert codebase repository: %w", err)
		}
	}

	return nil
}

func (r *codebaseRepository) find(
	ctx context.Context,
	query string,
	args ...any,
) (entity.Codebase, error) {
	codebase, err := scanCodebase(r.db.Querier(ctx).QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Codebase{}, entity.ErrCodebaseNotFound
		}

		return entity.Codebase{}, fmt.Errorf("find codebase: %w", err)
	}

	held := []entity.Codebase{codebase}
	if err := r.hydrate(ctx, held); err != nil {
		return entity.Codebase{}, err
	}

	return held[0], nil
}

func (r *codebaseRepository) list(
	ctx context.Context,
	query string,
	args ...any,
) ([]entity.Codebase, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list codebases: %w", err)
	}

	defer func() { _ = rows.Close() }()

	codebases := make([]entity.Codebase, 0)

	for rows.Next() {
		codebase, err := scanCodebase(rows)
		if err != nil {
			return nil, fmt.Errorf("scan codebase: %w", err)
		}

		codebases = append(codebases, codebase)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate codebases: %w", err)
	}

	if err := r.hydrate(ctx, codebases); err != nil {
		return nil, err
	}

	return codebases, nil
}

func (r *codebaseRepository) Connect(
	ctx context.Context,
	runnerID uuid.UUID,
	inventory repository.CodebaseInventory,
	connectedAt time.Time,
) (entity.Codebase, error) {
	tools, err := encodeTools(inventory.Tools)
	if err != nil {
		return entity.Codebase{}, err
	}

	connected, err := scanCodebase(r.db.Querier(ctx).QueryRowContext(
		ctx,
		insertCodebaseQuery,
		runnerID.String(),
		inventory.Name,
		inventory.RootPath,
		string(entity.CodebaseStateActive),
		types.StringArray(inventory.SharedFiles),
		runtimeStrings(inventory.Runtimes),
		tools,
		connectedAt,
	))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == uniqueViolationCode &&
			pgErr.ConstraintName == liveRootUniqueIndex {
			return entity.Codebase{}, entity.ErrCodebaseRootTaken
		}

		return entity.Codebase{}, fmt.Errorf("insert codebase: %w", err)
	}

	if err := r.writeRepositories(ctx, connected.ID, inventory.Repositories); err != nil {
		return entity.Codebase{}, err
	}

	connected.Repositories = inventory.Repositories

	return connected, nil
}

func (r *codebaseRepository) GetByID(
	ctx context.Context,
	codebaseID uuid.UUID,
) (entity.Codebase, error) {
	return r.find(ctx, codebaseByIDQuery, codebaseID.String())
}

func (r *codebaseRepository) GetLiveByRoot(
	ctx context.Context,
	runnerID uuid.UUID,
	rootPath string,
) (entity.Codebase, error) {
	return r.find(ctx, liveCodebaseByRootQuery, runnerID.String(), rootPath)
}

func (r *codebaseRepository) ListByRunnerID(
	ctx context.Context,
	runnerID uuid.UUID,
) ([]entity.Codebase, error) {
	return r.list(ctx, codebasesByRunnerQuery, runnerID.String())
}

func (r *codebaseRepository) ListByAgentID(
	ctx context.Context,
	agentID uuid.UUID,
) ([]entity.Codebase, error) {
	return r.list(ctx, codebasesByAgentQuery, agentID.String())
}

func (r *codebaseRepository) Replace(
	ctx context.Context,
	codebaseID uuid.UUID,
	inventory repository.CodebaseInventory,
	state entity.CodebaseState,
	at time.Time,
) (entity.Codebase, error) {
	tools, err := encodeTools(inventory.Tools)
	if err != nil {
		return entity.Codebase{}, err
	}

	updated, err := scanCodebase(r.db.Querier(ctx).QueryRowContext(
		ctx,
		replaceCodebaseQuery,
		codebaseID.String(),
		inventory.Name,
		types.StringArray(inventory.SharedFiles),
		runtimeStrings(inventory.Runtimes),
		tools,
		string(state),
		at,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Codebase{}, entity.ErrCodebaseDisconnected
		}

		return entity.Codebase{}, fmt.Errorf("replace codebase inventory: %w", err)
	}

	if err := r.writeRepositories(ctx, updated.ID, inventory.Repositories); err != nil {
		return entity.Codebase{}, err
	}

	updated.Repositories = inventory.Repositories

	return updated, nil
}

func (r *codebaseRepository) Confirm(
	ctx context.Context,
	codebaseID uuid.UUID,
	at time.Time,
) (entity.Codebase, error) {
	confirmed, err := r.find(ctx, confirmCodebaseQuery, codebaseID.String(), at)
	if err != nil {
		if errors.Is(err, entity.ErrCodebaseNotFound) {
			return entity.Codebase{}, entity.ErrCodebaseNotDrifted
		}

		return entity.Codebase{}, err
	}

	return confirmed, nil
}

func (r *codebaseRepository) Disconnect(
	ctx context.Context,
	codebaseID uuid.UUID,
	at time.Time,
) (entity.Codebase, error) {
	disconnected, err := r.find(ctx, disconnectCodebaseQuery, codebaseID.String(), at)
	if err != nil {
		if errors.Is(err, entity.ErrCodebaseNotFound) {
			return entity.Codebase{}, entity.ErrCodebaseDisconnected
		}

		return entity.Codebase{}, err
	}

	return disconnected, nil
}

func (r *codebaseRepository) RecordSeen(
	ctx context.Context,
	codebaseID uuid.UUID,
	seenAt time.Time,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, recordCodebaseSeenQuery, codebaseID.String(), seenAt,
	); err != nil {
		return fmt.Errorf("record codebase seen: %w", err)
	}

	return nil
}
