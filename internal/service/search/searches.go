package search

import (
	"context"
	"errors"
	"slices"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type searchesService struct {
	search     repository.Search
	issues     repository.Issue
	authorizer service.Authorizer
	knowledge  service.KnowledgeAuthorizer
	transactor repository.Transactor
}

func New(
	search repository.Search,
	issues repository.Issue,
	authorizer service.Authorizer,
	knowledge service.KnowledgeAuthorizer,
	transactor repository.Transactor,
) service.Searches {
	return &searchesService{
		search:     search,
		issues:     issues,
		authorizer: authorizer,
		knowledge:  knowledge,
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

	if err := s.reachDocs(ctx, &request); err != nil {
		return entity.SearchResults{}, err
	}

	groups, err := s.search.Search(ctx, request)
	if err != nil {
		return entity.SearchResults{}, err
	}

	results := entity.SearchResults{Query: query, Groups: groups}

	pinned, err := s.pinned(ctx, workspaceID, query, decision.Scope)
	if err != nil {
		return entity.SearchResults{}, err
	}

	results.Groups = pin(results.Groups, pinned)

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

func (s *searchesService) pinned(
	ctx context.Context,
	workspaceID uuid.UUID,
	query entity.SearchQuery,
	scope entity.TeamScope,
) ([]entity.SearchResult, error) {
	if query.Reference != nil {
		issue, err := s.issues.GetVisibleByReference(ctx, workspaceID, *query.Reference, scope)
		if err != nil {
			if errors.Is(err, entity.ErrIssueNotFound) {
				return nil, nil
			}

			return nil, err
		}

		return found([]entity.Issue{issue}), nil
	}

	if query.Number > 0 {
		issues, err := s.issues.ListVisibleByNumber(
			ctx, workspaceID, query.Number, entity.SearchPinnedMax, scope,
		)
		if err != nil {
			return nil, err
		}

		return found(issues), nil
	}

	return nil, nil
}

func found(issues []entity.Issue) []entity.SearchResult {
	results := make([]entity.SearchResult, 0, len(issues))

	for _, issue := range issues {
		if issue.Status != entity.IssueStatusActive {
			continue
		}

		results = append(results, entity.SearchResult{
			Kind:      entity.SearchKindIssue,
			ID:        issue.ID,
			IssueID:   issue.ID,
			Title:     issue.Title,
			Reference: issue.Reference(),
			TeamKey:   issue.TeamKey,
			Status:    string(issue.Status),
			TitleHit:  true,
			UpdatedAt: issue.UpdatedAt,
		})
	}

	return results
}

func pin(groups []entity.SearchGroup, hits []entity.SearchResult) []entity.SearchGroup {
	if len(hits) == 0 {
		return groups
	}

	pinnedIDs := make(map[uuid.UUID]struct{}, len(hits))
	for _, hit := range hits {
		pinnedIDs[hit.ID] = struct{}{}
	}

	for index, group := range groups {
		if group.Kind != entity.SearchKindIssue {
			continue
		}

		results := make([]entity.SearchResult, 0, len(group.Results)+len(hits))
		results = append(results, hits...)

		for _, result := range group.Results {
			if _, pinned := pinnedIDs[result.ID]; !pinned {
				results = append(results, result)
			}
		}

		groups[index].Results = results

		return groups
	}

	return append([]entity.SearchGroup{{
		Kind:    entity.SearchKindIssue,
		Results: hits,
	}}, groups...)
}

func (s *searchesService) reachDocs(ctx context.Context, request *repository.SearchRequest) error {
	if !slices.Contains(request.Kinds, entity.SearchKindDoc) {
		return nil
	}

	decision, err := s.knowledge.Reader(ctx, request.WorkspaceID)

	var denied entity.AccessDeniedError
	if errors.As(err, &denied) {
		request.Kinds = slices.DeleteFunc(request.Kinds, func(kind entity.SearchKind) bool {
			return kind == entity.SearchKindDoc
		})

		return nil
	}

	if err != nil {
		return err
	}

	reader := decision.Reader

	request.Audience = repository.SearchAudience{
		AllProjects: reader.AllProjects,
		ProjectIDs:  reader.ProjectIDs,
	}

	if reader.Identity.Personal {
		request.Audience.PersonalAuthorID = reader.Identity.Principal
	}

	return nil
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
