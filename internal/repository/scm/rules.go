package scm

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

type ruleRepository struct {
	db *postgres.Client
}

func NewSCMTransitionRule(db *postgres.Client) repository.SCMTransitionRule {
	return &ruleRepository{db: db}
}

const ruleColumns = `
    id, workspace_id, team_id, trigger, state_id, created_at, updated_at`

func scanRule(row interface{ Scan(...any) error }) (entity.SCMTransitionRule, error) {
	var rule entity.SCMTransitionRule

	err := row.Scan(
		&rule.ID,
		&rule.WorkspaceID,
		&rule.TeamID,
		&rule.Trigger,
		&rule.StateID,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	)
	if err != nil {
		return entity.SCMTransitionRule{}, err
	}

	return rule, nil
}

const listRulesQuery = `
SELECT` + ruleColumns + `
FROM workspace_scm_transition_rules
WHERE workspace_id = $1 AND team_id = $2
ORDER BY trigger`

func (r *ruleRepository) ListByTeam(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
) (entity.SCMTransitionRules, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, listRulesQuery, workspaceID, teamID)
	if err != nil {
		return nil, fmt.Errorf("list transition rules: %w", err)
	}

	defer func() { _ = rows.Close() }()

	rules := make(entity.SCMTransitionRules, 0)

	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, fmt.Errorf("read transition rule: %w", err)
		}

		rules = append(rules, rule)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list transition rules: %w", err)
	}

	return rules, nil
}

const insertRuleQuery = `
INSERT INTO workspace_scm_transition_rules (id, workspace_id, team_id, trigger, state_id)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (team_id, trigger) DO NOTHING`

func (r *ruleRepository) CreateMany(
	ctx context.Context,
	rules entity.SCMTransitionRules,
) error {
	for _, rule := range rules {
		if rule.ID == uuid.Nil {
			rule.ID = uuid.New()
		}

		if _, err := r.db.Querier(ctx).ExecContext(
			ctx, insertRuleQuery, rule.ID, rule.WorkspaceID, rule.TeamID, rule.Trigger, rule.StateID,
		); err != nil {
			return fmt.Errorf("seed transition rule: %w", err)
		}
	}

	return nil
}

const upsertRuleQuery = `
INSERT INTO workspace_scm_transition_rules (id, workspace_id, team_id, trigger, state_id)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (team_id, trigger) DO UPDATE
SET state_id = EXCLUDED.state_id, updated_at = now()
RETURNING` + ruleColumns

func (r *ruleRepository) Upsert(
	ctx context.Context,
	rule entity.SCMTransitionRule,
) (entity.SCMTransitionRule, error) {
	if rule.ID == uuid.Nil {
		rule.ID = uuid.New()
	}

	stored, err := scanRule(r.db.Querier(ctx).QueryRowContext(
		ctx, upsertRuleQuery, rule.ID, rule.WorkspaceID, rule.TeamID, rule.Trigger, rule.StateID,
	))
	if err != nil {
		return entity.SCMTransitionRule{}, fmt.Errorf("save transition rule: %w", err)
	}

	return stored, nil
}

const deleteRuleQuery = `
DELETE FROM workspace_scm_transition_rules
WHERE workspace_id = $1 AND team_id = $2 AND trigger = $3`

func (r *ruleRepository) Delete(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
	trigger entity.CodeChangeState,
) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx, deleteRuleQuery, workspaceID, teamID, trigger,
	)
	if err != nil {
		return fmt.Errorf("delete transition rule: %w", err)
	}

	return expectOne(result, entity.ErrSCMTransitionRuleNotFound)
}
