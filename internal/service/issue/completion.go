package issue

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
)

func (s *issuesService) resumeParent(ctx context.Context, issue entity.Issue, target entity.WorkflowState) {
	if issue.ParentIssueID == uuid.Nil || target.ID == uuid.Nil || entity.OpenCategory(target.Category) {
		return
	}

	if err := s.jobs.EnqueueSCMResume(ctx, entity.SCMResumePayload{
		WorkspaceID: issue.WorkspaceID,
		IssueID:     issue.ParentIssueID,
	}); err != nil {
		logging.From(ctx).WarnContext(
			ctx,
			"asking source control to reconsider a deferred move failed; it stays deferred until "+
				"the issue is touched again",
			"issue_id", issue.ParentIssueID.String(),
			"error", err.Error(),
		)
	}
}
