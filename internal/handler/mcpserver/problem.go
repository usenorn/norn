package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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
	entity.ErrCheckNotFound,
	entity.ErrCheckSettled,
	entity.ErrCheckDeclined,
	entity.ErrCheckLimitReached,
	entity.ErrEvidenceEmpty,
	errWorkspaceUnknown,
}

func blockingCount(blocking int) string {
	if blocking == 1 {
		return "one of its criteria is not proven"
	}

	return strconv.Itoa(blocking) + " of its criteria are not proven"
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

	var unproven entity.IssueChecksUnprovenError
	if errors.As(err, &unproven) {
		return fmt.Errorf(
			"this issue is not finished: %s. Do not retry the move. Either prove what is left "+
				"with norn_submit_evidence, or say plainly in a comment that you cannot, so a "+
				"person can waive the criterion or record it as a gap. Unproven criteria: %s",
			blockingCount(len(unproven.Checks)),
			entity.CheckStatements(unproven.Checks),
		)
	}

	var unratified entity.IssueChecksUnratifiedError
	if errors.As(err, &unratified) {
		return fmt.Errorf(
			"nobody has approved what this issue is graded against, so it cannot be finished "+
				"yet. Stop and wait for a person to approve the criteria; do not retry the "+
				"move. Waiting on approval: %s",
			entity.CheckStatements(unratified.Checks),
		)
	}

	if errors.Is(err, entity.ErrCheckDecisionNotPersonal) ||
		errors.Is(err, entity.ErrCheckWaiverNotPersonal) ||
		errors.Is(err, entity.ErrCheckRemovalNotPersonal) {
		return errors.New(
			"only a person can approve, waive, or remove a criterion; an agent cannot settle " +
				"what it is graded against. Ask in a comment instead",
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
