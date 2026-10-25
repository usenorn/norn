package dashboard

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceMemories(
	ctx context.Context,
	request api.ListWorkspaceMemoriesRequestObject,
) (api.ListWorkspaceMemoriesResponseObject, error) {
	input := service.ListMemoriesInput{ProjectID: request.Params.ProjectId}

	if request.Params.Kind != nil {
		for _, kind := range *request.Params.Kind {
			input.Kinds = append(input.Kinds, entity.MemoryKind(kind))
		}
	}

	if request.Params.Status != nil {
		for _, status := range *request.Params.Status {
			input.Statuses = append(input.Statuses, entity.MemoryStatus(status))
		}
	}

	if request.Params.Visibility != nil {
		for _, visibility := range *request.Params.Visibility {
			input.Visibilities = append(input.Visibilities, entity.KnowledgeVisibility(visibility))
		}
	}

	if request.Params.Reviewable != nil {
		input.Reviewable = *request.Params.Reviewable
	}

	if request.Params.Component != nil {
		input.Component = *request.Params.Component
	}

	if request.Params.Limit != nil {
		input.Limit = int(*request.Params.Limit)
	}

	if request.Params.Cursor != nil {
		input.Cursor = *request.Params.Cursor
	}

	list, err := h.memories.List(ctx, request.WorkspaceId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceMemories200JSONResponse(memoryPageDTO(list)), nil
}

func (h *handler) CreateWorkspaceMemory(
	ctx context.Context,
	request api.CreateWorkspaceMemoryRequestObject,
) (api.CreateWorkspaceMemoryResponseObject, error) {
	written, err := h.memories.Create(ctx, request.WorkspaceId, createMemoryInputOf(*request.Body))
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.CreateWorkspaceMemory201JSONResponse(memoryWriteDTO(written)), nil
}

func (h *handler) GetWorkspaceMemoryRelated(
	ctx context.Context,
	request api.GetWorkspaceMemoryRelatedRequestObject,
) (api.GetWorkspaceMemoryRelatedResponseObject, error) {
	written, err := h.memories.Related(ctx, request.WorkspaceId, request.MemoryId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceMemoryRelated200JSONResponse(memoryWriteDTO(written)), nil
}

func (h *handler) ReconcileWorkspaceMemory(
	ctx context.Context,
	request api.ReconcileWorkspaceMemoryRequestObject,
) (api.ReconcileWorkspaceMemoryResponseObject, error) {
	reconciled, err := h.memories.Reconcile(ctx, request.WorkspaceId, request.MemoryId, reconcileMemoryInputOf(*request.Body))
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ReconcileWorkspaceMemory200JSONResponse(memoryReconciliationDTO(reconciled)), nil
}

func (h *handler) GetWorkspaceMemory(
	ctx context.Context,
	request api.GetWorkspaceMemoryRequestObject,
) (api.GetWorkspaceMemoryResponseObject, error) {
	memory, err := h.memories.Get(ctx, request.WorkspaceId, request.MemoryId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceMemory200JSONResponse(memoryDTO(memory)), nil
}

func (h *handler) GetWorkspaceMemoryLineage(
	ctx context.Context,
	request api.GetWorkspaceMemoryLineageRequestObject,
) (api.GetWorkspaceMemoryLineageResponseObject, error) {
	lineage, err := h.memories.Lineage(ctx, request.WorkspaceId, request.MemoryId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	memories := make([]api.Memory, 0, len(lineage))
	for _, memory := range lineage {
		memories = append(memories, memoryDTO(memory))
	}

	return api.GetWorkspaceMemoryLineage200JSONResponse(api.MemoryLineage{Memories: memories}), nil
}

func (h *handler) CorrectWorkspaceMemory(
	ctx context.Context,
	request api.CorrectWorkspaceMemoryRequestObject,
) (api.CorrectWorkspaceMemoryResponseObject, error) {
	body := request.Body

	memory, err := h.memories.Correct(ctx, request.WorkspaceId, request.MemoryId, service.CorrectMemoryInput{
		MemoryContent: memoryContentOf(api.MemoryContent{
			Kind:           body.Kind,
			Statement:      body.Statement,
			Rationale:      body.Rationale,
			Conditions:     body.Conditions,
			AppliesTo:      body.AppliesTo,
			Basis:          body.Basis,
			Evidence:       body.Evidence,
			EffectiveFrom:  body.EffectiveFrom,
			EffectiveUntil: body.EffectiveUntil,
		}),
		ExpectedVersion: int(body.ExpectedVersion),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.CorrectWorkspaceMemory200JSONResponse(memoryDTO(memory)), nil
}

func (h *handler) SupersedeWorkspaceMemory(
	ctx context.Context,
	request api.SupersedeWorkspaceMemoryRequestObject,
) (api.SupersedeWorkspaceMemoryResponseObject, error) {
	memory, err := h.memories.Supersede(ctx, request.WorkspaceId, request.MemoryId, service.SupersedeMemoryInput{
		Replacement:     createMemoryInputOf(request.Body.Replacement),
		ExpectedVersion: int(request.Body.ExpectedVersion),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SupersedeWorkspaceMemory201JSONResponse(memoryDTO(memory)), nil
}

func (h *handler) WithdrawWorkspaceMemory(
	ctx context.Context,
	request api.WithdrawWorkspaceMemoryRequestObject,
) (api.WithdrawWorkspaceMemoryResponseObject, error) {
	memory, err := h.memories.Withdraw(ctx, request.WorkspaceId, request.MemoryId, int(request.Body.ExpectedVersion))
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.WithdrawWorkspaceMemory200JSONResponse(memoryDTO(memory)), nil
}

func (h *handler) AcceptWorkspaceMemory(
	ctx context.Context,
	request api.AcceptWorkspaceMemoryRequestObject,
) (api.AcceptWorkspaceMemoryResponseObject, error) {
	input := service.AcceptMemoryInput{ExpectedVersion: int(request.Body.ExpectedVersion)}

	if request.Body.Content != nil {
		content := memoryContentOf(*request.Body.Content)
		input.Content = &content
	}

	memory, err := h.memories.Accept(ctx, request.WorkspaceId, request.MemoryId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.AcceptWorkspaceMemory200JSONResponse(memoryDTO(memory)), nil
}

func (h *handler) RejectWorkspaceMemory(
	ctx context.Context,
	request api.RejectWorkspaceMemoryRequestObject,
) (api.RejectWorkspaceMemoryResponseObject, error) {
	memory, err := h.memories.Reject(ctx, request.WorkspaceId, request.MemoryId, service.RejectMemoryInput{
		ExpectedVersion: int(request.Body.ExpectedVersion),
		Reason:          request.Body.Reason,
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RejectWorkspaceMemory200JSONResponse(memoryDTO(memory)), nil
}

func (h *handler) ShareWorkspaceMemory(
	ctx context.Context,
	request api.ShareWorkspaceMemoryRequestObject,
) (api.ShareWorkspaceMemoryResponseObject, error) {
	memory, err := h.memories.Reshare(ctx, request.WorkspaceId, request.MemoryId, service.ReshareMemoryInput{
		ExpectedVersion: int(request.Body.ExpectedVersion),
		Audience:        docAudienceOf(request.Body.Audience),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ShareWorkspaceMemory200JSONResponse(memoryDTO(memory)), nil
}

func (h *handler) ListWorkspaceMemoryRevisions(
	ctx context.Context,
	request api.ListWorkspaceMemoryRevisionsRequestObject,
) (api.ListWorkspaceMemoryRevisionsResponseObject, error) {
	var input service.ListMemoryRevisionsInput

	if request.Params.Limit != nil {
		input.Limit = int(*request.Params.Limit)
	}

	if request.Params.Cursor != nil {
		input.Cursor = *request.Params.Cursor
	}

	history, err := h.memories.Revisions(ctx, request.WorkspaceId, request.MemoryId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceMemoryRevisions200JSONResponse(memoryHistoryDTO(history)), nil
}

func (h *handler) GetWorkspaceMemoryRevision(
	ctx context.Context,
	request api.GetWorkspaceMemoryRevisionRequestObject,
) (api.GetWorkspaceMemoryRevisionResponseObject, error) {
	revision, err := h.memories.Revision(ctx, request.WorkspaceId, request.MemoryId, request.RevisionId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceMemoryRevision200JSONResponse(memoryRevisionDTO(revision)), nil
}
