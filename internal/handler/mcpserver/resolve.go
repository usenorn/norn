package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

var errWorkspaceUnknown = errors.New(
	"workspace not found; call norn_list_workspaces to see the workspaces this token reaches",
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

const memberPageSize = 100

func (t *toolset) resolveAssignee(ctx context.Context, workspaceID uuid.UUID, ref string) (uuid.UUID, error) {
	wanted := strings.TrimSpace(ref)

	if strings.EqualFold(wanted, "me") {
		actor, ok := identity.Actor(ctx)
		if !ok {
			return uuid.Nil, entity.ErrAccountForbidden
		}

		return actor.Authority(), nil
	}

	if accountID, err := uuid.Parse(wanted); err == nil {
		return accountID, nil
	}

	if strings.Contains(wanted, "@") {
		return t.assigneeByEmail(ctx, workspaceID, wanted)
	}

	return uuid.Nil, refusal(
		"assignee_invalid",
		"assignee must be \"me\" for the person this connection acts for, an account id from "+
			"norn_list_workspace_members, or the exact email address of a person in this workspace",
	)
}

func (t *toolset) assigneeByEmail(ctx context.Context, workspaceID uuid.UUID, email string) (uuid.UUID, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	cursor := ""

	for {
		page, err := t.workspaces.ListMembers(ctx, workspaceID, service.ListMembersInput{
			Cursor: cursor,
			Limit:  memberPageSize,
		})
		if err != nil {
			return uuid.Nil, err
		}

		for _, member := range page.Members {
			if strings.ToLower(strings.TrimSpace(member.Email)) != normalized {
				continue
			}

			if member.AccountKind.Machine() || member.Membership.Deactivated() {
				return uuid.Nil, refusal(
					"assignee_not_assignable",
					fmt.Sprintf("%s belongs to an agent, an integration or a deactivated member, and "+
						"only an active person can be assigned an issue", normalized),
				)
			}

			return member.Membership.AccountID, nil
		}

		if page.NextCursor == "" {
			return uuid.Nil, refusal(
				"assignee_not_found",
				fmt.Sprintf("nobody in this workspace has the email address %s; call "+
					"norn_list_workspace_members to find the person", normalized),
			)
		}

		cursor = page.NextCursor
	}
}

func (t *toolset) resolveLabels(
	ctx context.Context,
	workspaceID uuid.UUID,
	refs []string,
) ([]uuid.UUID, error) {
	if len(refs) == 0 {
		return []uuid.UUID{}, nil
	}

	labels, err := t.labels.List(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	return matchLabels(labels, refs)
}

func matchLabels(labels []entity.Label, refs []string) ([]uuid.UUID, error) {
	resolved := make([]uuid.UUID, 0, len(refs))

	for _, ref := range refs {
		id, found := matchLabel(labels, ref)
		if !found {
			return nil, refusal(
				"label_not_found",
				fmt.Sprintf("no label %q in this workspace; call norn_get_workspace_structure for "+
					"its labels", strings.TrimSpace(ref)),
			)
		}

		resolved = append(resolved, id)
	}

	return resolved, nil
}

func matchLabel(labels []entity.Label, ref string) (uuid.UUID, bool) {
	if labelID, err := uuid.Parse(ref); err == nil {
		for _, label := range labels {
			if label.ID == labelID {
				return label.ID, true
			}
		}

		return uuid.Nil, false
	}

	wanted := strings.TrimSpace(ref)

	for _, label := range labels {
		if strings.EqualFold(label.Name, wanted) {
			return label.ID, true
		}
	}

	return uuid.Nil, false
}

func (t *toolset) resolveTeams(
	ctx context.Context,
	workspaceID uuid.UUID,
	refs []string,
) ([]uuid.UUID, error) {
	resolved := make([]uuid.UUID, 0, len(refs))

	for _, ref := range refs {
		team, err := t.resolveTeam(ctx, workspaceID, ref)
		if err != nil {
			return nil, err
		}

		resolved = append(resolved, team.ID)
	}

	return resolved, nil
}
