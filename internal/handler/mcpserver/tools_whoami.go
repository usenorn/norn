package mcpserver

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
)

type whoamiInput struct {
	Workspace string `json:"workspace,omitempty" jsonschema:"a workspace slug or id, to also read that workspace's per-team rules"`
}

type teamRulesDTO struct {
	Team              string `json:"team"`
	TeamID            string `json:"teamId"`
	HoldComments      string `json:"holdComments"`
	HoldStateChanges  string `json:"holdStateChanges"`
	HoldIssueEdits    string `json:"holdIssueEdits"`
	HoldIssueCreation string `json:"holdIssueCreation"`
	HoldCheckSets     string `json:"holdCheckSets"`
}

type whoamiOutput struct {
	Kind     string         `json:"kind"`
	Name     string         `json:"name,omitempty"`
	Scopes   []string       `json:"scopes"`
	Teams    []teamRulesDTO `json:"teams,omitempty"`
	Reminder string         `json:"reminder"`
}

const whoamiReminder = "A held write is not an error and not a reason to retry: Norn records it " +
	"as a proposal and applies it as you if a person approves. A check set is always held, " +
	"whatever a team's other rules say."

func (t *toolset) whoami(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input whoamiInput,
) (*mcp.CallToolResult, whoamiOutput, error) {
	actor, ok := identity.Actor(ctx)
	if !ok {
		return nil, whoamiOutput{}, toolFailure(ctx, entity.ErrAccountForbidden)
	}

	output := whoamiOutput{
		Kind:     string(actor.Kind),
		Scopes:   actor.Scopes.Normalized().Strings(),
		Reminder: whoamiReminder,
	}

	if actor.Kind == entity.ActorKindToken {
		output.Name = actor.TokenName
	}

	if strings.TrimSpace(input.Workspace) == "" {
		return nil, output, nil
	}

	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, whoamiOutput{}, toolFailure(ctx, err)
	}

	if actor.Kind == entity.ActorKindAgent {
		output.Name = t.agentName(ctx, workspace.ID, actor)
	}

	teams, err := t.teams.List(ctx, workspace.ID, entity.TeamStatusActive)
	if err != nil {
		output.Reminder += " This connection cannot read the teams of that workspace, so which " +
			"of its writes are held is not shown here; a held write will say so when it happens."

		return nil, output, nil
	}

	for _, team := range teams {
		settings, err := t.agents.Settings(ctx, workspace.ID, team.ID)
		if err != nil {
			continue
		}

		output.Teams = append(output.Teams, teamRulesDTO{
			Team:              team.Key,
			TeamID:            team.ID.String(),
			HoldComments:      string(settings.Holds(entity.AgentActionComment)),
			HoldStateChanges:  string(settings.Holds(entity.AgentActionStateChange)),
			HoldIssueEdits:    string(settings.Holds(entity.AgentActionIssueEdit)),
			HoldIssueCreation: string(settings.Holds(entity.AgentActionIssueCreate)),
			HoldCheckSets:     string(settings.Holds(entity.AgentActionCheckSet)),
		})
	}

	return nil, output, nil
}

func (t *toolset) agentName(
	ctx context.Context,
	workspaceID uuid.UUID,
	actor entity.Actor,
) string {
	if actor.AgentID == nil {
		return ""
	}

	owned, err := t.agents.Get(ctx, workspaceID, *actor.AgentID)
	if err != nil {
		return ""
	}

	return owned.Agent.Name
}
