package dashboard

import (
	"context"
	"errors"
	"net/http"

	"github.com/usenorn/norn/internal/entity"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func workspaceDomainDTO(domain entity.WorkspaceDomain) api.WorkspaceDomain {
	return api.WorkspaceDomain{
		Id:         domain.ID,
		Domain:     domain.Domain,
		Record:     domain.Record(),
		VerifiedAt: domain.VerifiedAt,
		CheckedAt:  domain.CheckedAt,
	}
}

func (h *handler) ListWorkspaceDomains(
	ctx context.Context,
	request api.ListWorkspaceDomainsRequestObject,
) (api.ListWorkspaceDomainsResponseObject, error) {
	domains, err := h.workspaceDomains.List(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	body := make(api.ListWorkspaceDomains200JSONResponse, 0, len(domains))
	for _, domain := range domains {
		body = append(body, workspaceDomainDTO(domain))
	}

	return body, nil
}

func (h *handler) AddWorkspaceDomain(
	ctx context.Context,
	request api.AddWorkspaceDomainRequestObject,
) (api.AddWorkspaceDomainResponseObject, error) {
	added, err := h.workspaceDomains.Add(ctx, request.WorkspaceId, request.Body.Domain)
	if err != nil {
		if problem, ok := domainProblem(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.AddWorkspaceDomain201JSONResponse(workspaceDomainDTO(added)), nil
}

func (h *handler) VerifyWorkspaceDomain(
	ctx context.Context,
	request api.VerifyWorkspaceDomainRequestObject,
) (api.VerifyWorkspaceDomainResponseObject, error) {
	checked, err := h.workspaceDomains.Verify(ctx, request.WorkspaceId, request.DomainId)
	if err != nil {
		if problem, ok := domainProblem(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.VerifyWorkspaceDomain200JSONResponse(workspaceDomainDTO(checked)), nil
}

func (h *handler) RemoveWorkspaceDomain(
	ctx context.Context,
	request api.RemoveWorkspaceDomainRequestObject,
) (api.RemoveWorkspaceDomainResponseObject, error) {
	if err := h.workspaceDomains.Remove(ctx, request.WorkspaceId, request.DomainId); err != nil {
		if problem, ok := domainProblem(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RemoveWorkspaceDomain204Response{}, nil
}

func domainProblem(err error) (problemResponse, bool) {
	switch {
	case errors.Is(err, entity.ErrWorkspaceDomainNotFound):
		return newProblem(http.StatusNotFound, err.Error()), true
	case errors.Is(err, entity.ErrWorkspaceDomainTaken):
		return newProblem(http.StatusConflict, err.Error()), true
	case errors.Is(err, entity.ErrWorkspaceDomainLookupFailed):
		return newProblem(http.StatusServiceUnavailable, entity.ErrWorkspaceDomainLookupFailed.Error()), true
	}

	return problemFor(err)
}

func (r problemResponse) VisitListWorkspaceDomainsResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitAddWorkspaceDomainResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitVerifyWorkspaceDomainResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r problemResponse) VisitRemoveWorkspaceDomainResponse(w http.ResponseWriter) error {
	return r.write(w)
}
