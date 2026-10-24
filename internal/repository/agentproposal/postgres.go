package agentproposal

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const proposalColumns = `
	p.id, p.workspace_id, p.agent_id, coalesce(a.name, ''),
	coalesce(p.issue_id::text, ''), coalesce(p.team_id::text, ''), coalesce(p.doc_id::text, ''),
	p.action, p.change, p.reasoning, p.status, coalesce(p.decided_by_account_id::text, ''),
	p.decided_at, coalesce(p.failure, ''), p.created_at, p.updated_at
	FROM workspace_agent_proposals p
	LEFT JOIN workspace_agents a ON a.id = p.agent_id`

const insertProposalQuery = `
	INSERT INTO workspace_agent_proposals (
	    id, workspace_id, agent_id, issue_id, team_id, doc_id, action, change, reasoning
	)
	VALUES ($1, $2, $3, nullif($4, '')::uuid, nullif($5, '')::uuid, nullif($6, '')::uuid, $7, $8, $9)`

const rewriteProposalQuery = `
	UPDATE workspace_agent_proposals
	SET change = $2, updated_at = now()
	WHERE id = $1 AND status = 'pending'`

const settleProposalQuery = `
	UPDATE workspace_agent_proposals
	SET status = $2, decided_by_account_id = $3, decided_at = $4,
	    failure = nullif($5, ''), updated_at = now()
	WHERE id = $1 AND status = 'pending'`

type agentProposalRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.AgentProposal {
	return &agentProposalRepository{db: db}
}

func (r *agentProposalRepository) Rewrite(
	ctx context.Context,
	proposalID uuid.UUID,
	change entity.AgentChange,
) error {
	encoded, err := json.Marshal(change)
	if err != nil {
		return fmt.Errorf("encode agent proposal change: %w", err)
	}

	result, err := r.db.Querier(ctx).ExecContext(
		ctx, rewriteProposalQuery, proposalID.String(), encoded,
	)
	if err != nil {
		return fmt.Errorf("rewrite agent proposal: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rewrite agent proposal: %w", err)
	}

	if affected == 0 {
		return entity.ErrAgentProposalSettled
	}

	return nil
}

func (r *agentProposalRepository) Create(
	ctx context.Context,
	proposal entity.AgentProposal,
) (entity.AgentProposal, error) {
	if proposal.ID == uuid.Nil {
		proposal.ID = uuid.New()
	}

	change, err := json.Marshal(proposal.Change)
	if err != nil {
		return entity.AgentProposal{}, fmt.Errorf("encode agent proposal change: %w", err)
	}

	reasoning, err := json.Marshal(proposal.Reasoning)
	if err != nil {
		return entity.AgentProposal{}, fmt.Errorf("encode agent proposal reasoning: %w", err)
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		insertProposalQuery,
		proposal.ID.String(),
		proposal.WorkspaceID.String(),
		proposal.AgentID.String(),
		optionalReference(proposal.IssueID),
		optionalReference(proposal.TeamID),
		optionalReference(proposal.DocID),
		string(proposal.Action),
		change,
		reasoning,
	); err != nil {
		return entity.AgentProposal{}, fmt.Errorf("insert agent proposal: %w", err)
	}

	return r.GetByID(ctx, proposal.WorkspaceID, proposal.ID)
}

func optionalReference(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}

	return id.String()
}

func (r *agentProposalRepository) GetByID(
	ctx context.Context,
	workspaceID, proposalID uuid.UUID,
) (entity.AgentProposal, error) {
	proposals, err := r.many(
		ctx,
		`SELECT`+proposalColumns+` WHERE p.workspace_id = $1 AND p.id = $2`,
		workspaceID.String(),
		proposalID.String(),
	)
	if err != nil {
		return entity.AgentProposal{}, err
	}

	if len(proposals) == 0 {
		return entity.AgentProposal{}, entity.ErrAgentProposalNotFound
	}

	return proposals[0], nil
}

