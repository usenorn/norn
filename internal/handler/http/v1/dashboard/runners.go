package dashboard

import (
	"context"

	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) EnrolRunner(
	ctx context.Context,
	request api.EnrolRunnerRequestObject,
) (api.EnrolRunnerResponseObject, error) {
	input := service.EnrolRunnerInput{
		PublicKey: request.Body.PublicKey,
		Host:      runnerHostOf(request.Body.Host),
	}

	if request.Body.Name != nil {
		input.Name = *request.Body.Name
	}

	enrolled, err := h.runners.Enrol(ctx, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.EnrolRunner201JSONResponse{
		Runner:       runnerDTO(enrolled.Runner),
		RefreshToken: enrolled.RefreshToken,
	}, nil
}

func (h *handler) ExchangeRunnerToken(
	ctx context.Context,
	request api.ExchangeRunnerTokenRequestObject,
) (api.ExchangeRunnerTokenResponseObject, error) {
	session, err := h.runners.Exchange(ctx, service.ExchangeRunnerTokenInput{
		RefreshToken: request.Body.RefreshToken,
		RunnerID:     request.Body.RunnerId,
		Nonce:        request.Body.Nonce,
		IssuedAt:     request.Body.IssuedAt,
		Audience:     request.Body.Audience,
		Signature:    request.Body.Signature,
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ExchangeRunnerToken200JSONResponse{
		Runner:          runnerDTO(session.Runner),
		AccessToken:     session.AccessToken,
		ExpiresIn:       int32(session.AccessTTL.Seconds()),
		Ticket:          session.Ticket,
		TicketExpiresIn: int32(session.TicketTTL.Seconds()),
	}, nil
}

func (h *handler) GetCurrentRunner(
	ctx context.Context,
	_ api.GetCurrentRunnerRequestObject,
) (api.GetCurrentRunnerResponseObject, error) {
	runner, err := h.runners.Self(ctx)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetCurrentRunner200JSONResponse(runnerDTO(runner)), nil
}

func (h *handler) ListWorkspaceRunners(
	ctx context.Context,
	request api.ListWorkspaceRunnersRequestObject,
) (api.ListWorkspaceRunnersResponseObject, error) {
	runners, err := h.runners.List(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceRunners200JSONResponse(runnerDTOs(runners)), nil
}

func (h *handler) RevokeWorkspaceRunner(
	ctx context.Context,
	request api.RevokeWorkspaceRunnerRequestObject,
) (api.RevokeWorkspaceRunnerResponseObject, error) {
	if err := h.runners.Revoke(ctx, request.WorkspaceId, request.RunnerId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RevokeWorkspaceRunner204Response{}, nil
}
