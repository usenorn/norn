package runner

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (s *runnersService) Exchange(
	ctx context.Context,
	input service.ExchangeRunnerTokenInput,
) (service.RunnerSession, error) {
	if err := entity.NewValidationError(
		entity.ValidateRunnerNonce("nonce", input.Nonce),
		entity.ValidateRunnerAudience("audience", input.Audience),
	); err != nil {
		return service.RunnerSession{}, err
	}

	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(input.Signature))
	if err != nil {
		return service.RunnerSession{}, entity.ErrRunnerAssertionForged
	}

	held, err := s.runners.GetByRefreshHash(ctx, entity.HashRunnerSecret(input.RefreshToken))
	if err != nil {
		if errors.Is(err, entity.ErrRunnerNotFound) {
			return service.RunnerSession{}, entity.ErrRunnerCredentialInvalid
		}

		return service.RunnerSession{}, err
	}

	if held.Revoked() {
		return service.RunnerSession{}, entity.ErrRunnerRevoked
	}

	agent, err := s.agents.GetByID(ctx, held.WorkspaceID, held.AgentID)
	if err != nil {
		return service.RunnerSession{}, err
	}

	if agent.Disabled() {
		return service.RunnerSession{}, entity.ErrAgentDisabled
	}

	held.AgentName = agent.Name

	if input.RunnerID != held.ID {
		return service.RunnerSession{}, entity.ErrRunnerAssertionMismatch
	}

	assertion := entity.RunnerAssertion{
		RunnerID:  held.ID,
		Nonce:     strings.TrimSpace(input.Nonce),
		IssuedAt:  input.IssuedAt,
		Audience:  strings.TrimSpace(input.Audience),
		Signature: signature,
	}

	now := time.Now().UTC()

	if !assertion.Fresh(now, s.cfg.MaxClockSkew) {
		return service.RunnerSession{}, entity.ErrRunnerAssertionStale
	}

	if err := assertion.Verify(held.PublicKey); err != nil {
		return service.RunnerSession{}, err
	}

	fresh, err := s.sessions.ClaimNonce(ctx, held.ID, assertion.Nonce)
	if err != nil {
		return service.RunnerSession{}, err
	}

	if !fresh {
		return service.RunnerSession{}, entity.ErrRunnerAssertionReplayed
	}

	access, accessHash, err := entity.NewRunnerSecret(entity.RunnerAccessPrefix)
	if err != nil {
		return service.RunnerSession{}, err
	}

	ticket, ticketHash, err := entity.NewRunnerSecret(entity.RunnerTicketPrefix)
	if err != nil {
		return service.RunnerSession{}, err
	}

	if err := s.sessions.Grant(ctx, accessHash, held.ID, s.cfg.AccessTTL); err != nil {
		return service.RunnerSession{}, err
	}

	if err := s.sessions.IssueTicket(ctx, ticketHash, held.ID, s.cfg.TicketTTL); err != nil {
		return service.RunnerSession{}, err
	}

	if err := s.runners.RecordSeen(ctx, held.ID, now); err != nil {
		return service.RunnerSession{}, err
	}

	return service.RunnerSession{
		Runner:      held,
		AccessToken: access,
		AccessTTL:   s.cfg.AccessTTL,
		Ticket:      ticket,
		TicketTTL:   s.cfg.TicketTTL,
	}, nil
}

func (s *runnersService) Authenticate(ctx context.Context, token string) (entity.Actor, error) {
	runnerID, err := s.sessions.Resolve(ctx, entity.HashRunnerSecret(token))
	if err != nil {
		return entity.Actor{}, err
	}

	actor, _, err := s.ActorFor(ctx, runnerID)

	return actor, err
}

func (s *runnersService) ActorFor(
	ctx context.Context,
	runnerID uuid.UUID,
) (entity.Actor, entity.Runner, error) {
	held, err := s.runners.GetByID(ctx, runnerID)
	if err != nil {
		return entity.Actor{}, entity.Runner{}, err
	}

	if held.Revoked() {
		return entity.Actor{}, entity.Runner{}, entity.ErrRunnerRevoked
	}

	agent, err := s.agents.GetByID(ctx, held.WorkspaceID, held.AgentID)
	if err != nil {
		return entity.Actor{}, entity.Runner{}, err
	}

	if agent.Disabled() {
		return entity.Actor{}, entity.Runner{}, entity.ErrAgentDisabled
	}

	actor := held.Authority.Replay(entity.ActorKindAgent, agent.AccountID, "", held.WorkspaceID)

	agentID := agent.ID
	machineID := held.ID

	actor.AgentID = &agentID
	actor.AgentAllowance = agent.Allowance()
	actor.OwnerAccountID = agent.OwnerAccountID
	actor.RunnerID = &machineID

	return actor, held, nil
}
