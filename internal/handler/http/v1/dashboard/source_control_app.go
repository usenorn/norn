package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/sourcecontrol"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) GetWorkspaceSourceControlApplication(
	ctx context.Context,
	request api.GetWorkspaceSourceControlApplicationRequestObject,
) (api.GetWorkspaceSourceControlApplicationResponseObject, error) {
	dto := api.SourceControlApplication{
		Provider:    api.SourceControlProvider(entity.SCMProviderGitHub),
		Registered:  false,
		CanRegister: !h.sourceControlCfg.GitHubAppConfigured(),
	}

	registered, err := h.sourceControlApps.Application(
		ctx, request.WorkspaceId, entity.SCMProviderGitHub,
	)

	switch {
	case err == nil:
		dto.Registered = registered.Registered()
		dto.Slug = &registered.Slug

		install := registered.InstallURL()
		dto.InstallUrl = &install

	case errors.Is(err, entity.ErrSCMAppNotFound):

	default:
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceSourceControlApplication200JSONResponse(dto), nil
}

func (h *handler) BeginWorkspaceSourceControlAppRegistration(
	ctx context.Context,
	request api.BeginWorkspaceSourceControlAppRegistrationRequestObject,
) (api.BeginWorkspaceSourceControlAppRegistrationResponseObject, error) {
	organization := ""
	if request.Body != nil {
		organization = strings.TrimSpace(optionalString(request.Body.Organization))
	}

	base := strings.TrimRight(h.app.BaseURL, "/")

	registration, err := h.sourceControlApps.Registration(ctx, service.RegisterSCMAppInput{
		WorkspaceID:  request.WorkspaceId,
		Organization: organization,
		InstanceURL:  base,
		InstanceName: h.app.Name,
		HookURL:      base + sourcecontrol.AppDeliveryPath,
		RedirectURL:  base + sourcecontrol.RegisteredPath,
		CallbackURL:  base + sourcecontrol.ConnectedPath,
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	manifest, err := json.Marshal(registration.Manifest)
	if err != nil {
		return nil, fmt.Errorf("encode the application manifest: %w", err)
	}

	return api.BeginWorkspaceSourceControlAppRegistration200JSONResponse{
		Target:   registration.Target,
		State:    registration.State,
		Manifest: string(manifest),
	}, nil
}

func (h *handler) BeginWorkspaceSourceControlAppAuthorization(
	ctx context.Context,
	request api.BeginWorkspaceSourceControlAppAuthorizationRequestObject,
) (api.BeginWorkspaceSourceControlAppAuthorizationResponseObject, error) {
	callback := strings.TrimRight(h.app.BaseURL, "/") + sourcecontrol.ConnectedPath

	address, err := h.sourceControlApps.Authorization(ctx, request.WorkspaceId, callback)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.BeginWorkspaceSourceControlAppAuthorization200JSONResponse{Url: address}, nil
}

func (h *handler) ListWorkspaceSourceControlInstallations(
	ctx context.Context,
	request api.ListWorkspaceSourceControlInstallationsRequestObject,
) (api.ListWorkspaceSourceControlInstallationsResponseObject, error) {
	choice, err := h.sourceControlApps.Choice(
		ctx, request.WorkspaceId, request.Params.Handle,
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	dtos := make([]api.SourceControlInstallation, len(choice.Installations))

	for i, installation := range choice.Installations {
		dtos[i] = api.SourceControlInstallation{
			ExternalId:   installation.ExternalID,
			AccountLogin: installation.AccountLogin,
		}

		if installation.AccountKind != "" {
			dtos[i].AccountKind = &installation.AccountKind
		}
	}

	return api.ListWorkspaceSourceControlInstallations200JSONResponse(dtos), nil
}

func (r problemResponse) VisitGetWorkspaceSourceControlApplicationResponse(
	w http.ResponseWriter,
) error {
	return r.write(w)
}

func (r problemResponse) VisitBeginWorkspaceSourceControlAppRegistrationResponse(
	w http.ResponseWriter,
) error {
	return r.write(w)
}

func (r problemResponse) VisitBeginWorkspaceSourceControlAppAuthorizationResponse(
	w http.ResponseWriter,
) error {
	return r.write(w)
}

func (r problemResponse) VisitListWorkspaceSourceControlInstallationsResponse(
	w http.ResponseWriter,
) error {
	return r.write(w)
}

func (h *handler) ListWorkspaceSourceControlAvailableRepositories(
	ctx context.Context,
	request api.ListWorkspaceSourceControlAvailableRepositoriesRequestObject,
) (api.ListWorkspaceSourceControlAvailableRepositoriesResponseObject, error) {
	found, err := h.sourceControl.AvailableRepositories(
		ctx, request.WorkspaceId, request.ConnectionId,
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	dtos := make([]api.AvailableSourceControlRepository, len(found))

	for i, repository := range found {
		dtos[i] = api.AvailableSourceControlRepository{
			ExternalId: repository.ExternalID,
			FullName:   repository.FullName,
			Private:    pointer(repository.Private),
		}

		if repository.DefaultBranch != "" {
			dtos[i].DefaultBranch = &repository.DefaultBranch
		}
	}

	return api.ListWorkspaceSourceControlAvailableRepositories200JSONResponse(dtos), nil
}

func (r problemResponse) VisitListWorkspaceSourceControlAvailableRepositoriesResponse(
	w http.ResponseWriter,
) error {
	return r.write(w)
}
