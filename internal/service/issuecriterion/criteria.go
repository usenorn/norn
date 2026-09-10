package issuecriterion

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type issueCriteriaService struct {
	evidence   repository.IssueCriterion
	issues     repository.Issue
	revisions  repository.IssueRevision
	authorizer service.Authorizer
}

func New(
	evidence repository.IssueCriterion,
	issues repository.Issue,
	revisions repository.IssueRevision,
	authorizer service.Authorizer,
) service.IssueCriteria {
	return &issueCriteriaService{
		evidence:   evidence,
		issues:     issues,
		revisions:  revisions,
		authorizer: authorizer,
	}
}

func (s *issueCriteriaService) reachable(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	action entity.Action,
) (entity.Decision, entity.Issue, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      action,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return entity.Decision{}, entity.Issue{}, err
	}

	issue, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
	if err != nil {
		return entity.Decision{}, entity.Issue{}, err
	}

	return decision, issue, nil
}

func (s *issueCriteriaService) List(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) ([]entity.AcceptanceCriterion, error) {
	_, issue, err := s.reachable(ctx, workspaceID, issueID, entity.ActionRead)
	if err != nil {
		return nil, err
	}

	evidence, err := s.evidence.ListForIssue(ctx, workspaceID, issueID)
	if err != nil {
		return nil, err
	}

	return entity.WithEvidence(entity.AcceptanceCriteria(issue.DescriptionDoc), evidence), nil
}

func (s *issueCriteriaService) Record(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	input service.RecordEvidenceInput,
) (entity.CriterionEvidence, error) {
	decision, issue, err := s.reachable(ctx, workspaceID, issueID, entity.ActionManage)
	if err != nil {
		return entity.CriterionEvidence{}, err
	}

	if err := entity.NewValidationError(
		entity.ValidateEvidence(input.Kind, input.Label, input.URL)...,
	); err != nil {
		return entity.CriterionEvidence{}, err
	}

	criterion, found := named(entity.AcceptanceCriteria(issue.DescriptionDoc), input.CriterionID)
	if !found {
		return entity.CriterionEvidence{}, entity.ErrCriterionNotFound
	}

	counted, err := s.evidence.CountForCriterion(ctx, workspaceID, issueID, input.CriterionID)
	if err != nil {
		return entity.CriterionEvidence{}, err
	}

	if counted >= entity.EvidencePerCriterionMax {
		return entity.CriterionEvidence{}, entity.ErrTooMuchEvidence
	}

	return s.evidence.Record(ctx, entity.CriterionEvidence{
		WorkspaceID:           workspaceID,
		IssueID:               issueID,
		CriterionID:           input.CriterionID,
		CriterionText:         criterion.Text,
		Kind:                  input.Kind,
		Label:                 strings.TrimSpace(input.Label),
		URL:                   strings.TrimSpace(input.URL),
		AttachmentID:          input.AttachmentID,
		DescriptionRevisionID: s.against(ctx, issue),
		RecordedByAccountID:   decision.Actor.AccountID,
		RecordedAt:            time.Now().UTC(),
	})
}

func (s *issueCriteriaService) Remove(
	ctx context.Context,
	workspaceID, issueID, evidenceID uuid.UUID,
) error {
	if _, _, err := s.reachable(ctx, workspaceID, issueID, entity.ActionManage); err != nil {
		return err
	}

	return s.evidence.Remove(ctx, workspaceID, issueID, evidenceID)
}

func (s *issueCriteriaService) against(ctx context.Context, issue entity.Issue) uuid.UUID {
	held, err := s.revisions.Latest(ctx, issue.WorkspaceID, issue.ID)
	if err != nil {
		if !errors.Is(err, entity.ErrIssueRevisionNotFound) {
			return uuid.Nil
		}

		return uuid.Nil
	}

	return held.ID
}

func named(
	criteria []entity.AcceptanceCriterion,
	id string,
) (entity.AcceptanceCriterion, bool) {
	for _, criterion := range criteria {
		if criterion.ID == id {
			return criterion, true
		}
	}

	return entity.AcceptanceCriterion{}, false
}
