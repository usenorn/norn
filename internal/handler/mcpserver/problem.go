package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
)

type toolError struct {
	code    string
	message string
}

func (e toolError) Error() string {
	return e.code + ": " + e.message
}

func refusal(code, message string) error {
	return toolError{code: code, message: message}
}

type disclosure struct {
	err     error
	code    string
	message string
}

var disclosedFailures = []disclosure{
	{entity.ErrIssueNotFound, "issue_not_found", "issue not found"},
	{entity.ErrIssueCommentNotFound, "comment_not_found", "comment not found"},
	{
		entity.ErrTeamNotFound, "team_not_found",
		"team not found; call norn_get_workspace_structure for the team keys this connection can use",
	},
	{
		entity.ErrCycleNotFound, "cycle_not_found",
		"cycle not found; call norn_list_cycles for the cycles of a team",
	},
	{entity.ErrCycleClosed, "cycle_closed", "the cycle is closed and takes no more issues"},
	{
		entity.ErrProjectNotFound, "project_not_found",
		"project not found; call norn_list_projects for the project slugs of this workspace",
	},
	{
		entity.ErrProjectNotFinished, "project_not_finished",
		"the project has not been completed or cancelled",
	},
	{
		entity.ErrProjectSlugTaken, "project_slug_taken",
		"another project in this workspace already uses that address",
	},
	{
		entity.ErrProjectArchived, "project_archived",
		"the project is archived; bring it back with norn_archive_project or choose another project",
	},
	{entity.ErrWorkspaceNotFound, "workspace_not_found", errWorkspaceUnknown.Error()},
	{
		entity.ErrWorkflowStateNotFound, "state_not_found",
		"workflow state not found; call norn_get_workspace_structure for the states of the issue's team",
	},
	{
		entity.ErrLabelNotFound, "label_not_found",
		"label not found; call norn_get_workspace_structure for the labels of this workspace",
	},
	{
		entity.ErrLabelOutOfScope, "label_out_of_scope",
		"a label belongs to another team and cannot go on an issue in this team; use a " +
			"workspace-wide label or one of this team's labels from norn_get_workspace_structure",
	},
	{entity.ErrAccountNotFound, "account_not_found", "account not found"},
	{
		entity.ErrMembershipNotFound, "member_not_found",
		"that account is not a member of this workspace; call norn_list_workspace_members",
	},
	{
		entity.ErrMembershipDeactivated, "member_deactivated",
		"the person behind this connection is deactivated in this workspace",
	},
	{entity.ErrSearchQueryEmpty, "search_query_empty", "the search needs a query"},
	{entity.ErrSearchKindUnknown, "search_kind_unknown", "that is not a kind of result search returns"},
	{
		entity.ErrIssueCommentNotReplyable, "comment_not_replyable",
		"that comment cannot be replied to; reply to the comment that starts its thread",
	},
	{
		entity.ErrIssueReferenceInvalid, "issue_reference_invalid",
		"an issue reference is a team key and a number, like ENG-42",
	},
	{
		entity.ErrIssueChildrenOpen, "issue_children_open",
		"the issue still has open sub-issues; finish them first, or pass " +
			"acknowledge_open_children: true to close it anyway",
	},
	{entity.ErrIssueAlreadyOnTeam, "issue_already_on_team", "the issue is already on that team"},
	{
		entity.ErrIssueLabelsOutOfScope, "issue_labels_out_of_scope",
		"the issue carries labels the destination team cannot hold; pass drop_stranded_labels: " +
			"true to move it without them",
	},
	{
		entity.ErrIssueDestinationIncapable, "team_lacks_state_category",
		"the destination team has no state in the category the issue is in",
	},
	{
		entity.ErrIssueParentCycle, "parent_cycle",
		"an issue cannot be filed under itself or under one of its own sub-issues",
	},
	{
		entity.ErrIssueParentNotActive, "parent_not_active",
		"an issue cannot be filed under an archived or deleted issue",
	},
	{
		entity.ErrIssueParentTooDeep, "parent_too_deep",
		"filing it there would nest sub-issues deeper than allowed",
	},
	{
		entity.ErrIssueStatusTransition, "status_unchanged",
		"the issue is already in that status",
	},
	{errWorkspaceUnknown, "workspace_not_found", errWorkspaceUnknown.Error()},
}

