package execution

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func (s *executionsService) List(
	ctx context.Context,
	workspaceID uuid.UUID,
	page entity.ExecutionPage,
) ([]entity.ExecutionListing, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return nil, err
	}

	return s.executions.ListVisible(ctx, decision.Scope, page)
}
