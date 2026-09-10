package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=issue_criteria.go -destination=issuecriterion/mock_issue_criteria.go -package=issuecriterion -mock_names=IssueCriteria=MockIssueCriteria

type RecordEvidenceInput struct {
	CriterionID  string
	Kind         entity.EvidenceKind
	Label        string
	URL          string
	AttachmentID uuid.UUID
}

type IssueCriteria interface {
	List(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.AcceptanceCriterion, error)
	Record(ctx context.Context, workspaceID, issueID uuid.UUID, input RecordEvidenceInput) (entity.CriterionEvidence, error)
	Remove(ctx context.Context, workspaceID, issueID, evidenceID uuid.UUID) error
}
