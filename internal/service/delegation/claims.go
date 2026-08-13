package delegation

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

func (s *delegationsService) Queue(
	ctx context.Context,
	workspaceID uuid.UUID,
) ([]service.DelegatedWork, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return nil, err
	}

	agentID, err := callingAgent(decision)
	if err != nil {
		return nil, err
	}

	delegations, err := s.delegations.ListOpenByAgent(ctx, workspaceID, agentID)
	if err != nil {
		return nil, err
	}

	queue := make([]service.DelegatedWork, 0, len(delegations))

	for _, delegation := range delegations {
		issue, err := s.issues.GetVisible(ctx, workspaceID, delegation.IssueID, decision.Scope)
		if err != nil {
			if errors.Is(err, entity.ErrIssueNotFound) {
				continue
			}

			return nil, err
		}

		if issue.Status != entity.IssueStatusActive {
			continue
		}

		queue = append(queue, service.DelegatedWork{Delegation: delegation, Issue: issue})
	}

	slices.SortFunc(queue, func(a, b service.DelegatedWork) int {
		return entity.CompareIssueQueueOrder(a.Issue, b.Issue)
	})

	return queue, nil
}

func (s *delegationsService) Claim(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	input service.ClaimDelegationInput,
) (entity.IssueDelegation, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return entity.IssueDelegation{}, err
	}

	ttl := entity.DelegationClaimTTL(input.TTL)

	if err := entity.NewValidationError(
		entity.ValidateRunnerName("runner", input.Runner),
		entity.ValidateDelegationClaimTTL("ttlSeconds", ttl),
	); err != nil {
		return entity.IssueDelegation{}, err
	}

	agentID, err := callingAgent(decision)
	if err != nil {
		return entity.IssueDelegation{}, err
	}

	if _, err := s.held(ctx, workspaceID, issueID, decision, agentID); err != nil {
		return entity.IssueDelegation{}, err
	}

	claimedAt := time.Now().UTC()

	return s.delegations.Claim(ctx, workspaceID, repository.ClaimDelegation{
		IssueID:   issueID,
		AgentID:   agentID,
		Runner:    strings.TrimSpace(input.Runner),
		Token:     uuid.New(),
		ClaimedAt: claimedAt,
		ExpiresAt: claimedAt.Add(ttl),
	})
}

func (s *delegationsService) Heartbeat(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	input service.HeartbeatDelegationInput,
) (entity.IssueDelegation, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return entity.IssueDelegation{}, err
	}

	ttl := entity.DelegationClaimTTL(input.TTL)

	if err := entity.NewValidationError(
		entity.ValidateDelegationClaimTTL("ttlSeconds", ttl),
	); err != nil {
		return entity.IssueDelegation{}, err
	}

	agentID, err := callingAgent(decision)
	if err != nil {
		return entity.IssueDelegation{}, err
	}

	if _, err := s.held(ctx, workspaceID, issueID, decision, agentID); err != nil {
		return entity.IssueDelegation{}, err
	}

	return s.delegations.Heartbeat(ctx, workspaceID, repository.HeartbeatDelegation{
		IssueID:   issueID,
		Token:     input.Token,
		ExpiresAt: time.Now().UTC().Add(ttl),
	})
}

func (s *delegationsService) ReleaseClaim(
	ctx context.Context,
	workspaceID, issueID, token uuid.UUID,
) (entity.IssueDelegation, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return entity.IssueDelegation{}, err
	}

	agentID, err := callingAgent(decision)
	if err != nil {
		return entity.IssueDelegation{}, err
	}

	if _, err := s.held(ctx, workspaceID, issueID, decision, agentID); err != nil {
		return entity.IssueDelegation{}, err
	}

	return s.delegations.ReleaseClaim(ctx, workspaceID, repository.ReleaseDelegation{
		IssueID: issueID,
		Token:   token,
	})
}

// Reading the delegation before writing the claim is what separates "there is nothing here for
// you" from "somebody else got here first": once this has passed, an empty result from the claim
// statement can only mean another runner holds it.
func (s *delegationsService) held(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	decision entity.Decision,
	agentID uuid.UUID,
) (entity.IssueDelegation, error) {
	if _, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope); err != nil {
		return entity.IssueDelegation{}, err
	}

	delegation, err := s.delegations.Open(ctx, workspaceID, issueID)
	if err != nil {
		return entity.IssueDelegation{}, err
	}

	if delegation.AgentID != agentID {
		return entity.IssueDelegation{}, entity.ErrIssueDelegationNotYours
	}

	return delegation, nil
}

func callingAgent(decision entity.Decision) (uuid.UUID, error) {
	if decision.Actor.Kind != entity.ActorKindAgent || decision.Actor.AgentID == nil {
		return uuid.Nil, entity.ErrDelegationQueueNotAgent
	}

	return *decision.Actor.AgentID, nil
}
