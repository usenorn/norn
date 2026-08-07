package dashboard

import (
	"context"
	"net/http"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/middleware"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceSsoIdentities(
	ctx context.Context,
	request api.ListWorkspaceSsoIdentitiesRequestObject,
) (api.ListWorkspaceSsoIdentitiesResponseObject, error) {
	identities, err := h.workspaces.ListSSOIdentities(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	dtos := make([]api.SsoIdentity, len(identities))
	for i, identity := range identities {
		dtos[i] = api.SsoIdentity{
			WorkspaceId: identity.WorkspaceID,
			AccountId:   identity.AccountID,
			LinkedAt:    identity.LinkedAt,
		}
	}

	return api.ListWorkspaceSsoIdentities200JSONResponse(dtos), nil
}

func (h *handler) UnlinkWorkspaceSsoIdentity(
	ctx context.Context,
	request api.UnlinkWorkspaceSsoIdentityRequestObject,
) (api.UnlinkWorkspaceSsoIdentityResponseObject, error) {
	if err := h.workspaces.UnlinkSSOIdentity(ctx, request.WorkspaceId, request.AccountId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UnlinkWorkspaceSsoIdentity204Response{}, nil
}

func (h *handler) RedeemRecoveryCode(
	ctx context.Context,
	request api.RedeemRecoveryCodeRequestObject,
) (api.RedeemRecoveryCodeResponseObject, error) {
	client := middleware.ClientFrom(ctx)

	err := h.workspaces.RedeemRecoveryCode(ctx, service.RedeemRecoveryCodeInput{
		WorkspaceSlug: request.Body.Workspace,
		Code:          request.Body.Code,
		From:          client.IP.String(),
		FromAddress:   client.IP,
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RedeemRecoveryCode204Response{}, nil
}

func enforcementProblem(blocker entity.EnforcementBlocker, detail string) problemResponse {
	base := baseProblem(http.StatusUnprocessableEntity, detail)

	return problemResponse{
		status: http.StatusUnprocessableEntity,
		body: api.EnforcementRefusedProblem{
			Code:     api.EnforcementRefusedProblemCodeEnforcementRefused,
			Blocker:  api.EnforcementBlocker(blocker),
			Detail:   base.Detail,
			Instance: base.Instance,
			Status:   base.Status,
			Title:    base.Title,
			Type:     base.Type,
		},
	}
}

func (r problemResponse) VisitListWorkspaceSsoIdentitiesResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitUnlinkWorkspaceSsoIdentityResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRedeemRecoveryCodeResponse(w http.ResponseWriter) error {
	return r.write(w)
}
