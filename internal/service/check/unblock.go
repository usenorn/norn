package check

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
)

func (s *checksService) resumeWhenClear(ctx context.Context, workspaceID, issueID uuid.UUID) {
	read, err := s.report(ctx, workspaceID, issueID)
	if err != nil {
		logging.From(ctx).WarnContext(
			ctx,
			"reading an issue's checks to see whether anything was waiting on them failed",
			"issue_id", issueID.String(),
			"error", err.Error(),
		)

		return
	}

	if read.Summary.Blocking > 0 {
		return
	}

	if err := s.jobs.EnqueueSCMResume(ctx, entity.SCMResumePayload{
		WorkspaceID: workspaceID,
		IssueID:     issueID,
	}); err != nil {
		logging.From(ctx).WarnContext(
			ctx,
			"asking source control to reconsider a deferred move failed; it stays deferred until "+
				"the issue is touched again",
			"issue_id", issueID.String(),
			"error", err.Error(),
		)
	}
}