func toolFailure(ctx context.Context, err error) error {
	var refused toolError
	if errors.As(err, &refused) {
		return refused
	}

	var validation entity.ValidationError
	if errors.As(err, &validation) {
		return refusal("invalid_input", validationSentence(validation))
	}

	var held entity.AgentActionHeldError
	if errors.As(err, &held) {
		return refusal("held_for_approval", fmt.Sprintf(
			"this workspace holds changes like this one until a person approves them; it is "+
				"waiting as proposal %s and will apply if they accept it. Do not retry it",
			held.ProposalID,
		))
	}

	if errors.Is(err, entity.ErrAgentRateLimited) {
		return refusal(
			"agent_rate_limited",
			"this agent has spent its actions for the minute; wait and make the change again",
		)
	}

	if errors.Is(err, entity.ErrAgentDisabled) {
		return refusal("agent_disabled", "this agent has been disabled; its credential no longer works")
	}

	var stale entity.IssueStaleError
	if errors.As(err, &stale) {
		return refusal("version_conflict", fmt.Sprintf(
			"the issue changed since it was read; call norn_get_issue and retry with "+
				"expected_version=%d (conflicting fields: %s)",
			stale.Version,
			strings.Join(stale.Conflicts, ", "),
		))
	}

	for _, disclosed := range disclosedFailures {
		if errors.Is(err, disclosed.err) {
			return refusal(disclosed.code, disclosed.message)
		}
	}

	var denied entity.AccessDeniedError
	if errors.As(err, &denied) {
		if sentence, named := permissionSentence(denied); named {
			return refusal("permission_denied", sentence)
		}
	}

	if errors.Is(err, entity.ErrAccountForbidden) {
		return refusal(
			"permission_denied",
			"this token is not permitted to do that; its permissions may not cover the change, "+
				"or the workspace or team may be outside what it was granted",
		)
	}

	logging.From(ctx).ErrorContext(ctx, "mcp tool failed", "error", err.Error())

	return refusal("operation_failed", "the operation failed")
}

func permissionSentence(denied entity.AccessDeniedError) (string, bool) {
	if denied.Resource == "" || denied.Action == "" {
		return "", false
	}

	permission := string(entity.NewAPIScope(denied.Resource, denied.Action))

	switch denied.Reason {
	case entity.DenyReasonTokenPermissionMissing:
		return fmt.Sprintf(
			"this connection was not granted %s, which this change needs; the person who set it "+
				"up can widen its permissions",
			permission,
		), true
	case entity.DenyReasonRoleLacksAction:
		return fmt.Sprintf(
			"the role of the person this connection acts for does not allow %s in this workspace",
			permission,
		), true
	case entity.DenyReasonTokenWorkspaceMismatch:
		return fmt.Sprintf(
			"this connection was not granted this workspace, so %s is refused here",
			permission,
		), true
	default:
		return "", false
	}
}

func validationSentence(validation entity.ValidationError) string {
	parts := make([]string, 0, len(validation.Fields))

	for _, field := range validation.Fields {
		parts = append(parts, fieldSentence(field))
	}

	sort.Strings(parts)

	return "the change was refused: " + strings.Join(parts, "; ")
}

func fieldSentence(field entity.FieldError) string {
	stable := " (" + field.Field + ": " + field.Code + ")"

	switch {
	case field.Field == "assigneeId" && field.Code == entity.ValidationCodeUnsupportedValue:
		return "the assignee must be a person who belongs to this workspace; agents and " +
			"integrations cannot be assigned" + stable
	case field.Field == "labelIds" && field.Code == entity.ValidationCodeUnsupportedValue:
		return "an issue may carry only one label from the same label group" + stable
	}

	switch field.Code {
	case entity.ValidationCodeRequired:
		return field.Field + " is required" + stable
	case entity.ValidationCodeTooShort:
		return field.Field + " is too short" + stable
	case entity.ValidationCodeTooLong:
		return field.Field + " is too long" + stable
	case entity.ValidationCodeMalformed:
		return field.Field + " is not in the expected format" + stable
	case entity.ValidationCodeOutOfRange:
		return field.Field + " is out of range" + stable
	case entity.ValidationCodeUnsupportedValue:
		return field.Field + " has a value this field does not accept" + stable
	default:
		return field.Field + " is not valid" + stable
	}
}
