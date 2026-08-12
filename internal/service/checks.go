package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=checks.go -destination=check/mock_checks.go -package=check -mock_names=Checks=MockChecks

type Checks interface {
	List(ctx context.Context, workspaceID, issueID uuid.UUID) (IssueChecks, error)
	Ledger(ctx context.Context, workspaceID, issueID uuid.UUID) (IssueChecks, error)
	Add(ctx context.Context, workspaceID, issueID uuid.UUID, input AddChecksInput) ([]entity.Check, error)
	Update(ctx context.Context, workspaceID, issueID, checkID uuid.UUID, input UpdateCheckInput) (entity.Check, error)
	Remove(ctx context.Context, workspaceID, issueID, checkID uuid.UUID) error
	Decide(ctx context.Context, workspaceID, issueID, checkID uuid.UUID, input DecideCheckInput) (entity.Check, error)
	Waive(ctx context.Context, workspaceID, issueID, checkID uuid.UUID, input WaiveCheckInput) (entity.Check, error)
	DeclareGap(ctx context.Context, workspaceID, issueID, checkID uuid.UUID, input DeclareGapInput) (DeclaredGap, error)
	Submit(ctx context.Context, workspaceID, issueID, checkID uuid.UUID, input SubmitEvidenceInput) (SubmittedEvidence, error)
	Evidence(ctx context.Context, workspaceID, issueID, checkID uuid.UUID) ([]entity.EvidenceRecord, error)
	SweepExpiry(ctx context.Context) error
}

type IssueChecks struct {
	Reports []entity.CheckReport
	Summary entity.CheckSummary
}

type SubmittedEvidence struct {
	Record entity.EvidenceRecord
	Report entity.CheckReport
}

type DeclaredGap struct {
	Check entity.Check
	Child entity.Issue
}
