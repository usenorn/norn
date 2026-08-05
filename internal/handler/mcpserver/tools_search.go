package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type searchInput struct {
	Workspace string   `json:"workspace" jsonschema:"the workspace slug or id"`
	Query     string   `json:"query" jsonschema:"the search text; an issue reference like ENG-42 pins that issue first"`
	Kinds     []string `json:"kinds,omitempty" jsonschema:"restrict to issue, comment, project, team, or person"`
	Limit     int      `json:"limit,omitempty" jsonschema:"results per kind, at most 25"`
}

type searchGroupDTO struct {
	Kind    string            `json:"kind"`
	Results []searchResultDTO `json:"results"`
	More    bool              `json:"more"`
}

type searchOutput struct {
	Groups []searchGroupDTO `json:"groups"`
	Fuzzy  bool             `json:"fuzzy"`
}

func (t *toolset) search(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input searchInput,
) (*mcp.CallToolResult, searchOutput, error) {
	workspace, err := t.resolveWorkspace(ctx, input.Workspace)
	if err != nil {
		return nil, searchOutput{}, toolFailure(ctx, err)
	}

	kinds := make([]entity.SearchKind, 0, len(input.Kinds))

	for _, kind := range input.Kinds {
		kinds = append(kinds, entity.SearchKind(kind))
	}

	results, err := t.searches.Search(ctx, workspace.ID, service.SearchInput{
		Query: input.Query,
		Kinds: kinds,
		Limit: input.Limit,
	})
	if err != nil {
		return nil, searchOutput{}, toolFailure(ctx, err)
	}

	output := searchOutput{Fuzzy: results.Fuzzy, Groups: make([]searchGroupDTO, 0, len(results.Groups))}

	for _, group := range results.Groups {
		dto := searchGroupDTO{
			Kind:    string(group.Kind),
			Results: make([]searchResultDTO, 0, len(group.Results)),
			More:    group.More,
		}

		for _, result := range group.Results {
			dto.Results = append(dto.Results, searchResultDTOFrom(result))
		}

		output.Groups = append(output.Groups, dto)
	}

	return nil, output, nil
}