func (r *agentProposalRepository) ListWaiting(
	ctx context.Context,
	workspaceID uuid.UUID,
	limit int,
) ([]entity.AgentProposal, error) {
	return r.many(
		ctx,
		`SELECT`+proposalColumns+`
		WHERE p.workspace_id = $1 AND p.status = 'pending'
		ORDER BY p.created_at, p.id
		LIMIT $2`,
		workspaceID.String(),
		limit,
	)
}

func (r *agentProposalRepository) ListByAgent(
	ctx context.Context,
	agentID uuid.UUID,
	limit int,
) ([]entity.AgentProposal, error) {
	return r.many(
		ctx,
		`SELECT`+proposalColumns+`
		WHERE p.agent_id = $1
		ORDER BY p.created_at DESC
		LIMIT $2`,
		agentID.String(),
		limit,
	)
}

func (r *agentProposalRepository) Settle(
	ctx context.Context,
	proposalID uuid.UUID,
	status entity.AgentProposalStatus,
	decidedBy uuid.UUID,
	decidedAt time.Time,
	failure string,
) error {
	var decider any
	if decidedBy != uuid.Nil {
		decider = decidedBy.String()
	}

	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		settleProposalQuery,
		proposalID.String(),
		string(status),
		decider,
		decidedAt,
		failure,
	)
	if err != nil {
		return fmt.Errorf("settle agent proposal: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("settle agent proposal: %w", err)
	}

	if affected == 0 {
		return entity.ErrAgentProposalSettled
	}

	return nil
}

func (r *agentProposalRepository) many(
	ctx context.Context,
	query string,
	args ...any,
) ([]entity.AgentProposal, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query agent proposals: %w", err)
	}

	defer func() { _ = rows.Close() }()

	proposals := make([]entity.AgentProposal, 0)

	for rows.Next() {
		var (
			rawID, rawWorkspace, rawAgent string
			agentName                     string
			rawIssue, rawTeam, rawDoc     string
			action, status, decidedBy     string
			change, reasoning             []byte
			decidedAt                     sql.NullTime
			failure                       string
			createdAt, updatedAt          time.Time
		)

		if err := rows.Scan(
			&rawID, &rawWorkspace, &rawAgent, &agentName, &rawIssue, &rawTeam, &rawDoc,
			&action, &change, &reasoning, &status, &decidedBy,
			&decidedAt, &failure, &createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan agent proposal: %w", err)
		}

		proposal := entity.AgentProposal{
			AgentName: agentName,
			Action:    entity.AgentAction(action),
			Status:    entity.AgentProposalStatus(status),
			Failure:   failure,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		}

		for target, raw := range map[*uuid.UUID]string{
			&proposal.ID:          rawID,
			&proposal.WorkspaceID: rawWorkspace,
			&proposal.AgentID:     rawAgent,
		} {
			parsed, err := uuid.Parse(raw)
			if err != nil {
				return nil, fmt.Errorf("parse agent proposal identifier: %w", err)
			}

			*target = parsed
		}

		for target, raw := range map[*uuid.UUID]string{
			&proposal.IssueID: rawIssue,
			&proposal.TeamID:  rawTeam,
			&proposal.DocID:   rawDoc,
		} {
			if raw == "" {
				continue
			}

			parsed, err := uuid.Parse(raw)
			if err != nil {
				return nil, fmt.Errorf("parse agent proposal subject: %w", err)
			}

			*target = parsed
		}

		if err := json.Unmarshal(change, &proposal.Change); err != nil {
			return nil, fmt.Errorf("decode agent proposal change: %w", err)
		}

		if err := json.Unmarshal(reasoning, &proposal.Reasoning); err != nil {
			return nil, fmt.Errorf("decode agent proposal reasoning: %w", err)
		}

		if decidedBy != "" {
			decider, err := uuid.Parse(decidedBy)
			if err != nil {
				return nil, fmt.Errorf("parse agent proposal decider: %w", err)
			}

			proposal.DecidedBy = decider
		}

		if decidedAt.Valid {
			proposal.DecidedAt = &decidedAt.Time
		}

		proposals = append(proposals, proposal)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read agent proposals: %w", err)
	}

	return proposals, nil
}
