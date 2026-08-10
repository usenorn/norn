package account

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

func (s *accountsService) SignedIn(ctx context.Context) ([]service.SignedInAccount, error) {
	held := identity.SignedIn(ctx)
	if len(held) == 0 {
		return nil, entity.ErrAccountForbidden
	}

	current, _ := identity.CurrentSession(ctx)

	accounts := make([]service.SignedInAccount, 0, len(held))
	position := make(map[uuid.UUID]int, len(held))

	for _, session := range held {
		if err := s.authorizeSelf(identity.WithSession(ctx, session), entity.ActionRead, session.AccountID); err != nil {
			continue
		}

		index, seen := position[session.AccountID]

		if !seen {
			entry, err := s.signedInAccount(ctx, session)
			if err != nil {
				return nil, err
			}

			accounts = append(accounts, entry)
			index = len(accounts) - 1
			position[session.AccountID] = index
		}

		if session.ID == current.ID {
			accounts[index].Current = true
		}

		if err := s.attachReach(ctx, &accounts[index], session); err != nil {
			return nil, err
		}
	}

	if len(accounts) == 0 {
		return nil, entity.ErrAccountForbidden
	}

	return accounts, nil
}

func (s *accountsService) signedInAccount(
	ctx context.Context,
	session entity.Session,
) (service.SignedInAccount, error) {
	account, err := s.accounts.GetByID(ctx, session.AccountID)
	if err != nil {
		return service.SignedInAccount{}, err
	}

	workspaces, err := s.workspaces.ListByAccountID(ctx, session.AccountID)
	if err != nil {
		return service.SignedInAccount{}, err
	}

	entry := service.SignedInAccount{
		Account:     account,
		DefaultSlot: session.Slot,
		Workspaces:  make([]service.SignedInWorkspace, 0, len(workspaces)),
	}

	for _, workspace := range workspaces {
		entry.Workspaces = append(entry.Workspaces, service.SignedInWorkspace{Workspace: workspace})
	}

	return entry, nil
}

// A workspace that enforces single sign-on is only reachable by the session its provider issued,
// so an account holding several sessions points each workspace at the one that can act in it.
func (s *accountsService) attachReach(
	ctx context.Context,
	entry *service.SignedInAccount,
	session entity.Session,
) error {
	actor := entity.UserActor(session)

	for index, reach := range entry.Workspaces {
		if reach.Reachable {
			continue
		}

		policy, err := s.authPolicies.Get(ctx, reach.Workspace.ID)
		if err != nil {
			return err
		}

		if policy.Enforcement.PermitsActor(actor, reach.Workspace.ID) {
			entry.Workspaces[index].Slot = session.Slot
			entry.Workspaces[index].Reachable = true

			continue
		}

		if reach.Slot == "" {
			entry.Workspaces[index].Slot = session.Slot
		}
	}

	return nil
}
