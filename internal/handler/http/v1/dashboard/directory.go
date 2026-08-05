package dashboard

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/scim"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) GetWorkspaceDirectory(
	ctx context.Context,
	request api.GetWorkspaceDirectoryRequestObject,
) (api.GetWorkspaceDirectoryResponseObject, error) {
	settings, err := h.directories.Settings(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceDirectory200JSONResponse(directorySettingsDTO(settings)), nil
}

func (h *handler) ConnectWorkspaceDirectory(
	ctx context.Context,
	request api.ConnectWorkspaceDirectoryRequestObject,
) (api.ConnectWorkspaceDirectoryResponseObject, error) {
	settings, err := h.directories.Connect(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ConnectWorkspaceDirectory201JSONResponse(directorySettingsDTO(settings)), nil
}

func (h *handler) RotateWorkspaceDirectoryToken(
	ctx context.Context,
	request api.RotateWorkspaceDirectoryTokenRequestObject,
) (api.RotateWorkspaceDirectoryTokenResponseObject, error) {
	settings, err := h.directories.RotateToken(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RotateWorkspaceDirectoryToken200JSONResponse(directorySettingsDTO(settings)), nil
}

func (h *handler) ConfigureWorkspaceDirectory(
	ctx context.Context,
	request api.ConfigureWorkspaceDirectoryRequestObject,
) (api.ConfigureWorkspaceDirectoryResponseObject, error) {
	settings, err := h.directories.Configure(ctx, request.WorkspaceId, entity.DirectoryConnection{
		Enabled:    request.Body.Enabled,
		OnUnknown:  entity.DirectoryUnknownPolicy(request.Body.OnUnknown),
		OnAbsent:   entity.DirectoryAbsentPolicy(request.Body.OnAbsent),
		AdminGroup: request.Body.AdminGroup,
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ConfigureWorkspaceDirectory200JSONResponse(directorySettingsDTO(settings)), nil
}

func (h *handler) DisconnectWorkspaceDirectory(
	ctx context.Context,
	request api.DisconnectWorkspaceDirectoryRequestObject,
) (api.DisconnectWorkspaceDirectoryResponseObject, error) {
	if err := h.directories.Disconnect(ctx, request.WorkspaceId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DisconnectWorkspaceDirectory204Response{}, nil
}

func (h *handler) GetWorkspaceDirectoryAvailability(
	ctx context.Context,
	_ api.GetWorkspaceDirectoryAvailabilityRequestObject,
) (api.GetWorkspaceDirectoryAvailabilityResponseObject, error) {
	available := h.directories.Availability(ctx)

	dto := api.DirectoryAvailability{Available: available.Available}

	return api.GetWorkspaceDirectoryAvailability200JSONResponse(dto), nil
}

func (h *handler) ListWorkspaceDirectoryRuns(
	ctx context.Context,
	request api.ListWorkspaceDirectoryRunsRequestObject,
) (api.ListWorkspaceDirectoryRunsResponseObject, error) {
	page := entity.DirectoryRunPage{Cursor: request.Params.Cursor}

	if request.Params.Limit != nil {
		page.Limit = int(*request.Params.Limit)
	}

	page = page.Normalized()

	runs, err := h.directories.Runs(ctx, request.WorkspaceId, page.Lookahead())
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	dto := api.DirectoryRunPage{Runs: directoryRunDTOs(runs)}

	if len(runs) > page.Limit {
		dto.Runs = directoryRunDTOs(runs[:page.Limit])
		cursor := runs[page.Limit-1].StartedAt
		dto.NextCursor = &cursor
	}

	return api.ListWorkspaceDirectoryRuns200JSONResponse(dto), nil
}

func (h *handler) ListWorkspaceDirectoryChanges(
	ctx context.Context,
	request api.ListWorkspaceDirectoryChangesRequestObject,
) (api.ListWorkspaceDirectoryChangesResponseObject, error) {
	changes, err := h.directories.Changes(ctx, request.WorkspaceId, request.RunId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceDirectoryChanges200JSONResponse(directoryChangeDTOs(changes)), nil
}

func directorySettingsDTO(settings service.DirectorySettings) api.DirectorySettings {
	dto := api.DirectorySettings{
		Connected:   settings.Connected,
		ScimBaseUrl: scim.BasePath,
	}

	if settings.Token != "" {
		dto.Token = &settings.Token
	}

	if !settings.Connected {
		return dto
	}

	connection := api.DirectoryConnection{
		Enabled:    settings.Connection.Enabled,
		OnUnknown:  api.DirectoryUnknownPolicy(settings.Connection.OnUnknown),
		OnAbsent:   api.DirectoryAbsentPolicy(settings.Connection.OnAbsent),
		AdminGroup: settings.Connection.AdminGroup,
		LastSyncAt: settings.Connection.LastSyncAt,
	}

	dto.Connection = &connection

	return dto
}

func directoryRunDTOs(runs []entity.DirectorySyncRun) []api.DirectoryRun {
	dtos := make([]api.DirectoryRun, len(runs))

	for i, run := range runs {
		dtos[i] = api.DirectoryRun{
			Id:         run.ID,
			Trigger:    string(run.Trigger),
			Outcome:    api.DirectorySyncOutcome(run.Outcome),
			Operation:  run.Operation,
			StartedAt:  run.StartedAt,
			FinishedAt: run.FinishedAt,
			Detail:     nilIfEmpty(run.Detail),
		}
	}

	return dtos
}

func directoryChangeDTOs(changes []entity.DirectorySyncChange) []api.DirectoryChange {
	dtos := make([]api.DirectoryChange, len(changes))

	for i, change := range changes {
		dto := api.DirectoryChange{
			Id:         change.ID,
			Subject:    change.Subject,
			Kind:       string(change.Kind),
			Outcome:    api.DirectorySyncOutcome(change.Outcome),
			RecordedAt: change.RecordedAt,
		}

		if len(change.Detail) > 0 {
			detail := change.Detail
			dto.Detail = &detail
		}

		dtos[i] = dto
	}

	return dtos
}
