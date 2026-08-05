package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=searches.go -destination=search/mock_searches.go -package=search -mock_names=Searches=MockSearches

type SearchInput struct {
	Query string
	Kinds []entity.SearchKind
	Limit int
}

type Searches interface {
	Search(ctx context.Context, workspaceID uuid.UUID, input SearchInput) (entity.SearchResults, error)
}
