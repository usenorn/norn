package check

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

func (s *checksService) Decide(
	ctx context.Context,
	workspaceID, issueID, checkID uuid.UUID,
	input service.DecideCheckInput,
) (entity.Check, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return entity.Check{}, err
	}

	if !input.Approval.Decided() {
		return entity.Check{}, entity.NewValidationError(entity.FieldError{
			Field: "approval",
			Code:  entity.ValidationCodeUnsupportedValue,
		})
	}

	if decision.Actor.Kind != entity.ActorKindUser {
		return entity.Check{}, entity.ErrCheckDecisionNotPersonal
	}

	var decided entity.Check

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		issue, held, err := s.checkOnIssue(ctx, workspaceID, issueID, checkID, decision)
		if err != nil {
			return err
		}

		if held.Approval.Decided() {
			return entity.ErrCheckDecided
		}

		decided, err = s.checks.Decide(ctx, workspaceID, repository.CheckDecision{
			CheckID:   checkID,
			Approval:  input.Approval,
			AccountID: decision.Actor.AccountID,
			DecidedAt: time.Now().UTC(),
		})
		if err != nil {
			return err
		}

		kind := entity.ActivityKindCheckApproved
		if input.Approval == entity.CheckApprovalDeclined {
			kind = entity.ActivityKindCheckDeclined
		}

		return s.record(ctx, workspaceID, issue, decision, kind, decided)
	})
	if err != nil {
		return entity.Check{}, err
	}

	return decided, nil
}

func (s *checksService) Waive(
	ctx context.Context,
	workspaceID, issueID, checkID uuid.UUID,
	input service.WaiveCheckInput,
) (entity.Check, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return entity.Check{}, err
	}

	if decision.Actor.Kind != entity.ActorKindUser {
		return entity.Check{}, entity.ErrCheckWaiverNotPersonal
	}

	if err := entity.NewValidationError(entity.ValidateCheckReason("reason", input.Reason)); err != nil {
		return entity.Check{}, err
	}

	var waived entity.Check

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		issue, held, err := s.checkOnIssue(ctx, workspaceID, issueID, checkID, decision)
		if err != nil {
			return err
		}

		if held.Resolved() {
			return entity.ErrCheckSettled
		}

		waived, err = s.checks.Resolve(ctx, workspaceID, repository.CheckResolutionInput{
			CheckID:    checkID,
			Resolution: entity.CheckResolutionWaived,
			Reason:     strings.TrimSpace(input.Reason),
			AccountID:  decision.Actor.AccountID,
			ResolvedAt: time.Now().UTC(),
		})
		if err != nil {
			return err
		}

		return s.record(ctx, workspaceID, issue, decision, entity.ActivityKindCheckWaived, waived)
	})
	if err != nil {
		return entity.Check{}, err
	}

	s.resumeWhenClear(ctx, workspaceID, issueID)

	return waived, nil
}

func (s *checksService) DeclareGap(
	ctx context.Context,
	workspaceID, issueID, checkID uuid.UUID,
	input service.DeclareGapInput,
) (service.DeclaredGap, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return service.DeclaredGap{}, err
	}

	if err := entity.NewValidationError(entity.ValidateCheckReason("reason", input.Reason)); err != nil {
		return service.DeclaredGap{}, err
	}

	var declared service.DeclaredGap

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		issue, held, err := s.checkOnIssue(ctx, workspaceID, issueID, checkID, decision)
		if err != nil {
			return err
		}

		if held.Resolved() {
			return entity.ErrCheckSettled
		}

		title := strings.TrimSpace(input.Title)
		if title == "" {
			title = entity.GapIssueTitle(held.Statement)
		}

		child, err := s.issueWriter.Create(ctx, service.CreateIssueInput{
			WorkspaceID: workspaceID,
			TeamID:      issue.TeamID,
			Title:       title,
			Description: strings.TrimSpace(input.Reason),
			ProjectID:   issue.ProjectID,
		})
		if err != nil {
			return err
		}

		child, err = s.issueWriter.SetParent(ctx, workspaceID, child.ID, service.SetIssueParentInput{
			ExpectedVersion: child.Version,
			ParentID:        &issue.ID,
		})
		if err != nil {
			return err
		}

		resolved, err := s.checks.Resolve(ctx, workspaceID, repository.CheckResolutionInput{
			CheckID:    checkID,
			Resolution: entity.CheckResolutionGap,
			Reason:     strings.TrimSpace(input.Reason),
			GapIssueID: child.ID,
			AccountID:  decision.Actor.AccountID,
			ResolvedAt: time.Now().UTC(),
		})
		if err != nil {
			return err
		}

		if err := s.record(
			ctx, workspaceID, issue, decision, entity.ActivityKindCheckGapDeclared, resolved,
		); err != nil {
			return err
		}

		if err := s.announce(ctx, issue, decision, entity.NotificationKindGapDeclared); err != nil {
			return err
		}

		declared = service.DeclaredGap{Check: resolved, Child: child}

		return nil
	})
	if err != nil {
		return service.DeclaredGap{}, err
	}

	return declared, nil
}
