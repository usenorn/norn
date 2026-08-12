-- +goose Up
INSERT INTO workspace_scm_transition_rules (workspace_id, team_id, trigger, state_id)
SELECT workspace_id, team_id, 'open', id
FROM (
    SELECT DISTINCT ON (team_id) workspace_id, team_id, id
    FROM workspace_workflow_states
    WHERE category = 'active'
    ORDER BY team_id, position DESC, id
) AS review
ON CONFLICT (team_id, trigger) DO NOTHING;

INSERT INTO workspace_scm_transition_rules (workspace_id, team_id, trigger, state_id)
SELECT workspace_id, team_id, 'merged', id
FROM workspace_workflow_states
WHERE is_completion
ON CONFLICT (team_id, trigger) DO NOTHING;

-- +goose Down
DELETE FROM workspace_scm_transition_rules r
USING (
    SELECT DISTINCT ON (team_id) team_id, id
    FROM workspace_workflow_states
    WHERE category = 'active'
    ORDER BY team_id, position DESC, id
) AS review
WHERE r.trigger = 'open' AND r.team_id = review.team_id AND r.state_id = review.id;

DELETE FROM workspace_scm_transition_rules r
USING workspace_workflow_states s
WHERE r.trigger = 'merged' AND r.team_id = s.team_id AND r.state_id = s.id AND s.is_completion;
