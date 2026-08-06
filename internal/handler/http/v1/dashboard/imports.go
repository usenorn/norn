package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

const importFileFormField = "file"

func (h *handler) ListImportSources(
	ctx context.Context,
	request api.ListImportSourcesRequestObject,
) (api.ListImportSourcesResponseObject, error) {
	kinds, err := h.imports.Sources(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListImportSources200JSONResponse(importSourceDTOs(kinds)), nil
}

func (h *handler) ListWorkspaceImports(
	ctx context.Context,
	request api.ListWorkspaceImportsRequestObject,
) (api.ListWorkspaceImportsResponseObject, error) {
	page, err := importRunPage(request.Params)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	runs, err := h.imports.List(ctx, request.WorkspaceId, page)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceImports200JSONResponse(importRunPageDTO(runs, page.Limit)), nil
}

func (h *handler) CreateWorkspaceImport(
	ctx context.Context,
	request api.CreateWorkspaceImportRequestObject,
) (api.CreateWorkspaceImportResponseObject, error) {
	run, err := h.imports.Connect(ctx, service.CreateImportInput{
		WorkspaceID: request.WorkspaceId,
		SourceKind:  request.Body.SourceKind,
		SourceLabel: textOf(request.Body.SourceLabel),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.CreateWorkspaceImport201JSONResponse(importRunDTO(run)), nil
}

func (h *handler) GetWorkspaceImport(
	ctx context.Context,
	request api.GetWorkspaceImportRequestObject,
) (api.GetWorkspaceImportResponseObject, error) {
	run, err := h.imports.Get(ctx, request.WorkspaceId, request.ImportRunId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceImport200JSONResponse(importRunDTO(run)), nil
}

func (h *handler) ConfigureWorkspaceImport(
	ctx context.Context,
	request api.ConfigureWorkspaceImportRequestObject,
) (api.ConfigureWorkspaceImportResponseObject, error) {
	settings, err := importSettings(request.Body.Settings)
	if err != nil {
		return newProblem(http.StatusUnprocessableEntity, err.Error()), nil
	}

	input := service.ConfigureImportInput{
		Secret:   textOf(request.Body.ApiKey),
		Settings: settings,
	}

	if request.Body.UnknownReferences != nil {
		input.UnknownReferences = entity.ImportUnknownPolicy(*request.Body.UnknownReferences)
	}

	run, err := h.imports.Configure(ctx, request.WorkspaceId, request.ImportRunId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ConfigureWorkspaceImport200JSONResponse(importRunDTO(run)), nil
}

func (h *handler) UploadWorkspaceImportFile(
	ctx context.Context,
	request api.UploadWorkspaceImportFileRequestObject,
) (api.UploadWorkspaceImportFileResponseObject, error) {
	part, err := request.Body.NextPart()
	if err != nil {
		return newProblem(http.StatusBadRequest, "the request must carry a single "+importFileFormField+" part"), nil
	}
	defer func() { _ = part.Close() }()

	if part.FormName() != importFileFormField {
		return newProblem(http.StatusBadRequest, "the request must carry a single "+importFileFormField+" part"), nil
	}

	file, err := h.imports.Upload(ctx, request.WorkspaceId, request.ImportRunId, service.ImportUpload{
		FileName: part.FileName(),
		Body:     http.MaxBytesReader(nil, part, h.importing.MaxUploadBytes),
	})
	if err != nil {
		var oversized *http.MaxBytesError
		if errors.As(err, &oversized) {
			return newProblem(http.StatusRequestEntityTooLarge, entity.ErrAttachmentTooLarge.Error()), nil
		}

		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UploadWorkspaceImportFile200JSONResponse(importFileDTO(file)), nil
}

func (h *handler) GetWorkspaceImportCatalogue(
	ctx context.Context,
	request api.GetWorkspaceImportCatalogueRequestObject,
) (api.GetWorkspaceImportCatalogueResponseObject, error) {
	catalogue, err := h.imports.Catalogue(ctx, request.WorkspaceId, request.ImportRunId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceImportCatalogue200JSONResponse(importCatalogueDTO(catalogue)), nil
}

func (h *handler) StageWorkspaceImport(
	ctx context.Context,
	request api.StageWorkspaceImportRequestObject,
) (api.StageWorkspaceImportResponseObject, error) {
	run, err := h.imports.Stage(ctx, request.WorkspaceId, request.ImportRunId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.StageWorkspaceImport202JSONResponse(importRunDTO(run)), nil
}

func (h *handler) GetWorkspaceImportMappings(
	ctx context.Context,
	request api.GetWorkspaceImportMappingsRequestObject,
) (api.GetWorkspaceImportMappingsResponseObject, error) {
	plan, err := h.imports.Mappings(ctx, request.WorkspaceId, request.ImportRunId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceImportMappings200JSONResponse(importMappingPlanDTO(plan)), nil
}

func (h *handler) DecideWorkspaceImportMappings(
	ctx context.Context,
	request api.DecideWorkspaceImportMappingsRequestObject,
) (api.DecideWorkspaceImportMappingsResponseObject, error) {
	plan, err := h.imports.Map(ctx, request.WorkspaceId, request.ImportRunId, service.MapImportInput{
		Decisions: importMappingDecisions(request.Body.Decisions),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DecideWorkspaceImportMappings200JSONResponse(importMappingPlanDTO(plan)), nil
}

func (h *handler) PreviewWorkspaceImport(
	ctx context.Context,
	request api.PreviewWorkspaceImportRequestObject,
) (api.PreviewWorkspaceImportResponseObject, error) {
	preview, err := h.imports.Preview(ctx, request.WorkspaceId, request.ImportRunId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.PreviewWorkspaceImport200JSONResponse(importPreviewDTO(preview)), nil
}

func (h *handler) ExecuteWorkspaceImport(
	ctx context.Context,
	request api.ExecuteWorkspaceImportRequestObject,
) (api.ExecuteWorkspaceImportResponseObject, error) {
	input := service.ExecuteImportInput{PreviewDigest: request.Body.PreviewDigest}

	if request.Body.AcknowledgeTriage != nil {
		input.AcknowledgeTriage = *request.Body.AcknowledgeTriage
	}

	run, err := h.imports.Execute(ctx, request.WorkspaceId, request.ImportRunId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ExecuteWorkspaceImport202JSONResponse(importRunDTO(run)), nil
}

func (h *handler) RevertWorkspaceImport(
	ctx context.Context,
	request api.RevertWorkspaceImportRequestObject,
) (api.RevertWorkspaceImportResponseObject, error) {
	run, err := h.imports.Revert(ctx, request.WorkspaceId, request.ImportRunId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RevertWorkspaceImport202JSONResponse(importRunDTO(run)), nil
}

func (h *handler) GetWorkspaceImportReport(
	ctx context.Context,
	request api.GetWorkspaceImportReportRequestObject,
) (api.GetWorkspaceImportReportResponseObject, error) {
	view, err := h.imports.Report(ctx, request.WorkspaceId, request.ImportRunId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceImportReport200JSONResponse(importReportDTO(view)), nil
}

func importRunPage(params api.ListWorkspaceImportsParams) (entity.ImportRunPage, error) {
	page := entity.ImportRunPage{}

	if params.Limit != nil {
		page.Limit = int(*params.Limit)
	}

	if params.Cursor != nil && *params.Cursor != "" {
		cursor, err := entity.DecodeImportRunCursor(*params.Cursor)
		if err != nil {
			return entity.ImportRunPage{}, err
		}

		page.Cursor = &cursor
	}

	return page.Normalized(), nil
}

func importSettings(settings *map[string]interface{}) (json.RawMessage, error) {
	if settings == nil {
		return nil, nil
	}

	encoded, err := json.Marshal(*settings)
	if err != nil {
		return nil, err
	}

	return encoded, nil
}

func importMappingDecisions(decisions []api.ImportMappingDecision) []entity.ImportMapping {
	chosen := make([]entity.ImportMapping, 0, len(decisions))

	for _, decision := range decisions {
		mapping := entity.ImportMapping{
			Kind:        entity.ImportMappingKind(decision.Kind),
			SourceKey:   decision.SourceKey,
			Decision:    entity.ImportDecision(decision.Decision),
			TargetValue: textOf(decision.TargetValue),
		}

		if decision.TargetId != nil {
			mapping.TargetID = *decision.TargetId
		}

		chosen = append(chosen, mapping)
	}

	return chosen
}

func importSourceDTOs(kinds []string) []api.ImportSource {
	sources := make([]api.ImportSource, 0, len(kinds))
	for _, kind := range kinds {
		sources = append(sources, api.ImportSource{Kind: kind})
	}

	return sources
}

func importRunDTO(run entity.ImportRun) api.ImportRun {
	dto := api.ImportRun{
		Id:                run.ID,
		SourceKind:        run.SourceKind,
		SourceLabel:       nilIfEmpty(run.SourceLabel),
		Status:            api.ImportStatus(run.Status),
		RequestedBy:       auditID(run.RequestedByAccount),
		RevertedBy:        auditID(run.RevertedByAccount),
		PhaseError:        nilIfEmpty(run.PhaseError),
		PreviewDigest:     nilIfEmpty(run.PreviewDigest),
		AcknowledgeTriage: run.AcknowledgeTriage,
		Settings:          importSettingsDTO(run.Settings),
		UnknownReferences: api.ImportUnknownPolicy(run.UnknownReferences.Or(entity.ImportUnknownSkip)),
		SourceSecretSet:   run.SourceSecretSet,
		Attempt:           int32(run.Attempt),
		Staged:            int32(run.Staged),
		Processed:         int32(run.Processed),
		StagedAt:          run.StagedAt,
		MappedAt:          run.MappedAt,
		StartedAt:         run.StartedAt,
		FinishedAt:        run.FinishedAt,
		RevertedAt:        run.RevertedAt,
		CreatedAt:         run.CreatedAt,
		UpdatedAt:         run.UpdatedAt,
	}

	return dto
}

func importSettingsDTO(settings json.RawMessage) *map[string]interface{} {
	if len(settings) == 0 {
		return nil
	}

	held := map[string]interface{}{}
	if err := json.Unmarshal(settings, &held); err != nil {
		return nil
	}

	return &held
}

func importRunDTOs(runs []entity.ImportRun) []api.ImportRun {
	dtos := make([]api.ImportRun, 0, len(runs))
	for _, run := range runs {
		dtos = append(dtos, importRunDTO(run))
	}

	return dtos
}

func importRunPageDTO(runs []entity.ImportRun, limit int) api.ImportRunPage {
	page := api.ImportRunPage{Runs: importRunDTOs(runs)}

	if len(runs) == limit && limit > 0 {
		cursor := runs[len(runs)-1].Cursor().Encode()
		page.NextCursor = &cursor
	}

	return page
}

func importFileDTO(file service.ImportFile) api.ImportFile {
	return api.ImportFile{ObjectKey: file.ObjectKey, FileName: file.FileName}
}

func importCatalogueDTO(catalogue entity.ImportCatalogue) api.ImportCatalogue {
	scopes := make([]api.ImportScope, 0, len(catalogue.Scopes))
	for _, scope := range catalogue.Scopes {
		scopes = append(scopes, api.ImportScope{
			Key:    scope.Key,
			Name:   scope.Name,
			Detail: nilIfEmpty(scope.Detail),
		})
	}

	columns := make([]api.ImportColumn, 0, len(catalogue.Columns))
	for _, column := range catalogue.Columns {
		columns = append(columns, api.ImportColumn{
			Index:      int32(column.Index),
			Header:     column.Header,
			Proposed:   nilIfEmpty(column.Proposed),
			Confidence: nilIfEmpty(column.Confidence),
		})
	}

	notes := catalogue.Notes
	if notes == nil {
		notes = make([]string, 0)
	}

	return api.ImportCatalogue{Scopes: scopes, Columns: columns, Notes: notes}
}

func importMappingPlanDTO(plan entity.MappingPlan) api.ImportMappingPlan {
	mappings := make([]api.ImportMapping, 0, len(plan.Mappings))

	for _, mapping := range plan.Mappings {
		dto := api.ImportMapping{
			Kind:              api.ImportMappingKind(mapping.Kind),
			SourceKey:         mapping.SourceKey,
			SourceLabel:       nilIfEmpty(mapping.SourceLabel),
			SourceEmail:       nilIfEmpty(mapping.SourceEmail),
			TargetId:          auditID(mapping.TargetID),
			TargetValue:       nilIfEmpty(mapping.TargetValue),
			SuggestedTargetId: auditID(mapping.SuggestedTargetID),
			SuggestedReason:   nilIfEmpty(mapping.SuggestedReason),
			DecidedBy:         auditID(mapping.DecidedByAccount),
			DecidedAt:         mapping.DecidedAt,
		}

		if mapping.Decision.Valid() {
			decision := api.ImportDecision(mapping.Decision)
			dto.Decision = &decision
		}

		mappings = append(mappings, dto)
	}

	return api.ImportMappingPlan{Mappings: mappings, Complete: plan.Complete()}
}

func importPreviewLineDTOs(lines []entity.ImportPreviewLine) []api.ImportPreviewLine {
	dtos := make([]api.ImportPreviewLine, 0, len(lines))

	for _, line := range lines {
		dtos = append(dtos, api.ImportPreviewLine{
			Resource:   api.ImportResource(line.Resource),
			Subject:    nilIfEmpty(line.Subject),
			ExternalId: nilIfEmpty(line.ExternalID),
			Outcome:    api.ImportOutcome(line.Outcome),
			Detail:     nilIfEmpty(line.Detail),
		})
	}

	return dtos
}

func importPreviewDTO(preview entity.ImportPreview) api.ImportPreview {
	teams := preview.TriageTeams
	if teams == nil {
		teams = make([]string, 0)
	}

	return api.ImportPreview{
		Digest:       preview.Digest(),
		Created:      importPreviewLineDTOs(preview.Created),
		Changed:      importPreviewLineDTOs(preview.Changed),
		Skipped:      importPreviewLineDTOs(preview.Skipped),
		Unattributed: importPreviewLineDTOs(preview.Unattributed),
		Warnings:     importPreviewLineDTOs(preview.Warnings),
		TriageTeams:  teams,
	}
}

func importLedgerEntryDTOs(entries []entity.ImportLedgerEntry) []api.ImportLedgerEntry {
	dtos := make([]api.ImportLedgerEntry, 0, len(entries))

	for _, entry := range entries {
		dto := api.ImportLedgerEntry{
			Resource:   api.ImportResource(entry.Resource),
			CreatedId:  entry.CreatedID,
			ExternalId: entry.ExternalID,
			Reference:  entry.Reference,
			OrderSeq:   entry.OrderSeq,
			CreatedAt:  entry.CreatedAtRecorded,
			RevertedAt: entry.RevertedAt,
		}

		if entry.RevertOutcome != "" {
			outcome := api.ImportOutcome(entry.RevertOutcome)
			dto.RevertOutcome = &outcome
		}

		dtos = append(dtos, dto)
	}

	return dtos
}

func importReportLineDTOs(lines []entity.ImportReportLine) []api.ImportReportLine {
	dtos := make([]api.ImportReportLine, 0, len(lines))

	for _, line := range lines {
		dto := api.ImportReportLine{
			Id:         line.ID,
			Phase:      api.ImportPhase(line.Phase),
			Resource:   api.ImportResource(line.Resource),
			Subject:    nilIfEmpty(line.Subject),
			ExternalId: nilIfEmpty(line.ExternalID),
			Outcome:    api.ImportOutcome(line.Outcome),
			RecordedAt: line.RecordedAt,
		}

		if len(line.Detail) > 0 {
			detail := line.Detail
			dto.Detail = &detail
		}

		dtos = append(dtos, dto)
	}

	return dtos
}

func importReportDTO(view service.ImportReportView) api.ImportReport {
	report := api.ImportReport{
		Run:     importRunDTO(view.Run),
		Created: importLedgerEntryDTOs(view.Created),
		Lines:   importReportLineDTOs(view.Lines),
	}

	if view.NextCreatedCursor > 0 {
		cursor := view.NextCreatedCursor
		report.NextCreatedCursor = &cursor
	}

	if view.NextLineCursor != nil {
		report.NextLineCursor = &api.ImportReportCursor{
			RecordedAt: view.NextLineCursor.RecordedAt,
			Id:         view.NextLineCursor.ID,
		}
	}

	return report
}
