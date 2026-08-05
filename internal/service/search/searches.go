package search

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type searchesService struct {
	search     repository.Search
	issues     repository.Issue
	authorizer service.Authorizer
	transactor repository.Transactor
}

func New(
	search repository.Search,
	issues repository.Issue,
	authorizer service.Authorizer,
	transactor repository.Transactor,
) service.Searches {
	return &searchesService{
		search:     search,
		issues:     issues,
		authorizer: authorizer,
		transactor: transactor,
	}
}

func (s *searchesService) Search(
	ctx context.Context,
	workspaceID uuid.UUID,
	input service.SearchInput,
) (entity.SearchResults, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return entity.SearchResults{}, err
	}

	query := entity.ParseSearchQuery(input.Query)

	if query.Empty() {
		return entity.SearchResults{}, entity.ErrSearchQueryEmpty
	}

	request := repository.SearchRequest{
		WorkspaceID: workspaceID,
		AccountID:   decision.Actor.AccountID,
		Query:       query,
		Kinds:       kinds(input.Kinds),
		Limit:       limit(input.Limit),
		Scope:       decision.Scope,
	}

	groups, err := s.search.Search(ctx, request)
	if err != nil {
		return entity.SearchResults{}, err
	}

	results := entity.SearchResults{Query: query, Groups: groups}

	if pinned, found, err := s.reference(ctx, workspaceID, query, decision.Scope); err != nil {
		return entity.SearchResults{}, err
	} else if found {
		results.Groups = pin(results.Groups, pinned)
	}

	if !results.Empty() {
		return results, nil
	}

	fuzzy, err := s.fuzzy(ctx, request)
	if err != nil {
		return entity.SearchResults{}, err
	}

	results.Groups = fuzzy
	results.Fuzzy = true

	return results, nil
}

func (s *searchesService) fuzzy(
	ctx context.Context,
	request repository.SearchRequest,
) ([]entity.SearchGroup, error) {
	var groups []entity.SearchGroup

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		found, err := s.search.Fuzzy(ctx, request)
		if err != nil {
			return err
		}

		groups = found

		return nil
	}); err != nil {
		return nil, err
	}

	return groups, nil
}

func (s *searchesService) reference(
	ctx context.Context,
	workspaceID uuid.UUID,
	query entity.SearchQuery,
	scope entity.TeamScope,
) (entity.SearchResult, bool, error) {
	if query.Reference == nil {
		return entity.SearchResult{}, false, nil
	}

	issue, err := s.issues.GetVisibleByReference(ctx, workspaceID, *query.Reference, scope)
	if err != nil {
		if errors.Is(err, entity.ErrIssueNotFound) {
			return entity.SearchResult{}, false, nil
		}

		return entity.SearchResult{}, false, err
	}

	if issue.Status != entity.IssueStatusActive {
		return entity.SearchResult{}, false, nil
	}

	return entity.SearchResult{
		Kind:      entity.SearchKindIssue,
		ID:        issue.ID,
		IssueID:   issue.ID,
		Title:     issue.Title,
		Reference: issue.Reference(),
		TeamKey:   issue.TeamKey,
		Status:    string(issue.Status),
		TitleHit:  true,
		UpdatedAt: issue.UpdatedAt,
	}, true, nil
}

func pin(groups []entity.SearchGroup, hit entity.SearchResult) []entity.SearchGroup {
	for index, group := range groups {
		if group.Kind != entity.SearchKindIssue {
			continue
		}

		results := make([]entity.SearchResult, 0, len(group.Results)+1)
		results = append(results, hit)

		for _, result := range group.Results {
			if result.ID != hit.ID {
				results = append(results, result)
			}
		}

		groups[index].Results = results

		return groups
	}

	return append([]entity.SearchGroup{{
		Kind:    entity.SearchKindIssue,
		Results: []entity.SearchResult{hit},
	}}, groups...)
}

func kinds(requested []entity.SearchKind) []entity.SearchKind {
	if len(requested) == 0 {
		return entity.SearchKinds()
	}

	allowed := make([]entity.SearchKind, 0, len(requested))

	for _, kind := range requested {
		if kind.Valid() {
			allowed = append(allowed, kind)
		}
	}

	return allowed
}

func limit(requested int) int {
	if requested <= 0 {
		return entity.SearchPaletteSize
	}

	if requested > entity.SearchGroupMaxSize {
		return entity.SearchGroupMaxSize
	}

	return requested
}
