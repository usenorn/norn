package issue

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (s *issuesService) Query(
	ctx context.Context,
	workspaceID uuid.UUID,
	input service.QueryIssuesInput,
) (service.IssueQueryResult, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return service.IssueQueryResult{}, err
	}

	page, err := queryPage(input)
	if err != nil {
		return service.IssueQueryResult{}, err
	}

	issues, err := s.issues.ListVisible(ctx, decision.Scope, page.Lookahead())
	if err != nil {
		return service.IssueQueryResult{}, err
	}

	result := service.IssueQueryResult{Issues: issues}

	if len(issues) > page.Limit {
		result.Issues = issues[:page.Limit]

		last := result.Issues[len(result.Issues)-1]
		result.NextCursor = entity.IssueQueryCursor{
			Keys:    entity.IssueSortKeys(last, page.Sort),
			IssueID: last.ID,
		}.Encode()
	}

	if input.GroupBy == "" {
		return result, nil
	}

	groups, err := s.issues.TallyByGroup(ctx, decision.Scope, page, input.GroupBy)
	if err != nil {
		return service.IssueQueryResult{}, err
	}

	result.Groups = groups

	return result, nil
}

func queryPage(input service.QueryIssuesInput) (entity.IssuePage, error) {
	if input.Filter != nil {
		if err := input.Filter.Validate(); err != nil {
			return entity.IssuePage{}, err
		}
	}

	if input.GroupBy != "" && !input.GroupBy.Valid() {
		return entity.IssuePage{}, entity.NewValidationError(entity.FieldError{
			Field: "groupBy",
			Code:  entity.ValidationCodeUnsupportedValue,
		})
	}

	sort, err := entity.NormalizedIssueSort(input.Sort)
	if err != nil {
		return entity.IssuePage{}, err
	}

	page := entity.IssuePage{
		Limit:    input.Limit,
		Text:     entity.ParseSearchQuery(input.Text),
		Filter:   input.Filter,
		Sort:     sort,
		Statuses: entity.RequestedIssueStatuses(nil),
	}.Normalized()

	if statusFiltered(input.Filter) {
		page.Statuses = nil
	}

	if input.Cursor == "" {
		return page, nil
	}

	cursor, err := entity.DecodeIssueQueryCursor(input.Cursor)
	if err != nil {
		return entity.IssuePage{}, entity.NewValidationError(entity.FieldError{
			Field: "cursor",
			Code:  entity.ValidationCodeUnsupportedValue,
		})
	}

	page.QueryCursor = &cursor

	return page, nil
}

func statusFiltered(filter *entity.IssueFilter) bool {
	if filter == nil {
		return false
	}

	if filter.Field == entity.IssueFilterFieldStatus {
		return true
	}

	if filter.Not != nil && statusFiltered(filter.Not) {
		return true
	}

	for _, branch := range append(append([]entity.IssueFilter{}, filter.All...), filter.Any...) {
		if statusFiltered(&branch) {
			return true
		}
	}

	return false
}
