package search_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func TestABareNumberPinsTheIssueThatCarriesIt(t *testing.T) {
	h := newHarness(t)
	wanted := uuid.New()
	other := uuid.New()

	h.expectGroups(issueGroup(entity.SearchResult{
		Kind: entity.SearchKindIssue, ID: other, Title: "Retry after 55 seconds",
	}))

	h.issues.EXPECT().
		ListVisibleByNumber(gomock.Any(), h.workspaceID, 55, entity.SearchPinnedMax, gomock.Any()).
		Return([]entity.Issue{{
			ID:           wanted,
			Title:        "Rotate the signing key",
			ReferenceKey: "GAM",
			TeamKey:      "GAM",
			Number:       55,
			Status:       entity.IssueStatusActive,
		}}, nil)

	results, err := h.service.Search(context.Background(), h.workspaceID, service.SearchInput{Query: "55"})
	if err != nil {
		t.Fatalf("searching failed: %v", err)
	}

	first := results.Groups[0].Results[0]

	if first.ID != wanted {
		t.Fatalf(
			"the first result is %q, not the issue numbered 55. People quote an issue by its "+
				"number alone, and the text lanes only reach titles that spell that number out.",
			first.Title,
		)
	}

	if first.Reference != "GAM-55" {
		t.Fatalf("the pinned row reads %q, so nobody can tell which team it belongs to", first.Reference)
	}
}

func TestANumberSharedByTeamsPinsEveryOneOfThem(t *testing.T) {
	h := newHarness(t)
	older := uuid.New()
	newer := uuid.New()

	h.expectGroups(issueGroup())

	h.issues.EXPECT().
		ListVisibleByNumber(gomock.Any(), gomock.Any(), 55, gomock.Any(), gomock.Any()).
		Return([]entity.Issue{
			{
				ID: newer, Title: "Billing", ReferenceKey: "BIL", Number: 55,
				Status: entity.IssueStatusActive, UpdatedAt: time.Now(),
			},
			{
				ID: older, Title: "Gameplay", ReferenceKey: "GAM", Number: 55,
				Status: entity.IssueStatusActive, UpdatedAt: time.Now().Add(-time.Hour),
			},
		}, nil)

	results, _ := h.service.Search(context.Background(), h.workspaceID, service.SearchInput{Query: "55"})

	references := make([]string, 0, 2)
	for _, result := range results.Groups[0].Results {
		references = append(references, result.Reference)
	}

	if len(references) != 2 || references[0] != "BIL-55" || references[1] != "GAM-55" {
		t.Fatalf(
			"a number that several teams use pinned %v. A workspace numbers issues per team, so "+
				"the reader picks, and the order the repository returned is the order they read.",
			references,
		)
	}
}

func TestAnArchivedIssueIsNeverPinnedByItsNumber(t *testing.T) {
	h := newHarness(t)

	h.expectGroups(issueGroup())

	h.issues.EXPECT().
		ListVisibleByNumber(gomock.Any(), gomock.Any(), 55, gomock.Any(), gomock.Any()).
		Return([]entity.Issue{{
			ID: uuid.New(), ReferenceKey: "GAM", Number: 55, Status: entity.IssueStatusArchived,
		}}, nil)

	h.search.EXPECT().Fuzzy(gomock.Any(), gomock.Any()).Return([]entity.SearchGroup{}, nil)

	results, _ := h.service.Search(context.Background(), h.workspaceID, service.SearchInput{Query: "55"})

	if !results.Empty() {
		t.Fatal("an archived issue was pinned, so search offers a row that leads nowhere")
	}
}

func TestAReferenceNeverAsksForEveryTeamsCopyOfThatNumber(t *testing.T) {
	h := newHarness(t)

	h.expectGroups(issueGroup())

	h.issues.EXPECT().
		GetVisibleByReference(gomock.Any(), gomock.Any(), entity.IssueReference{Key: "GAM", Number: 55}, gomock.Any()).
		Return(entity.Issue{
			ID: uuid.New(), Title: "Gameplay", ReferenceKey: "GAM", Number: 55,
			Status: entity.IssueStatusActive,
		}, nil)

	if _, err := h.service.Search(
		context.Background(), h.workspaceID, service.SearchInput{Query: "GAM-55"},
	); err != nil {
		t.Fatalf("searching failed: %v", err)
	}
}
