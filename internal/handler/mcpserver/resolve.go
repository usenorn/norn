package mcpserver

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

var errWorkspaceUnknown = errors.New(
	"workspace not found; call norn_list_workspaces to see the workspaces this connection reaches",
)

func (t *toolset) resolveWorkspace(ctx context.Context, ref string) (entity.Workspace, error) {
	actor, ok := identity.Actor(ctx)
	if !ok {
		return entity.Workspace{}, errWorkspaceUnknown
	}

	reachable, err := t.workspaces.ListForAccount(ctx, actor.AccountID)
	if err != nil {
		return entity.Workspace{}, err
	}

	for _, workspace := range reachable {
		if !actor.ConfinedTo(workspace.ID) {
			continue
		}

		if strings.EqualFold(workspace.Slug, ref) || workspace.ID.String() == ref {
			return workspace, nil
		}
	}

	return entity.Workspace{}, errWorkspaceUnknown
}

func (t *toolset) resolveTeam(ctx context.Context, workspaceID uuid.UUID, ref string) (entity.Team, error) {
	if teamID, err := uuid.Parse(ref); err == nil {
		return t.teams.Get(ctx, workspaceID, teamID)
	}

	teams, err := t.teams.List(ctx, workspaceID, entity.TeamStatusActive)
	if err != nil {
		return entity.Team{}, err
	}

	for _, team := range teams {
		if strings.EqualFold(team.Key, ref) {
			return team, nil
		}
	}

	return entity.Team{}, entity.ErrTeamNotFound
}

func (t *toolset) resolveIssue(ctx context.Context, workspaceID uuid.UUID, ref string) (entity.Issue, error) {
	if issueID, err := uuid.Parse(ref); err == nil {
		return t.issues.Get(ctx, workspaceID, issueID)
	}

	return t.issues.GetByReference(ctx, workspaceID, strings.ToUpper(strings.TrimSpace(ref)))
}

func (t *toolset) resolveState(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
	ref string,
) (entity.WorkflowState, error) {
	states, err := t.workflowStates.List(ctx, workspaceID, teamID)
	if err != nil {
		return entity.WorkflowState{}, err
	}

	for _, state := range states {
		if state.ID.String() == ref || strings.EqualFold(state.Name, ref) {
			return state, nil
		}
	}

	return entity.WorkflowState{}, entity.ErrWorkflowStateNotFound
}

func (t *toolset) resolveProject(
	ctx context.Context,
	workspaceID uuid.UUID,
	ref string,
) (service.ProjectView, error) {
	if projectID, err := uuid.Parse(ref); err == nil {
		return t.projects.Get(ctx, workspaceID, projectID)
	}

	return t.projects.GetBySlug(ctx, workspaceID, strings.ToLower(strings.TrimSpace(ref)))
}

func resolveAssignee(ctx context.Context, ref string) (uuid.UUID, error) {
	if strings.EqualFold(ref, "me") {
		actor, ok := identity.Actor(ctx)
		if !ok {
			return uuid.Nil, entity.ErrAccountForbidden
		}

		return actor.AccountID, nil
	}

	accountID, err := uuid.Parse(ref)
	if err != nil {
		return uuid.Nil, errors.New(
			"assignee must be an account id from norn_list_workspace_members, or \"me\"",
		)
	}

	return accountID, nil
}
