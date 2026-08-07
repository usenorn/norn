package dashboard

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceSourceControlConnections(
	ctx context.Context,
	request api.ListWorkspaceSourceControlConnectionsRequestObject,
) (api.ListWorkspaceSourceControlConnectionsResponseObject, error) {
	connections, err := h.sourceControl.List(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceSourceControlConnections200JSONResponse(
		sourceControlDTOs(connections),
	), nil
}

func (h *handler) ConnectWorkspaceSourceControl(
	ctx context.Context,
	request api.ConnectWorkspaceSourceControlRequestObject,
) (api.ConnectWorkspaceSourceControlResponseObject, error) {
	connected, err := h.sourceControl.Connect(ctx, service.ConnectSourceControlInput{
		WorkspaceID: request.WorkspaceId,
		TeamID:      optionalUUID(request.Body.TeamId),
		Provider:    entity.SCMProvider(request.Body.Provider),
		BaseURL:     optionalString(request.Body.BaseUrl),
		Repository:  request.Body.Repository,
		Token:       optionalString(request.Body.Token),
		MirrorLabel: optionalString(request.Body.MirrorLabel),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ConnectWorkspaceSourceControl201JSONResponse{
		Connection:    sourceControlDTO(connected.Connection),
		WebhookUrl:    connected.WebhookURL,
		WebhookSecret: connected.WebhookSecret,
	}, nil
}

func (h *handler) GetWorkspaceSourceControlConnection(
	ctx context.Context,
	request api.GetWorkspaceSourceControlConnectionRequestObject,
) (api.GetWorkspaceSourceControlConnectionResponseObject, error) {
	connection, err := h.sourceControl.Get(ctx, request.WorkspaceId, request.ConnectionId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceSourceControlConnection200JSONResponse(
		sourceControlDTO(connection),
	), nil
}

func (h *handler) UpdateWorkspaceSourceControlConnection(
	ctx context.Context,
	request api.UpdateWorkspaceSourceControlConnectionRequestObject,
) (api.UpdateWorkspaceSourceControlConnectionResponseObject, error) {
	connection, err := h.sourceControl.Update(
		ctx,
		request.WorkspaceId,
		request.ConnectionId,
		service.UpdateSourceControlInput{
			TeamID:      optionalUUID(request.Body.TeamId),
			ClearTeam:   optionalBool(request.Body.ClearTeam),
			MirrorLabel: optionalString(request.Body.MirrorLabel),
		},
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UpdateWorkspaceSourceControlConnection200JSONResponse(
		sourceControlDTO(connection),
	), nil
}

func (h *handler) ReplaceWorkspaceSourceControlToken(
	ctx context.Context,
	request api.ReplaceWorkspaceSourceControlTokenRequestObject,
) (api.ReplaceWorkspaceSourceControlTokenResponseObject, error) {
	connection, err := h.sourceControl.ReplaceToken(
		ctx,
		request.WorkspaceId,
		request.ConnectionId,
		optionalString(request.Body.Token),
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ReplaceWorkspaceSourceControlToken200JSONResponse(
		sourceControlDTO(connection),
	), nil
}

func (h *handler) VerifyWorkspaceSourceControlConnection(
	ctx context.Context,
	request api.VerifyWorkspaceSourceControlConnectionRequestObject,
) (api.VerifyWorkspaceSourceControlConnectionResponseObject, error) {
	connection, err := h.sourceControl.Verify(ctx, request.WorkspaceId, request.ConnectionId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.VerifyWorkspaceSourceControlConnection200JSONResponse(
		sourceControlDTO(connection),
	), nil
}

func (h *handler) DisconnectWorkspaceSourceControl(
	ctx context.Context,
	request api.DisconnectWorkspaceSourceControlRequestObject,
) (api.DisconnectWorkspaceSourceControlResponseObject, error) {
	if err := h.sourceControl.Disconnect(
		ctx,
		request.WorkspaceId,
		request.ConnectionId,
	); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DisconnectWorkspaceSourceControl204Response{}, nil
}

func (h *handler) GetTeamSourceControlSettings(
	ctx context.Context,
	request api.GetTeamSourceControlSettingsRequestObject,
) (api.GetTeamSourceControlSettingsResponseObject, error) {
	settings, err := h.sourceControl.TeamSettings(ctx, request.WorkspaceId, request.TeamId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetTeamSourceControlSettings200JSONResponse(
		teamSourceControlDTO(request.TeamId, settings),
	), nil
}

func (h *handler) SetTeamSourceControlSettings(
	ctx context.Context,
	request api.SetTeamSourceControlSettingsRequestObject,
) (api.SetTeamSourceControlSettingsResponseObject, error) {
	settings, err := h.sourceControl.SetTeamSettings(
		ctx,
		request.WorkspaceId,
		request.TeamId,
		service.SetTeamSourceControlInput{
			AdvanceOnMerge: request.Body.AdvanceOnMerge,
			MergedStateID:  optionalUUID(request.Body.MergedStateId),
			ClearState:     optionalBool(request.Body.ClearState),
		},
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SetTeamSourceControlSettings200JSONResponse(
		teamSourceControlDTO(request.TeamId, settings),
	), nil
}

func (h *handler) ListWorkspaceIssueCodeLinks(
	ctx context.Context,
	request api.ListWorkspaceIssueCodeLinksRequestObject,
) (api.ListWorkspaceIssueCodeLinksResponseObject, error) {
	links, err := h.sourceControl.Links(ctx, request.WorkspaceId, request.IssueId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceIssueCodeLinks200JSONResponse(codeLinkDTOs(links)), nil
}

func (h *handler) LinkWorkspaceIssueCode(
	ctx context.Context,
	request api.LinkWorkspaceIssueCodeRequestObject,
) (api.LinkWorkspaceIssueCodeResponseObject, error) {
	link, err := h.sourceControl.Link(
		ctx,
		request.WorkspaceId,
		request.IssueId,
		service.LinkIssueCodeInput{URL: request.Body.Url},
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.LinkWorkspaceIssueCode201JSONResponse(codeLinkDTO(link)), nil
}

func (h *handler) UnlinkWorkspaceIssueCode(
	ctx context.Context,
	request api.UnlinkWorkspaceIssueCodeRequestObject,
) (api.UnlinkWorkspaceIssueCodeResponseObject, error) {
	if err := h.sourceControl.Unlink(
		ctx,
		request.WorkspaceId,
		request.IssueId,
		request.LinkId,
	); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UnlinkWorkspaceIssueCode204Response{}, nil
}

func (h *handler) GetWorkspaceIssueMirror(
	ctx context.Context,
	request api.GetWorkspaceIssueMirrorRequestObject,
) (api.GetWorkspaceIssueMirrorResponseObject, error) {
	mirror, err := h.sourceControl.MirrorOf(ctx, request.WorkspaceId, request.IssueId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceIssueMirror200JSONResponse(issueMirrorDTO(mirror)), nil
}

func (h *handler) MirrorWorkspaceIssue(
	ctx context.Context,
	request api.MirrorWorkspaceIssueRequestObject,
) (api.MirrorWorkspaceIssueResponseObject, error) {
	mirror, err := h.sourceControl.Mirror(
		ctx,
		request.WorkspaceId,
		request.IssueId,
		service.MirrorIssueInput{
			ConnectionID: request.Body.ConnectionId,
			Reference:    request.Body.Reference,
		},
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.MirrorWorkspaceIssue200JSONResponse(issueMirrorDTO(mirror)), nil
}

func (h *handler) UnmirrorWorkspaceIssue(
	ctx context.Context,
	request api.UnmirrorWorkspaceIssueRequestObject,
) (api.UnmirrorWorkspaceIssueResponseObject, error) {
	if err := h.sourceControl.Unmirror(
		ctx,
		request.WorkspaceId,
		request.IssueId,
	); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UnmirrorWorkspaceIssue204Response{}, nil
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

func optionalBool(value *bool) bool {
	return value != nil && *value
}

func optionalUUID(value *uuid.UUID) uuid.UUID {
	if value == nil {
		return uuid.Nil
	}

	return *value
}

func (r problemResponse) VisitListWorkspaceSourceControlConnectionsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitConnectWorkspaceSourceControlResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceSourceControlConnectionResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitUpdateWorkspaceSourceControlConnectionResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitReplaceWorkspaceSourceControlTokenResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitVerifyWorkspaceSourceControlConnectionResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitDisconnectWorkspaceSourceControlResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetTeamSourceControlSettingsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSetTeamSourceControlSettingsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceIssueCodeLinksResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitLinkWorkspaceIssueCodeResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitUnlinkWorkspaceIssueCodeResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceIssueMirrorResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitMirrorWorkspaceIssueResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitUnmirrorWorkspaceIssueResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (h *handler) ListWorkspaceSourceControlDeliveries(
	ctx context.Context,
	request api.ListWorkspaceSourceControlDeliveriesRequestObject,
) (api.ListWorkspaceSourceControlDeliveriesResponseObject, error) {
	deliveries, err := h.sourceControl.Deliveries(ctx, request.WorkspaceId, request.ConnectionId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceSourceControlDeliveries200JSONResponse(
		sourceControlDeliveryDTOs(deliveries),
	), nil
}

func (r problemResponse) VisitListWorkspaceSourceControlDeliveriesResponse(
	w http.ResponseWriter,
) error {
	return r.write(w)
}
