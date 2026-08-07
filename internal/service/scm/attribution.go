package scm

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
)

func (s *sync) attribute(
	ctx context.Context,
	from source,
	decision entity.Decision,
	link entity.CodeLink,
) entity.Decision {
	if link.Author == "" {
		return decision
	}

	identities, err := s.identities.List(ctx, from.workspaceID())
	if err != nil {
		logWarn(ctx, "reading platform identities failed", from.repository.ID, err)

		return decision
	}

	account, mapped := identities.AccountFor(from.connection.Provider, link.Author)
	if !mapped {
		return decision
	}

	agent, err := s.agents.GetByAccountID(ctx, account)
	if err != nil {
		return decision
	}

	if agent.Disabled() {
		return decision
	}

	attributed := decision
	attributed.Actor = entity.Actor{
		Kind:           entity.ActorKindAgent,
		AccountID:      agent.AccountID,
		AgentID:        &agent.ID,
		OwnerAccountID: agent.OwnerAccountID,
		AuthMethod:     decision.Actor.AuthMethod,
	}

	return attributed
}
