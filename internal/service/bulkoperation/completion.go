package bulkoperation

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

func (s *operationsService) completable(
	ctx context.Context,
	action entity.BulkAction,
	decision entity.Decision,
	issue entity.Issue,
	target entity.WorkflowState,
) error {
	if target.Category != entity.StateCategoryComplete ||
		issue.State.Category == entity.StateCategoryComplete {
		return nil
	}

	children, err := s.issues.ListChildren(ctx, action.WorkspaceID, issue.ID, decision.Scope)
	if err != nil {
		return err
	}

	if open := entity.OpenIssues(children); len(open) > 0 {
		return entity.IssueChildrenOpenError{Children: open}
	}

	blocking, err := s.checks.Blocking(ctx, action.WorkspaceID, issue.ID)
	if err != nil {
		return err
	}

	if len(blocking) == 0 {
		return nil
	}

	if decision.Actor.Kind == entity.ActorKindAgent {
		return entity.IssueChecksUnprovenError{Checks: blocking}
	}

	return s.record(ctx, action, decision, issue, entity.Activity{
		Kind:    entity.ActivityKindChecksOverridden,
		Field:   entity.ActivityFieldChecks,
		ToValue: entity.CheckStatements(blocking),
	})
}
