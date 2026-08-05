package dashboard

import (
	"context"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) SearchWorkspace(
	ctx context.Context,
	request api.SearchWorkspaceRequestObject,
) (api.SearchWorkspaceResponseObject, error) {
	input := service.SearchInput{Query: request.Params.Q}

	if request.Params.Limit != nil {
		input.Limit = int(*request.Params.Limit)
	}

	if request.Params.Kinds != nil {
		input.Kinds = searchKinds(*request.Params.Kinds)
	}

	results, err := h.searches.Search(ctx, request.WorkspaceId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SearchWorkspace200JSONResponse(searchResultsDTO(results)), nil
}

func searchKinds(requested []api.SearchKind) []entity.SearchKind {
	kinds := make([]entity.SearchKind, 0, len(requested))
	for _, kind := range requested {
		kinds = append(kinds, entity.SearchKind(kind))
	}

	return kinds
}
