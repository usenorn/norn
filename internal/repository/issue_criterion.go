package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=issue_criterion.go -destination=issuecriterion/mock_issue_criterion.go -package=issuecriterion -mock_names=IssueCriterion=MockIssueCriterion

type IssueCriterion interface {
	Record(ctx context.Context, evidence entity.CriterionEvidence) (entity.CriterionEvidence, error)
	ListForIssue(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.CriterionEvidence, error)
	CountForCriterion(ctx context.Context, workspaceID, issueID uuid.UUID, criterionID string) (int, error)
	Remove(ctx context.Context, workspaceID, issueID, evidenceID uuid.UUID) error
}
