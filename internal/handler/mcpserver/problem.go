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
			"this connection is not permitted to do that; its consent may be read-only or " +
				"narrowed",
		)
	}

	logging.From(ctx).ErrorContext(ctx, "mcp tool failed", "error", err.Error())

	return errors.New("the operation failed")
}
