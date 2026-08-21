package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
)

var disclosedFailures = []error{
	entity.ErrIssueNotFound,
	entity.ErrIssueCommentNotFound,
	entity.ErrTeamNotFound,
	entity.ErrCycleNotFound,
	entity.ErrCycleClosed,
	entity.ErrProjectNotFound,
	entity.ErrWorkspaceNotFound,
	entity.ErrWorkflowStateNotFound,
	entity.ErrLabelNotFound,
	entity.ErrAccountNotFound,
	entity.ErrMembershipNotFound,
	entity.ErrSearchQueryEmpty,
	entity.ErrSearchKindUnknown,
	entity.ErrIssueCommentNotReplyable,
	entity.ErrIssueReferenceInvalid,
	errWorkspaceUnknown,
}

func toolFailure(ctx context.Context, err error) error {
	var validation entity.ValidationError
	if errors.As(err, &validation) {
		return errors.New(validation.Error())
	}

	var held entity.AgentActionHeldError
	if errors.As(err, &held) {
		return fmt.Errorf(
			"this workspace holds changes like this one until a person approves them; it is "+
				"waiting as proposal %s and will apply if they accept it. Do not retry it",
			held.ProposalID,
		)
	}

	if errors.Is(err, entity.ErrAgentRateLimited) {
		return errors.New(
			"this agent has spent its actions for the minute; wait and make the change again",
		)
	}

	if errors.Is(err, entity.ErrAgentDisabled) {
		return errors.New("this agent has been disabled; its credential no longer works")
	}

	var stale entity.IssueStaleError
	if errors.As(err, &stale) {
		return fmt.Errorf(
			"the issue changed since it was read; call norn_get_issue and retry with "+
				"expected_version=%d (conflicting fields: %s)",
			stale.Version,
			strings.Join(stale.Conflicts, ", "),
		)
	}

	for _, disclosed := range disclosedFailures {
		if errors.Is(err, disclosed) {
			return disclosed
		}
	}

	if errors.Is(err, entity.ErrAccountForbidden) {
		return errors.New(
			"this token is not permitted to do that; its permissions may not cover the change, " +
				"or the workspace or team may be outside what it was granted",
		)
	}

	logging.From(ctx).ErrorContext(ctx, "mcp tool failed", "error", err.Error())

	return errors.New("the operation failed")
}
