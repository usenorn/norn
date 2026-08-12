package check

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
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
	notify      repository.NotificationEvent
	jobs        repository.JobProducer
	proposals   repository.AgentProposal
	agents      repository.Agent
	authorizer  service.Authorizer
	issueWriter service.Issues
	transactor  repository.Transactor
	sweepBatch  int
}

func New(
	checks repository.Check,
	evidence repository.CheckEvidence,
	issues repository.Issue,
	delegations repository.IssueDelegation,
	codeLinks repository.CodeLink,
	activity repository.Activity,
	notify repository.NotificationEvent,
	jobs repository.JobProducer,
	proposals repository.AgentProposal,
	agents repository.Agent,
	authorizer service.Authorizer,
	issueWriter service.Issues,
	transactor repository.Transactor,
	cfg config.Checks,
) service.Checks {
	return &checksService{
		checks:      checks,
		evidence:    evidence,
		issues:      issues,
		delegations: delegations,
		codeLinks:   codeLinks,
		activity:    activity,
		notify:      notify,
		jobs:        jobs,
		proposals:   proposals,
		agents:      agents,
		authorizer:  authorizer,
		issueWriter: issueWriter,
		transactor:  transactor,
		sweepBatch:  cfg.SweepBatch,
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
) (service.IssueChecks, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return service.IssueChecks{}, err
	}

	if _, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope); err != nil {
		return service.IssueChecks{}, err
	}

	return s.report(ctx, workspaceID, issueID)
}

func (s *checksService) Ledger(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) (service.IssueChecks, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return service.IssueChecks{}, err
	}

	if _, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope); err != nil {
		return service.IssueChecks{}, err
	}

	return s.assemble(ctx, workspaceID, issueID, s.evidence.ListByIssue)
}

func (s *checksService) report(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) (service.IssueChecks, error) {
	return s.assemble(ctx, workspaceID, issueID, s.evidence.Digest)
}

func (s *checksService) assemble(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	read func(context.Context, uuid.UUID, uuid.UUID) ([]entity.Evidence, error),
) (service.IssueChecks, error) {
	checks, err := s.checks.ListByIssue(ctx, workspaceID, issueID)
	if err != nil {
		return service.IssueChecks{}, err
	}

	evidence, err := read(ctx, workspaceID, issueID)
	if err != nil {
		return service.IssueChecks{}, err
	}

	horizon, err := s.horizon(ctx, workspaceID, issueID, time.Now().UTC())
	if err != nil {
		return service.IssueChecks{}, err
	}

	reports := entity.ReportChecks(checks, evidence, horizon)

	return service.IssueChecks{Reports: reports, Summary: entity.Summarise(reports)}, nil
}

func (s *checksService) horizon(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	now time.Time,
) (entity.EvidenceHorizon, error) {
	links, err := s.codeLinks.ListByIssue(ctx, workspaceID, issueID)
	if err != nil {
		return entity.EvidenceHorizon{}, err
	}

	return entity.EvidenceHorizon{Now: now, Heads: entity.HeadsOf(links)}, nil
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

	if err := entity.NewValidationError(
		entity.ValidateAgentReasoning("reasoning", input.Reasoning),
	); err != nil {
		return nil, err
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

		return s.propose(ctx, decision, issue, added, input.Reasoning)
	})
	if err != nil {
		return nil, err
	}

	return added, nil
}

func (s *checksService) Update(
	ctx context.Context,
	workspaceID, issueID, checkID uuid.UUID,
	input service.UpdateCheckInput,
) (entity.Check, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return entity.Check{}, err
	}

	if err := validateDraft(service.NewCheckInput(input)); err != nil {
		return entity.Check{}, err
	}

	var updated entity.Check

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		issue, held, err := s.checkOnIssue(ctx, workspaceID, issueID, checkID, decision)
		if err != nil {
			return err
		}

		if held.Resolved() {
			return entity.ErrCheckSettled
		}

		updated, err = s.checks.Update(ctx, workspaceID, repository.CheckUpdate{
			CheckID:   checkID,
			Statement: strings.TrimSpace(input.Statement),
			Method:    input.Method,
			Proof:     strings.TrimSpace(input.Proof),
			TimeLimit: input.TimeLimit,
		})
		if err != nil {
			return err
		}

		return s.record(ctx, workspaceID, issue, decision, entity.ActivityKindCheckEdited, updated)
	})
	if err != nil {
		return entity.Check{}, err
	}

	return updated, nil
}

func (s *checksService) Remove(ctx context.Context, workspaceID, issueID, checkID uuid.UUID) error {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return err
	}

	if decision.Actor.Kind != entity.ActorKindUser {
		return entity.ErrCheckRemovalNotPersonal
	}

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		issue, held, err := s.checkOnIssue(ctx, workspaceID, issueID, checkID, decision)
		if err != nil {
			return err
		}

		if err := s.checks.Delete(ctx, workspaceID, issueID, checkID); err != nil {
			return err
		}

		return s.record(ctx, workspaceID, issue, decision, entity.ActivityKindCheckRemoved, held)
	})
	if err != nil {
		return err
	}

	s.resumeWhenClear(ctx, workspaceID, issueID)

	return nil
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

func (s *checksService) announce(
	ctx context.Context,
	issue entity.Issue,
	decision entity.Decision,
	kind entity.NotificationKind,
) error {
	attribution := decision.ActivityActor()

	return s.notify.Record(ctx, entity.NotificationEvent{
		WorkspaceID: issue.WorkspaceID,
		Subject:     entity.NotifyIssue(issue.ID),
		Kind:        kind,
		Actor:       attribution.AccountID,
		ActorKind:   attribution.Kind,
	})
}
