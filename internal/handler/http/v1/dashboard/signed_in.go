package dashboard

import (
	"context"

	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListSignedInAccounts(
	ctx context.Context,
	_ api.ListSignedInAccountsRequestObject,
) (api.ListSignedInAccountsResponseObject, error) {
	accounts, err := h.accounts.SignedIn(ctx)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	dtos := make([]api.SignedInAccount, 0, len(accounts))

	for _, signedIn := range accounts {
		dtos = append(dtos, signedInAccountDTO(signedIn))
	}

	return api.ListSignedInAccounts200JSONResponse(dtos), nil
}

func (h *handler) SignOutEveryAccount(
	ctx context.Context,
	_ api.SignOutEveryAccountRequestObject,
) (api.SignOutEveryAccountResponseObject, error) {
	held := identity.SignedIn(ctx)

	if err := h.sessions.SignOutAll(ctx); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	for _, session := range held {
		h.expireSession(ctx, session.Slot)
	}

	return api.SignOutEveryAccount204Response{}, nil
}

func signedInAccountDTO(signedIn service.SignedInAccount) api.SignedInAccount {
	workspaces := make([]api.SignedInWorkspace, 0, len(signedIn.Workspaces))

	for _, reach := range signedIn.Workspaces {
		workspaces = append(workspaces, api.SignedInWorkspace{
			Workspace: workspaceDTO(reach.Workspace),
			Slot:      reach.Slot,
			Reachable: reach.Reachable,
		})
	}

	return api.SignedInAccount{
		Account:     accountDTO(signedIn.Account),
		DefaultSlot: signedIn.DefaultSlot,
		Workspaces:  workspaces,
		Current:     signedIn.Current,
	}
}
