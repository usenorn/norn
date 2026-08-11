package check

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

type checksService struct {
	checks      repository.Check
	evidence    repository.CheckEvidence
	issues      repository.Issue
	delegations repository.IssueDelegation
	codeLinks   repository.CodeLink
	activity    repository.Activity
	authorizer  service.Authorizer
	issueWriter service.Issues
	transactor  repository.Transactor
}

func New(
	checks repository.Check,
	evidence repository.CheckEvidence,
	issues repository.Issue,
	delegations repository.IssueDelegation,
	codeLinks repository.CodeLink,
	activity repository.Activity,
	authorizer service.Authorizer,
	issueWriter service.Issues,
	transactor repository.Transactor,
) service.Checks {
	return &checksService{
		checks:      checks,
		evidence:    evidence,
		issues:      issues,
		delegations: delegations,
		codeLinks:   codeLinks,
		activity:    activity,
		authorizer:  authorizer,
		issueWriter: issueWriter,
		transactor:  transactor,
	}
}

func (s *checksService) decide(
	ctx context.Context,
	workspaceID uuid.UUID,
	action entity.Action,
) (entity.Decision, error) {
	return s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceCheck,
		Action:      action,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
}

func (s *checksService) List(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) ([]entity.Check, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return nil, err
	}

	if _, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope); err != nil {
		return nil, err
	}

	return s.checks.ListByIssue(ctx, workspaceID, issueID)
}

func (s *checksService) Add(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	input service.AddChecksInput,
) ([]entity.Check, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return nil, err
	}

	if len(input.Checks) == 0 {
		return nil, entity.NewValidationError(entity.FieldError{
			Field: "checks",
			Code:  entity.ValidationCodeRequired,
		})
	}

	for _, drafted := range input.Checks {
		if err := validateDraft(drafted); err != nil {
			return nil, err
		}
	}

	var added []entity.Check

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		issue, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
		if err != nil {
			return err
		}

		held, err := s.checks.ListByIssue(ctx, workspaceID, issueID)
		if err != nil {
			return err
		}

		if len(held)+len(input.Checks) > entity.ChecksPerIssueMax {
			return entity.ErrCheckLimitReached
		}

		late, err := s.afterDelegation(ctx, workspaceID, issueID)
		if err != nil {
			return err
		}

		approval := entity.CheckApprovalOn(decision.Actor)
		position := nextPosition(held)
		added = make([]entity.Check, 0, len(input.Checks))

		for _, drafted := range input.Checks {
			check := entity.Check{
				WorkspaceID:          workspaceID,
				IssueID:              issue.ID,
				Position:             position,
				Statement:            strings.TrimSpace(drafted.Statement),
				Method:               drafted.Method,
				Proof:                strings.TrimSpace(drafted.Proof),
				TimeLimit:            drafted.TimeLimit,
				Approval:             approval,
				AuthorKind:           decision.Actor.Kind,
				CreatedByAccountID:   decision.Actor.AccountID,
				AddedAfterDelegation: late,
			}

			if approval == entity.CheckApprovalApproved {
				now := time.Now().UTC()
				check.ApprovedByAccountID = decision.Actor.AccountID
				check.ApprovedAt = &now
			}

			stored, err := s.checks.Create(ctx, check)
			if err != nil {
				return err
			}

			if err := s.record(
				ctx, workspaceID, issue, decision, entity.ActivityKindCheckAdded, stored,
			); err != nil {
				return err
			}

			added = append(added, stored)
			position++
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return added, nil
}

func (s *checksService) Remove(ctx context.Context, workspaceID, issueID, checkID uuid.UUID) error {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return err
	}

	return s.transactor.WithTx(ctx, func(ctx context.Context) error {
		issue, held, err := s.checkOnIssue(ctx, workspaceID, issueID, checkID, decision)
		if err != nil {
			return err
		}

		if err := s.checks.Delete(ctx, workspaceID, issueID, checkID); err != nil {
			return err
		}

		return s.record(ctx, workspaceID, issue, decision, entity.ActivityKindCheckRemoved, held)
	})
}

func (s *checksService) checkOnIssue(
	ctx context.Context,
	workspaceID, issueID, checkID uuid.UUID,
	decision entity.Decision,
) (entity.Issue, entity.Check, error) {
	issue, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
	if err != nil {
		return entity.Issue{}, entity.Check{}, err
	}

	held, err := s.checks.GetByID(ctx, workspaceID, checkID)
	if err != nil {
		return entity.Issue{}, entity.Check{}, err
	}

	if held.IssueID != issueID {
		return entity.Issue{}, entity.Check{}, entity.ErrCheckNotFound
	}

	return issue, held, nil
}

func (s *checksService) afterDelegation(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) (bool, error) {
	_, err := s.delegations.Open(ctx, workspaceID, issueID)

	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, entity.ErrIssueDelegationNotFound):
		return false, nil
	default:
		return false, err
	}
}

func (s *checksService) record(
	ctx context.Context,
	workspaceID uuid.UUID,
	issue entity.Issue,
	decision entity.Decision,
	kind entity.ActivityKind,
	check entity.Check,
) error {
	return s.activity.Record(ctx, entity.Activity{
		WorkspaceID: workspaceID,
		Subject:     entity.IssueSubject(issue.ID),
		Actor:       decision.ActivityActor(),
		Kind:        kind,
		Field:       entity.ActivityFieldCheck,
		ToValue:     check.Statement,
		Version:     issue.Version,
	})
}

func nextPosition(held []entity.Check) int {
	position := 0

	for _, check := range held {
		if check.Position >= position {
			position = check.Position + 1
		}
	}

	return position
}

func validateDraft(drafted service.NewCheckInput) error {
	return entity.NewValidationError(
		entity.ValidateCheckStatement("statement", drafted.Statement),
		entity.ValidateCheckMethod("method", drafted.Method),
		entity.ValidateCheckProof("proof", drafted.Proof),
		entity.ValidateCheckTimeLimit("timeLimitSeconds", drafted.TimeLimit),
	)
}
