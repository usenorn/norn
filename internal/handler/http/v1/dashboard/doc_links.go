package dashboard

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceDocLinks(
	ctx context.Context,
	request api.ListWorkspaceDocLinksRequestObject,
) (api.ListWorkspaceDocLinksResponseObject, error) {
	links, err := h.docLinks.List(ctx, request.WorkspaceId, request.DocId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	dtos := make([]api.DocLink, 0, len(links))
	for _, link := range links {
		dtos = append(dtos, docLinkDTO(link))
	}

	return api.ListWorkspaceDocLinks200JSONResponse{Links: dtos}, nil
}

func (h *handler) LinkWorkspaceDoc(
	ctx context.Context,
	request api.LinkWorkspaceDocRequestObject,
) (api.LinkWorkspaceDocResponseObject, error) {
	var input service.LinkDocInput

	if request.Body.IssueId != nil {
		input.IssueID = *request.Body.IssueId
	}

	if request.Body.ProjectId != nil {
		input.ProjectID = *request.Body.ProjectId
	}

	link, err := h.docLinks.Link(ctx, request.WorkspaceId, request.DocId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.LinkWorkspaceDoc201JSONResponse(docLinkDTO(link)), nil
}

func (h *handler) UnlinkWorkspaceDoc(
	ctx context.Context,
	request api.UnlinkWorkspaceDocRequestObject,
) (api.UnlinkWorkspaceDocResponseObject, error) {
	if err := h.docLinks.Unlink(ctx, request.WorkspaceId, request.DocId, request.LinkId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UnlinkWorkspaceDoc204Response{}, nil
}

func (h *handler) ListWorkspaceIssueDocs(
	ctx context.Context,
	request api.ListWorkspaceIssueDocsRequestObject,
) (api.ListWorkspaceIssueDocsResponseObject, error) {
	docs, err := h.docLinks.ForIssue(ctx, request.WorkspaceId, request.IssueId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceIssueDocs200JSONResponse(linkedDocsDTO(docs)), nil
}

func (h *handler) ListWorkspaceProjectDocs(
	ctx context.Context,
	request api.ListWorkspaceProjectDocsRequestObject,
) (api.ListWorkspaceProjectDocsResponseObject, error) {
	docs, err := h.docLinks.ForProject(ctx, request.WorkspaceId, request.ProjectId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceProjectDocs200JSONResponse(linkedDocsDTO(docs)), nil
}

func docLinkDTO(link entity.DocLink) api.DocLink {
	dto := api.DocLink{Id: link.ID, Target: api.DocLinkTarget(link.Target), CreatedAt: link.CreatedAt}

	switch link.Target {
	case entity.DocLinkTargetIssue:
		dto.Issue = &api.DocLinkedIssue{
			Id:        link.Issue.ID,
			Reference: link.Issue.Reference(),
			Title:     link.Issue.Title,
			State: api.DocLinkedIssueState{
				Name:     link.Issue.State.Name,
				Category: api.StateCategory(link.Issue.State.Category),
			},
		}
	case entity.DocLinkTargetProject:
		dto.Project = &api.DocLinkedProject{
			Id:       link.Project.ID,
			Slug:     link.Project.Slug,
			Name:     link.Project.Name,
			Archived: link.Project.Archived(),
		}
	}

	return dto
}

func linkedDocsDTO(docs []entity.DocSummary) api.LinkedDocList {
	dtos := make([]api.DocSummary, 0, len(docs))
	for _, summary := range docs {
		dtos = append(dtos, docSummaryDTO(summary))
	}

	return api.LinkedDocList{Docs: dtos}
}
