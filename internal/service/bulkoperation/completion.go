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

	return nil
}
