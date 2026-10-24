package dashboard

import (
	"context"

	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceDocs(
	ctx context.Context,
	request api.ListWorkspaceDocsRequestObject,
) (api.ListWorkspaceDocsResponseObject, error) {
	input := service.ListDocsInput{ParentID: request.Params.ParentId}

	if request.Params.Root != nil {
		input.Root = *request.Params.Root
	}

	if request.Params.Limit != nil {
		input.Limit = int(*request.Params.Limit)
	}

	if request.Params.Cursor != nil {
		input.Cursor = *request.Params.Cursor
	}

	list, err := h.docs.List(ctx, request.WorkspaceId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceDocs200JSONResponse(docPageDTO(list)), nil
}

func (h *handler) CreateWorkspaceDoc(
	ctx context.Context,
	request api.CreateWorkspaceDocRequestObject,
) (api.CreateWorkspaceDocResponseObject, error) {
	input := service.CreateDocInput{
		Title:    request.Body.Title,
		Content:  documentOf(request.Body.Content),
		ParentID: request.Body.ParentId,
		Audience: docAudienceOf(request.Body.Audience),
	}

	if request.Body.Markdown != nil {
		input.Markdown = *request.Body.Markdown
	}

	doc, err := h.docs.Create(ctx, request.WorkspaceId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.CreateWorkspaceDoc201JSONResponse(docDTO(doc)), nil
}

func (h *handler) GetWorkspaceDoc(
	ctx context.Context,
	request api.GetWorkspaceDocRequestObject,
) (api.GetWorkspaceDocResponseObject, error) {
	doc, err := h.docs.Get(ctx, request.WorkspaceId, request.DocId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceDoc200JSONResponse(docDTO(doc)), nil
}

func (h *handler) UpdateWorkspaceDoc(
	ctx context.Context,
	request api.UpdateWorkspaceDocRequestObject,
) (api.UpdateWorkspaceDocResponseObject, error) {
	doc, err := h.docs.Update(ctx, request.WorkspaceId, request.DocId, service.UpdateDocInput{
		ExpectedVersion: int(request.Body.ExpectedVersion),
		Title:           request.Body.Title,
		Markdown:        request.Body.Markdown,
		Content:         documentOf(request.Body.Content),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UpdateWorkspaceDoc200JSONResponse(docDTO(doc)), nil
}

func (h *handler) DeleteWorkspaceDoc(
	ctx context.Context,
	request api.DeleteWorkspaceDocRequestObject,
) (api.DeleteWorkspaceDocResponseObject, error) {
	doc, err := h.docs.Archive(ctx, request.WorkspaceId, request.DocId, int(request.Params.ExpectedVersion))
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DeleteWorkspaceDoc200JSONResponse(docDTO(doc)), nil
}

func (h *handler) MoveWorkspaceDoc(
	ctx context.Context,
	request api.MoveWorkspaceDocRequestObject,
) (api.MoveWorkspaceDocResponseObject, error) {
	doc, err := h.docs.Move(ctx, request.WorkspaceId, request.DocId, service.MoveDocInput{
		ExpectedVersion: int(request.Body.ExpectedVersion),
		ParentID:        request.Body.ParentId,
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.MoveWorkspaceDoc200JSONResponse(docDTO(doc)), nil
}

func (h *handler) ShareWorkspaceDoc(
	ctx context.Context,
	request api.ShareWorkspaceDocRequestObject,
) (api.ShareWorkspaceDocResponseObject, error) {
	doc, err := h.docs.Reshare(ctx, request.WorkspaceId, request.DocId, service.ReshareDocInput{
		ExpectedVersion: int(request.Body.ExpectedVersion),
		Audience:        docAudienceOf(request.Body.Audience),
	})
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ShareWorkspaceDoc200JSONResponse(docDTO(doc)), nil
}

func (h *handler) ListWorkspaceDocRevisions(
	ctx context.Context,
	request api.ListWorkspaceDocRevisionsRequestObject,
) (api.ListWorkspaceDocRevisionsResponseObject, error) {
	var input service.ListDocRevisionsInput

	if request.Params.Limit != nil {
		input.Limit = int(*request.Params.Limit)
	}

	if request.Params.Cursor != nil {
		input.Cursor = *request.Params.Cursor
	}

	history, err := h.docs.Revisions(ctx, request.WorkspaceId, request.DocId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceDocRevisions200JSONResponse(docHistoryDTO(history)), nil
}

func (h *handler) GetWorkspaceDocRevision(
	ctx context.Context,
	request api.GetWorkspaceDocRevisionRequestObject,
) (api.GetWorkspaceDocRevisionResponseObject, error) {
	revision, err := h.docs.Revision(ctx, request.WorkspaceId, request.DocId, request.RevisionId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceDocRevision200JSONResponse(docRevisionDTO(revision)), nil
}

func (h *handler) RestoreWorkspaceDocRevision(
	ctx context.Context,
	request api.RestoreWorkspaceDocRevisionRequestObject,
) (api.RestoreWorkspaceDocRevisionResponseObject, error) {
	doc, err := h.docs.Restore(
		ctx, request.WorkspaceId, request.DocId, request.RevisionId, int(request.Body.ExpectedVersion),
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RestoreWorkspaceDocRevision200JSONResponse(docDTO(doc)), nil
}
