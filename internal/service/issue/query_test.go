package issue_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func TestATallyIsTakenWithTheSameScopeAndPredicateAsThePage(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	teamID := uuid.New()
	scope := entity.TeamScope{WorkspaceID: workspaceID, TeamIDs: []uuid.UUID{teamID}}

	h.expectScope(workspaceID, scope)

	filter := entity.IssueFilter{
		Field: entity.IssueFilterFieldPriority, Op: entity.IssueFilterOpIs, Values: []string{"urgent"},
	}

	var (
		listed  entity.IssuePage
		tallied entity.IssuePage
		against entity.TeamScope
	)

	h.issues.EXPECT().
		ListVisible(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.TeamScope, page entity.IssuePage) ([]entity.Issue, error) {
			listed = page

			return issuesOf(workspaceID, teamID, 2), nil
		})

	h.issues.EXPECT().
		TallyByGroup(gomock.Any(), gomock.Any(), gomock.Any(), entity.IssueGroupByAssignee).
		DoAndReturn(func(
			_ context.Context, s entity.TeamScope, page entity.IssuePage, _ entity.IssueGroupBy,
		) ([]entity.IssueGroupTally, error) {
			against, tallied = s, page

			return []entity.IssueGroupTally{{Key: "", Issues: 2}}, nil
		})

	result, err := h.service.Query(context.Background(), workspaceID, service.QueryIssuesInput{
		Filter:  &filter,
		GroupBy: entity.IssueGroupByAssignee,
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	if !reflect.DeepEqual(against, scope) {
		t.Fatalf(
			"the tally was taken against %+v, want the caller's own scope %+v. A count taken with a "+
				"wider scope than the page describes work the caller cannot open.",
			against, scope,
		)
	}

	if !reflect.DeepEqual(tallied.Filter, listed.Filter) {
		t.Fatalf(
			"the tally used filter %+v while the page used %+v. A header that counts something other "+
				"than the list beneath it is worse than no header.",
			tallied.Filter, listed.Filter,
		)
	}

	if !reflect.DeepEqual(tallied.Statuses, listed.Statuses) {
		t.Fatalf("the tally counted statuses %v while the page listed %v", tallied.Statuses, listed.Statuses)
	}

	if len(result.Groups) != 1 || result.Groups[0].Issues != 2 {
		t.Fatalf("the tallies did not reach the caller: %+v", result.Groups)
	}
}

func TestAQueryWithNoStatusConditionStillAsksOnlyForActiveIssues(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})

	var requested entity.IssuePage

	h.issues.EXPECT().
		ListVisible(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.TeamScope, page entity.IssuePage) ([]entity.Issue, error) {
			requested = page

			return nil, nil
		})

	_, err := h.service.Query(context.Background(), workspaceID, service.QueryIssuesInput{
		Filter: &entity.IssueFilter{
			Field: entity.IssueFilterFieldPriority, Op: entity.IssueFilterOpIs, Values: []string{"high"},
		},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	if !reflect.DeepEqual(requested.Statuses, []entity.IssueStatus{entity.IssueStatusActive}) {
		t.Fatalf(
			"a query that says nothing about status asked for %v, want only active issues. "+
				"Archived and trashed work reappearing in a filtered list is a regression, not a feature.",
			requested.Statuses,
		)
	}
}

func TestAQueryThatFiltersStatusItselfIsNotOverruled(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})

	var requested entity.IssuePage

	h.issues.EXPECT().
		ListVisible(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.TeamScope, page entity.IssuePage) ([]entity.Issue, error) {
			requested = page

			return nil, nil
		})

	_, err := h.service.Query(context.Background(), workspaceID, service.QueryIssuesInput{
		Filter: &entity.IssueFilter{All: []entity.IssueFilter{
			{Field: entity.IssueFilterFieldStatus, Op: entity.IssueFilterOpIs, Values: []string{"archived"}},
		}},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	if requested.Statuses != nil {
		t.Fatalf(
			"the caller asked for archived issues and the default %v was applied anyway, "+
				"so the two conditions cancel and the query returns nothing",
			requested.Statuses,
		)
	}
}

func TestAQueryRefusesAHostileExpressionBeforeItReachesTheStore(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})

	_, err := h.service.Query(context.Background(), workspaceID, service.QueryIssuesInput{
		Filter: &entity.IssueFilter{
			Field: "id); DROP TABLE workspace_issues; --", Op: entity.IssueFilterOpIs, Values: []string{"1"},
		},
	})

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf(
			"an unknown field returned %v, want a validation error. The store mock expects no call, "+
				"so reaching it would fail the test — but the point is that it must be refused here.",
			err,
		)
	}
}

func TestAQueryPageEndsAtACursorBuiltFromTheRequestedSort(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	teamID := uuid.New()

	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, TeamIDs: []uuid.UUID{teamID}})

	issues := issuesOf(workspaceID, teamID, 3)
	issues[1].Priority = entity.IssuePriorityHigh

	h.issues.EXPECT().
		ListVisible(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(issues, nil)

	sort := []entity.IssueSort{{Field: entity.IssueSortFieldPriority}}

	result, err := h.service.Query(context.Background(), workspaceID, service.QueryIssuesInput{
		Sort:  sort,
		Limit: 2,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	if len(result.Issues) != 2 || result.NextCursor == "" {
		t.Fatalf("a full page returned %d issues and cursor %q", len(result.Issues), result.NextCursor)
	}

	cursor, err := entity.DecodeIssueQueryCursor(result.NextCursor)
	if err != nil {
		t.Fatalf("DecodeIssueQueryCursor: %v", err)
	}

	if cursor.IssueID != result.Issues[1].ID {
		t.Fatal("the cursor names a row other than the last one kept, so the next page skips or repeats")
	}

	if !reflect.DeepEqual(cursor.Keys, entity.IssueSortKeys(result.Issues[1], sort)) {
		t.Fatalf(
			"the cursor carries keys %v, want the sort keys of the boundary row. A cursor whose keys "+
				"do not match the ordering cannot name a position in it.",
			cursor.Keys,
		)
	}
}

func TestAStoredExpressionQueriesTheSameWayAfterARoundTrip(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	h.expectScope(workspaceID, entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true})

	filter := entity.IssueFilter{Any: []entity.IssueFilter{
		{Field: entity.IssueFilterFieldPriority, Op: entity.IssueFilterOpIn, Values: []string{"urgent", "high"}},
		{Not: &entity.IssueFilter{Field: entity.IssueFilterFieldAssignee, Op: entity.IssueFilterOpIsSet}},
	}}

	var requested entity.IssuePage

	h.issues.EXPECT().
		ListVisible(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.TeamScope, page entity.IssuePage) ([]entity.Issue, error) {
			requested = page

			return nil, nil
		})

	stored := filter
	restored := roundTrip(t, stored)

	if _, err := h.service.Query(context.Background(), workspaceID, service.QueryIssuesInput{
		Filter: &restored,
	}); err != nil {
		t.Fatalf("Query: %v", err)
	}

	if !reflect.DeepEqual(*requested.Filter, filter) {
		t.Fatalf(
			"the expression reaching the store after storage is %+v, want %+v. A saved view is "+
				"nothing but its expression; if it changes in storage it names a different set of issues.",
			*requested.Filter, filter,
		)
	}
}

func roundTrip(t *testing.T, filter entity.IssueFilter) entity.IssueFilter {
	t.Helper()

	stored, err := json.Marshal(filter)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var restored entity.IssueFilter

	if err := json.Unmarshal(stored, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	return restored
}
