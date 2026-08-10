package dashboard

import (
	"context"

	"github.com/usenorn/norn/internal/pkg/identity"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListSessions(ctx context.Context, _ api.ListSessionsRequestObject) (api.ListSessionsResponseObject, error) {
	current, ok := identity.CurrentSession(ctx)
	if !ok {
		return unauthorized(), nil
	}

	sessions, err := h.sessions.List(ctx, current.AccountID)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListSessions200JSONResponse(sessionDTOs(sessions, current.ID)), nil
}

func (h *handler) RevokeSession(ctx context.Context, request api.RevokeSessionRequestObject) (api.RevokeSessionResponseObject, error) {
	accountID, ok := h.currentAccountID(ctx)
	if !ok {
		return unauthorized(), nil
	}

	if err := h.sessions.Revoke(ctx, accountID, request.SessionId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RevokeSession204Response{}, nil
}

func (h *handler) RevokeAllSessions(ctx context.Context, _ api.RevokeAllSessionsRequestObject) (api.RevokeAllSessionsResponseObject, error) {
	accountID, ok := h.currentAccountID(ctx)
	if !ok {
		return unauthorized(), nil
	}

	if err := h.sessions.RevokeAllByAccountID(ctx, accountID); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	h.expireSessionsOf(ctx, accountID)

	return api.RevokeAllSessions204Response{}, nil
}
