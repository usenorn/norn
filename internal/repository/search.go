package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=search.go -destination=search/mock_search.go -package=search -mock_names=Search=MockSearch

type SearchRequest struct {
	WorkspaceID uuid.UUID
	AccountID   uuid.UUID
	Query       entity.SearchQuery
	Kinds       []entity.SearchKind
	Limit       int
}

type Search interface {
	Search(ctx context.Context, request SearchRequest) ([]entity.SearchGroup, error)
	Fuzzy(ctx context.Context, request SearchRequest) ([]entity.SearchGroup, error)
}
