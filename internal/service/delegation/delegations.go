package delegation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type delegationsService struct {
	delegations repository.IssueDelegation
	issues      repository.Issue
	agents      repository.Agent
	runs        repository.Execution
	activity    repository.Activity
	emitter     service.WebhookEmitter
	executions  service.Executions
	authorizer  service.Authorizer
	transactor  repository.Transactor
}

func New(
	delegations repository.IssueDelegation,
	issues repository.Issue,
	agents repository.Agent,
	runs repository.Execution,
	activity repository.Activity,
	emitter service.WebhookEmitter,
	executions service.Executions,
	authorizer service.Authorizer,
	transactor repository.Transactor,
) service.Delegations {
	return &delegationsService{
		delegations: delegations,
		issues:      issues,
		agents:      agents,
		runs:        runs,
		activity:    activity,
		emitter:     emitter,
		executions:  executions,
		authorizer:  authorizer,
		transactor:  transactor,
	}
}

func (s *delegationsService) decide(
	ctx context.Context,
	workspaceID uuid.UUID,
	action entity.Action,
) (entity.Decision, error) {
	return s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      action,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
}

func (s *delegationsService) Delegate(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	input service.DelegateIssueInput,
) (entity.IssueDelegation, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return entity.IssueDelegation{}, err
	}

	if input.AgentAccountID == uuid.Nil {
		return entity.IssueDelegation{}, entity.NewValidationError(entity.FieldError{
			Field: "agentAccountId",
			Code:  entity.ValidationCodeRequired,
		})
	}

	fields := append(
		[]entity.FieldError{entity.ValidateIssueBrief("brief", input.Brief)},
		entity.ValidateDelegationParams("params", input.Params)...,
	)

	if err := entity.NewValidationError(fields...); err != nil {
		return entity.IssueDelegation{}, err
	}

	var (
		delegated entity.IssueDelegation
		issue     entity.Issue
	)

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		issue, err = s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
		if err != nil {
			return err
		}

		if issue.AssigneeAccountID == uuid.Nil {
			return entity.ErrIssueDelegationUnassigned
		}

		agent, err := s.agents.GetByAccountID(ctx, input.AgentAccountID)
		if err != nil {
			return err
		}

		if agent.WorkspaceID != workspaceID {
			return entity.ErrAgentNotFound
		}

		if agent.Disabled() {
			return entity.ErrIssueDelegationAgentUnusable
		}

		if err := s.supersede(ctx, workspaceID, issue.ID, decision); err != nil {
			return err
		}

		delegated, err = s.delegations.Delegate(ctx, entity.IssueDelegation{
			WorkspaceID:          workspaceID,
			IssueID:              issue.ID,
			AgentID:              agent.ID,
			Brief:                strings.TrimSpace(input.Brief),
			Params:               input.Params,
			DelegatedByAccountID: decision.Actor.AccountID,
			DelegatedAt:          time.Now().UTC(),
		})
		if err != nil {
			return err
		}

		if err := s.activity.Record(ctx, entity.Activity{
			WorkspaceID: workspaceID,
			Subject:     entity.IssueSubject(issue.ID),
			Actor:       decision.ActivityActor(),
			Kind:        entity.ActivityKindDelegated,
			Field:       entity.ActivityFieldAgent,
			ToValue:     delegated.AgentName,
			Version:     issue.Version,
		}); err != nil {
			return err
		}

		return s.emit(ctx, issue, delegated, decision)
	})
	if err != nil {
		return entity.IssueDelegation{}, err
	}

	if err := s.executions.OnDelegated(ctx, issue, delegated); err != nil {
		return entity.IssueDelegation{}, err
	}

	return delegated, nil
}

func (s *delegationsService) supersede(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	decision entity.Decision,
) error {
	held, err := s.delegations.Open(ctx, workspaceID, issueID)
	if err != nil {
		if errors.Is(err, entity.ErrIssueDelegationNotFound) {
			return nil
		}

		return err
	}

	now := time.Now().UTC()

	run, err := s.runs.LiveByDelegation(ctx, held.ID)

	switch {
	case err == nil && run.Underway(now):
		return entity.ErrIssueDelegationHeld
	case err != nil && !errors.Is(err, entity.ErrExecutionNotFound):
		return err
	}

	_, err = s.delegations.Recall(ctx, workspaceID, repository.RecallDelegation{
		IssueID:    issueID,
		AccountID:  decision.Actor.AccountID,
		RecalledAt: now,
	})

	return err
}

func (s *delegationsService) Targets(
	ctx context.Context,
	workspaceID, issueID, agentAccountID uuid.UUID,
) (service.DelegationTargets, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return service.DelegationTargets{}, err
	}

	if agentAccountID == uuid.Nil {
		return service.DelegationTargets{}, entity.NewValidationError(entity.FieldError{
			Field: "agentAccountId",
			Code:  entity.ValidationCodeRequired,
		})
	}

	issue, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
	if err != nil {
		return service.DelegationTargets{}, err
	}

	agent, err := s.agents.GetByAccountID(ctx, agentAccountID)
	if err != nil {
		return service.DelegationTargets{}, err
	}

	if agent.WorkspaceID != workspaceID {
		return service.DelegationTargets{}, entity.ErrAgentNotFound
	}

	placement, err := s.executions.Placement(ctx, issue, agent.ID)
	if err != nil {
		return service.DelegationTargets{}, err
	}

	return service.DelegationTargets{Agent: agent, Placement: placement}, nil
}

func (s *delegationsService) Recall(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) (entity.IssueDelegation, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return entity.IssueDelegation{}, err
	}

	var recalled entity.IssueDelegation

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		issue, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
		if err != nil {
			return err
		}

		recalled, err = s.delegations.Recall(ctx, workspaceID, repository.RecallDelegation{
			IssueID:    issue.ID,
			AccountID:  decision.Actor.AccountID,
			RecalledAt: time.Now().UTC(),
		})
		if err != nil {
			return err
		}

		return s.activity.Record(ctx, entity.Activity{
			WorkspaceID: workspaceID,
			Subject:     entity.IssueSubject(issue.ID),
			Actor:       decision.ActivityActor(),
			Kind:        entity.ActivityKindRecalled,
			Field:       entity.ActivityFieldAgent,
			FromValue:   recalled.AgentName,
			Version:     issue.Version,
		})
	})
	if err != nil {
		return entity.IssueDelegation{}, err
	}

	return recalled, nil
}

func (s *delegationsService) History(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) ([]entity.IssueDelegation, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return nil, err
	}

	if _, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope); err != nil {
		return nil, err
	}

	return s.delegations.ListByIssue(ctx, workspaceID, issueID)
}

func (s *delegationsService) emit(
	ctx context.Context,
	issue entity.Issue,
	delegation entity.IssueDelegation,
	decision entity.Decision,
) error {
	body, err := json.Marshal(service.WebhookDelegation(issue, delegation))
	if err != nil {
		return fmt.Errorf("encode %s payload: %w", entity.WebhookIssueDelegated, err)
	}

	return s.emitter.Emit(ctx, entity.WebhookOutboxEntry{
		WorkspaceID: issue.WorkspaceID,
		Event:       entity.WebhookIssueDelegated,
		SubjectKind: string(entity.ResourceIssue),
		SubjectID:   issue.ID,
		TeamID:      issue.TeamID,
		ActorID:     decision.Actor.AccountID,
		ActorKind:   decision.Actor.Kind,
		Body:        body,
	})
}
