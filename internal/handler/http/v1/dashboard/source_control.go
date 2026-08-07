package dashboard

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceSourceControlConnections(
	ctx context.Context,
	request api.ListWorkspaceSourceControlConnectionsRequestObject,
) (api.ListWorkspaceSourceControlConnectionsResponseObject, error) {
	connections, err := h.sourceControl.ListConnections(ctx, request.WorkspaceId)
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
	connection, err := h.sourceControl.Connect(ctx, service.ConnectSourceControlInput{
		WorkspaceID: request.WorkspaceId,
		Provider:    entity.SCMProvider(request.Body.Provider),
		BaseURL:     optionalString(request.Body.BaseUrl),
		Label:       optionalString(request.Body.Label),
		Token:       optionalString(request.Body.Token),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ConnectWorkspaceSourceControl201JSONResponse(
		sourceControlDTO(connection),
	), nil
}

func (h *handler) GetWorkspaceSourceControlConnection(
	ctx context.Context,
	request api.GetWorkspaceSourceControlConnectionRequestObject,
) (api.GetWorkspaceSourceControlConnectionResponseObject, error) {
	connection, err := h.sourceControl.GetConnection(
		ctx, request.WorkspaceId, request.ConnectionId,
	)
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
	connection, err := h.sourceControl.UpdateConnection(
		ctx,
		request.WorkspaceId,
		request.ConnectionId,
		service.UpdateConnectionInput{Label: optionalString(request.Body.Label)},
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
		ctx, request.WorkspaceId, request.ConnectionId, optionalString(request.Body.Token),
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
	connection, err := h.sourceControl.VerifyConnection(
		ctx, request.WorkspaceId, request.ConnectionId,
	)
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
		ctx, request.WorkspaceId, request.ConnectionId,
	); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DisconnectWorkspaceSourceControl204Response{}, nil
}

func (h *handler) ListWorkspaceSourceControlRepositories(
	ctx context.Context,
	request api.ListWorkspaceSourceControlRepositoriesRequestObject,
) (api.ListWorkspaceSourceControlRepositoriesResponseObject, error) {
	stored, err := h.sourceControl.ListRepositories(
		ctx, request.WorkspaceId, optionalUUID(request.Params.ConnectionId),
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceSourceControlRepositories200JSONResponse(
		sourceControlRepositoryDTOs(stored),
	), nil
}

func (h *handler) AddWorkspaceSourceControlRepository(
	ctx context.Context,
	request api.AddWorkspaceSourceControlRepositoryRequestObject,
) (api.AddWorkspaceSourceControlRepositoryResponseObject, error) {
	added, err := h.sourceControl.AddRepository(ctx, request.WorkspaceId, service.AddRepositoryInput{
		ConnectionID: request.Body.ConnectionId,
		FullName:     request.Body.FullName,
		MirrorLabel:  optionalString(request.Body.MirrorLabel),
		PollInterval: optionalInterval(request.Body.PollIntervalSeconds),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.AddWorkspaceSourceControlRepository201JSONResponse{
		Repository:    sourceControlRepositoryDTO(added.Repository),
		WebhookUrl:    added.WebhookURL,
		WebhookSecret: added.WebhookSecret,
	}, nil
}

func optionalInterval(seconds *int32) time.Duration {
	if seconds == nil {
		return 0
	}

	return time.Duration(*seconds) * time.Second
}

func (h *handler) UpdateWorkspaceSourceControlRepository(
	ctx context.Context,
	request api.UpdateWorkspaceSourceControlRepositoryRequestObject,
) (api.UpdateWorkspaceSourceControlRepositoryResponseObject, error) {
	stored, err := h.sourceControl.UpdateRepository(
		ctx,
		request.WorkspaceId,
		request.RepositoryId,
		service.UpdateRepositoryInput{
			MirrorLabel:  optionalString(request.Body.MirrorLabel),
			PollInterval: optionalInterval(request.Body.PollIntervalSeconds),
		},
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UpdateWorkspaceSourceControlRepository200JSONResponse(
		sourceControlRepositoryDTO(stored),
	), nil
}

func (h *handler) RemoveWorkspaceSourceControlRepository(
	ctx context.Context,
	request api.RemoveWorkspaceSourceControlRepositoryRequestObject,
) (api.RemoveWorkspaceSourceControlRepositoryResponseObject, error) {
	if err := h.sourceControl.RemoveRepository(
		ctx, request.WorkspaceId, request.RepositoryId,
	); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RemoveWorkspaceSourceControlRepository204Response{}, nil
}

func (h *handler) ListWorkspaceSourceControlDeliveries(
	ctx context.Context,
	request api.ListWorkspaceSourceControlDeliveriesRequestObject,
) (api.ListWorkspaceSourceControlDeliveriesResponseObject, error) {
	deliveries, err := h.sourceControl.Deliveries(
		ctx, request.WorkspaceId, request.RepositoryId,
	)
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

func (h *handler) ListWorkspaceSourceControlRoutes(
	ctx context.Context,
	request api.ListWorkspaceSourceControlRoutesRequestObject,
) (api.ListWorkspaceSourceControlRoutesResponseObject, error) {
	routes, err := h.sourceControl.ListRoutes(
		ctx, request.WorkspaceId, request.RepositoryId,
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceSourceControlRoutes200JSONResponse(
		sourceControlRouteDTOs(routes),
	), nil
}

func (h *handler) AddWorkspaceSourceControlRoute(
	ctx context.Context,
	request api.AddWorkspaceSourceControlRouteRequestObject,
) (api.AddWorkspaceSourceControlRouteResponseObject, error) {
	route, err := h.sourceControl.AddRoute(ctx, request.WorkspaceId, service.AddRouteInput{
		RepositoryID: request.RepositoryId,
		TeamID:       request.Body.TeamId,
		PathPrefix:   optionalString(request.Body.PathPrefix),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.AddWorkspaceSourceControlRoute201JSONResponse(
		sourceControlRouteDTO(route),
	), nil
}

func (h *handler) RemoveWorkspaceSourceControlRoute(
	ctx context.Context,
	request api.RemoveWorkspaceSourceControlRouteRequestObject,
) (api.RemoveWorkspaceSourceControlRouteResponseObject, error) {
	if err := h.sourceControl.RemoveRoute(
		ctx, request.WorkspaceId, request.RouteId,
	); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RemoveWorkspaceSourceControlRoute204Response{}, nil
}

func (h *handler) ListTeamSourceControlRules(
	ctx context.Context,
	request api.ListTeamSourceControlRulesRequestObject,
) (api.ListTeamSourceControlRulesResponseObject, error) {
	rules, err := h.sourceControl.TeamRules(ctx, request.WorkspaceId, request.TeamId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListTeamSourceControlRules200JSONResponse(transitionRuleDTOs(rules)), nil
}

func (h *handler) SetTeamSourceControlRule(
	ctx context.Context,
	request api.SetTeamSourceControlRuleRequestObject,
) (api.SetTeamSourceControlRuleResponseObject, error) {
	rules, err := h.sourceControl.SetTeamRule(
		ctx,
		request.WorkspaceId,
		request.TeamId,
		service.SetTransitionRuleInput{
			Trigger: entity.CodeChangeState(request.Body.Trigger),
			StateID: request.Body.StateId,
		},
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SetTeamSourceControlRule200JSONResponse(transitionRuleDTOs(rules)), nil
}

func (h *handler) ClearTeamSourceControlRule(
	ctx context.Context,
	request api.ClearTeamSourceControlRuleRequestObject,
) (api.ClearTeamSourceControlRuleResponseObject, error) {
	rules, err := h.sourceControl.ClearTeamRule(
		ctx,
		request.WorkspaceId,
		request.TeamId,
		entity.CodeChangeState(request.Trigger),
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ClearTeamSourceControlRule200JSONResponse(transitionRuleDTOs(rules)), nil
}

func (h *handler) ListWorkspaceIssueCodeLinks(
	ctx context.Context,
	request api.ListWorkspaceIssueCodeLinksRequestObject,
) (api.ListWorkspaceIssueCodeLinksResponseObject, error) {
	links, reviewers, err := h.sourceControl.Links(ctx, request.WorkspaceId, request.IssueId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceIssueCodeLinks200JSONResponse(codeLinkDTOs(links, reviewers)), nil
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
		ctx, request.WorkspaceId, request.IssueId, request.LinkId,
	); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UnlinkWorkspaceIssueCode204Response{}, nil
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

	return api.GetTeamSourceControlSettings200JSONResponse(teamSCMSettingsDTO(settings)), nil
}

func (h *handler) SetTeamSourceControlSettings(
	ctx context.Context,
	request api.SetTeamSourceControlSettingsRequestObject,
) (api.SetTeamSourceControlSettingsResponseObject, error) {
	settings, err := h.sourceControl.SetTeamSettings(
		ctx,
		request.WorkspaceId,
		request.TeamId,
		service.SetTeamSCMSettingsInput{
			BranchTemplate: optionalString(request.Body.BranchTemplate),
		},
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SetTeamSourceControlSettings200JSONResponse(teamSCMSettingsDTO(settings)), nil
}

func (h *handler) GetWorkspaceIssueBranchName(
	ctx context.Context,
	request api.GetWorkspaceIssueBranchNameRequestObject,
) (api.GetWorkspaceIssueBranchNameResponseObject, error) {
	branch, err := h.sourceControl.BranchName(ctx, request.WorkspaceId, request.IssueId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceIssueBranchName200JSONResponse{Branch: branch}, nil
}

func (h *handler) SuppressWorkspaceIssueAutomation(
	ctx context.Context,
	request api.SuppressWorkspaceIssueAutomationRequestObject,
) (api.SuppressWorkspaceIssueAutomationResponseObject, error) {
	if err := h.sourceControl.SuppressAutomation(
		ctx, request.WorkspaceId, request.IssueId, request.Body.Suppressed,
	); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SuppressWorkspaceIssueAutomation204Response{}, nil
}

func (h *handler) ListWorkspaceIssueMirrors(
	ctx context.Context,
	request api.ListWorkspaceIssueMirrorsRequestObject,
) (api.ListWorkspaceIssueMirrorsResponseObject, error) {
	mirrors, err := h.sourceControl.Mirrors(ctx, request.WorkspaceId, request.IssueId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceIssueMirrors200JSONResponse(issueMirrorDTOs(mirrors)), nil
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
			RepositoryID: request.Body.RepositoryId,
			Reference:    request.Body.Reference,
		},
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.MirrorWorkspaceIssue201JSONResponse(issueMirrorDTO(mirror)), nil
}

func (h *handler) UnmirrorWorkspaceIssue(
	ctx context.Context,
	request api.UnmirrorWorkspaceIssueRequestObject,
) (api.UnmirrorWorkspaceIssueResponseObject, error) {
	if err := h.sourceControl.Unmirror(
		ctx, request.WorkspaceId, request.IssueId, request.MirrorId,
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

func (r problemResponse) VisitListWorkspaceSourceControlRepositoriesResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitAddWorkspaceSourceControlRepositoryResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitUpdateWorkspaceSourceControlRepositoryResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRemoveWorkspaceSourceControlRepositoryResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceSourceControlDeliveriesResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceSourceControlRoutesResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitAddWorkspaceSourceControlRouteResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRemoveWorkspaceSourceControlRouteResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListTeamSourceControlRulesResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSetTeamSourceControlRuleResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitClearTeamSourceControlRuleResponse(w http.ResponseWriter) error {
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

func (r problemResponse) VisitGetTeamSourceControlSettingsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSetTeamSourceControlSettingsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitGetWorkspaceIssueBranchNameResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitSuppressWorkspaceIssueAutomationResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceIssueMirrorsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitMirrorWorkspaceIssueResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitUnmirrorWorkspaceIssueResponse(w http.ResponseWriter) error {
	return r.write(w)
}
