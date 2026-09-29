package execution

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

func (s *executionsService) authorised(
	ctx context.Context,
	decision entity.Decision,
	execution entity.Execution,
) error {
	authority, err := s.delegates.Authority(ctx, execution.WorkspaceID, execution.IssueID)
	if err != nil {
		return err
	}

	if !authority.Permits(decision) {
		return entity.ErrIssueDecisionForbidden
	}

	return nil
}
