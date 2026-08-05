package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const (
	uniqueViolation = "23505"
	nameUniqueIndex = "workspace_agents_live_name_key"
)

const agentColumns = `
	a.id, a.workspace_id, a.account_id, a.owner_account_id, a.name, a.status,
	a.action_limit, a.disabled_at, a.created_at, a.updated_at`

const insertAgentQuery = `
	INSERT INTO workspace_agents (id, workspace_id, account_id, owner_account_id, name, action_limit)
	VALUES ($1, $2, $3, $4, $5, $6)`

type agentRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.Agent {
	return &agentRepository{db: db}
}

func (r *agentRepository) Create(ctx context.Context, agent entity.Agent) (entity.Agent, error) {
	if agent.ID == uuid.Nil {
		agent.ID = uuid.New()
	}

	var limit any
	if agent.ActionLimit != nil {
		limit = *agent.ActionLimit
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		insertAgentQuery,
		agent.ID.String(),
		agent.WorkspaceID.String(),
		agent.AccountID.String(),
		agent.OwnerAccountID.String(),
		agent.Name,
		limit,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation && pgErr.ConstraintName == nameUniqueIndex {
			return entity.Agent{}, entity.ErrAgentNameTaken
		}

		return entity.Agent{}, fmt.Errorf("insert agent: %w", err)
	}

	return r.GetByID(ctx, agent.WorkspaceID, agent.ID)
}

func (r *agentRepository) GetByID(ctx context.Context, workspaceID, agentID uuid.UUID) (entity.Agent, error) {
	return r.one(
		ctx,
		`SELECT`+agentColumns+` FROM workspace_agents a WHERE a.workspace_id = $1 AND a.id = $2`,
		workspaceID.String(),
		agentID.String(),
	)
}

func (r *agentRepository) GetByAccountID(ctx context.Context, accountID uuid.UUID) (entity.Agent, error) {
	return r.one(
		ctx,
		`SELECT`+agentColumns+` FROM workspace_agents a WHERE a.account_id = $1`,
		accountID.String(),
	)
}

func (r *agentRepository) ListByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]entity.Agent, error) {
	return r.many(
		ctx,
		`SELECT`+agentColumns+`
		FROM workspace_agents a
		WHERE a.workspace_id = $1
		ORDER BY a.status, lower(a.name)`,
		workspaceID.String(),
	)
}

func (r *agentRepository) Disable(
	ctx context.Context,
	workspaceID, agentID uuid.UUID,
	disabledAt time.Time,
) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		`UPDATE workspace_agents
		 SET status = 'disabled', disabled_at = $3, updated_at = now()
		 WHERE workspace_id = $1 AND id = $2 AND status = 'active'`,
		workspaceID.String(),
		agentID.String(),
		disabledAt,
	)
	if err != nil {
		return fmt.Errorf("disable agent: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("disable agent: %w", err)
	}

	if affected == 0 {
		return entity.ErrAgentNotFound
	}

	return nil
}

func (r *agentRepository) one(ctx context.Context, query string, args ...any) (entity.Agent, error) {
	agents, err := r.many(ctx, query, args...)
	if err != nil {
		return entity.Agent{}, err
	}

	if len(agents) == 0 {
		return entity.Agent{}, entity.ErrAgentNotFound
	}

	return agents[0], nil
}

func (r *agentRepository) many(ctx context.Context, query string, args ...any) ([]entity.Agent, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query agents: %w", err)
	}

	defer func() { _ = rows.Close() }()

	agents := make([]entity.Agent, 0)

	for rows.Next() {
		var (
			rawID, rawWorkspace  string
			rawAccount, rawOwner string
			name, status         string
			limit                sql.NullInt64
			disabledAt           sql.NullTime
			createdAt, updatedAt time.Time
		)

		if err := rows.Scan(
			&rawID, &rawWorkspace, &rawAccount, &rawOwner, &name, &status,
			&limit, &disabledAt, &createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}

		agent := entity.Agent{
			Name:      name,
			Status:    entity.AgentStatus(status),
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		}

		for target, raw := range map[*uuid.UUID]string{
			&agent.ID:             rawID,
			&agent.WorkspaceID:    rawWorkspace,
			&agent.AccountID:      rawAccount,
			&agent.OwnerAccountID: rawOwner,
		} {
			parsed, err := uuid.Parse(raw)
			if err != nil {
				return nil, fmt.Errorf("parse agent identifier: %w", err)
			}

			*target = parsed
		}

		if limit.Valid {
			allowance := int(limit.Int64)
			agent.ActionLimit = &allowance
		}

		if disabledAt.Valid {
			agent.DisabledAt = &disabledAt.Time
		}

		agents = append(agents, agent)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read agents: %w", err)
	}

	return agents, nil
}
