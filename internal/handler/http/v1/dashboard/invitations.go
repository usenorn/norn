package dashboard

import (
	"context"
	"errors"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/middleware"
	"github.com/usenorn/norn/internal/pkg/httpcookie"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceInvitations(ctx context.Context, request api.ListWorkspaceInvitationsRequestObject) (api.ListWorkspaceInvitationsResponseObject, error) {
	if _, ok := h.currentAccountID(ctx); !ok {
		return unauthorized(), nil
	}

	var status entity.InvitationStatus
	if request.Params.Status != nil {
		status = entity.InvitationStatus(*request.Params.Status)
	}

	invitations, err := h.invitations.List(ctx, request.WorkspaceId, status)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceInvitations200JSONResponse(invitationDTOs(invitations)), nil
}

func (h *handler) CreateWorkspaceInvitations(ctx context.Context, request api.CreateWorkspaceInvitationsRequestObject) (api.CreateWorkspaceInvitationsResponseObject, error) {
	if _, ok := h.currentAccountID(ctx); !ok {
		return unauthorized(), nil
	}

	recipients := make([]service.InvitationRecipient, len(request.Body.Invitations))
	for i, invitation := range request.Body.Invitations {
		recipients[i] = service.InvitationRecipient{
			Email: invitation.Email,
			Role:  entity.MembershipRole(invitation.Role),
		}

		if invitation.TeamIds != nil {
			recipients[i].TeamIDs = *invitation.TeamIds
		}
	}

	results, err := h.invitations.Create(ctx, service.CreateInvitationsInput{
		WorkspaceID: request.WorkspaceId,
		Recipients:  recipients,
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.CreateWorkspaceInvitations201JSONResponse{Results: invitationResultDTOs(results)}, nil
}

func (h *handler) RevokeWorkspaceInvitation(ctx context.Context, request api.RevokeWorkspaceInvitationRequestObject) (api.RevokeWorkspaceInvitationResponseObject, error) {
	if _, ok := h.currentAccountID(ctx); !ok {
		return unauthorized(), nil
	}

	if err := h.invitations.Revoke(ctx, request.WorkspaceId, request.InvitationId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RevokeWorkspaceInvitation204Response{}, nil
}

func (h *handler) ResendWorkspaceInvitation(ctx context.Context, request api.ResendWorkspaceInvitationRequestObject) (api.ResendWorkspaceInvitationResponseObject, error) {
	if _, ok := h.currentAccountID(ctx); !ok {
		return unauthorized(), nil
	}

	issued, err := h.invitations.Resend(ctx, request.WorkspaceId, request.InvitationId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ResendWorkspaceInvitation200JSONResponse{
		Invitation: invitationDTO(issued.Invitation),
	}, nil
}

func (h *handler) PreviewInvitation(ctx context.Context, request api.PreviewInvitationRequestObject) (api.PreviewInvitationResponseObject, error) {
	preview, err := h.invitations.Preview(ctx, request.Body.Token)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.PreviewInvitation200JSONResponse(invitationPreviewDTO(preview)), nil
}

func (h *handler) AcceptInvitation(ctx context.Context, request api.AcceptInvitationRequestObject) (api.AcceptInvitationResponseObject, error) {
	input := service.AcceptInvitationInput{
		Token:  request.Body.Token,
		Client: middleware.ClientFrom(ctx),
	}

	if request.Body.DisplayName != nil {
		input.DisplayName = *request.Body.DisplayName
	}

	if request.Body.Timezone != nil {
		input.Timezone = *request.Body.Timezone
	}

	if request.Body.Password != nil {
		input.Password = *request.Body.Password
	}

	accepted, err := h.invitations.Accept(ctx, input)
	if err != nil {
		if errors.Is(err, entity.ErrAccountEmailTaken) {
			return invitationUnusableProblem(api.InvitationUnusableProblemCodeAccountExists, err), nil
		}

		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	response := api.AcceptInvitation200JSONResponse{
		Workspace:  workspaceDTO(accepted.Workspace),
		Membership: membershipDTO(entity.WorkspaceMember{Membership: accepted.Membership}),
	}

	if accepted.SignedIn {
		httpcookie.Pending(ctx).Add(
			middleware.IssuedSessionCookie(h.session, accepted.Session.Session, accepted.Session.Token),
		)

		response.Slot = &accepted.Session.Session.Slot
	}

	return response, nil
}
