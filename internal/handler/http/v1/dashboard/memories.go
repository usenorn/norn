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
		kind := entity.MemoryKind(*request.Params.Kind)
		input.Kind = &kind
	}

	if request.Params.Status != nil {
		for _, status := range *request.Params.Status {
			input.Statuses = append(input.Statuses, entity.MemoryStatus(status))
		}
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

	return api.CreateWorkspaceMemory201JSONResponse(memoryDTO(written.Memory)), nil
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
