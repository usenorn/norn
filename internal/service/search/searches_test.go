package search_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	searchrepo "github.com/usenorn/norn/internal/repository/search"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	searchsvc "github.com/usenorn/norn/internal/service/search"
)

type harness struct {
	search     *searchrepo.MockSearch
	issues     *issuerepo.MockIssue
	authorizer *authorizersvc.MockAuthorizer
	transactor *transactorrepo.MockTransactor
	service    service.Searches

	workspaceID uuid.UUID
	accountID   uuid.UUID
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		search:      searchrepo.NewMockSearch(ctrl),
		issues:      issuerepo.NewMockIssue(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		transactor:  transactorrepo.NewMockTransactor(ctrl),
		workspaceID: uuid.New(),
		accountID:   uuid.New(),
	}

	h.service = searchsvc.New(h.search, h.issues, h.authorizer, h.transactor)

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Actor: entity.Actor{AccountID: h.accountID, Kind: entity.ActorKindUser},
			Scope: entity.TeamScope{WorkspaceID: h.workspaceID, AllTeams: true},
		}, nil).
		AnyTimes()

	h.transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	return h
}

func (h *harness) expectGroups(groups ...entity.SearchGroup) {
	h.search.EXPECT().Search(gomock.Any(), gomock.Any()).Return(groups, nil)
}

func issueGroup(results ...entity.SearchResult) entity.SearchGroup {
	return entity.SearchGroup{Kind: entity.SearchKindIssue, Results: results}
}

func TestAnExactReferenceIsTheFirstResultEvenWhenItsWordsAreNowhereInTheTitle(t *testing.T) {
	h := newHarness(t)
	referenced := uuid.New()
	other := uuid.New()

	h.expectGroups(issueGroup(entity.SearchResult{
		Kind: entity.SearchKindIssue, ID: other, Title: "Something else entirely",
	}))

	h.issues.EXPECT().
		GetVisibleByReference(gomock.Any(), h.workspaceID, entity.IssueReference{Key: "ENG", Number: 412}, gomock.Any()).
		Return(entity.Issue{
			ID:           referenced,
			Title:        "Rotate the signing key",
			ReferenceKey: "ENG",
			Number:       412,
			Status:       entity.IssueStatusActive,
		}, nil)

	results, err := h.service.Search(context.Background(), h.workspaceID, service.SearchInput{Query: "ENG-412"})
	if err != nil {
		t.Fatalf("searching failed: %v", err)
	}

	first := results.Groups[0].Results[0]

	if first.ID != referenced {
		t.Fatalf(
			"the first result is %q, not the referenced issue. Typing a reference is the one "+
				"case where the reader already knows exactly what they want, and the reference "+
				"row need not match the text query at all — ENG-412 tokenises to eng and 412.",
			first.Title,
		)
	}
}

func TestAReferenceThatAlsoMatchesTheTextAppearsOnlyOnce(t *testing.T) {
	h := newHarness(t)
	referenced := uuid.New()

	h.expectGroups(issueGroup(entity.SearchResult{
		Kind: entity.SearchKindIssue, ID: referenced, Title: "ENG-412 rollout",
	}))

	h.issues.EXPECT().
		GetVisibleByReference(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.Issue{
			ID: referenced, Title: "ENG-412 rollout", ReferenceKey: "ENG", Number: 412,
			Status: entity.IssueStatusActive,
		}, nil)

	results, _ := h.service.Search(context.Background(), h.workspaceID, service.SearchInput{Query: "ENG-412"})

	if len(results.Groups[0].Results) != 1 {
		t.Fatalf("the referenced issue appears %d times", len(results.Groups[0].Results))
	}
}

func TestAnArchivedIssueIsNeverPinnedByItsReference(t *testing.T) {
	h := newHarness(t)

	h.expectGroups(issueGroup())

	h.issues.EXPECT().
		GetVisibleByReference(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.Issue{
			ID: uuid.New(), ReferenceKey: "ENG", Number: 412, Status: entity.IssueStatusArchived,
		}, nil)

	h.search.EXPECT().Fuzzy(gomock.Any(), gomock.Any()).Return([]entity.SearchGroup{}, nil)

	results, _ := h.service.Search(context.Background(), h.workspaceID, service.SearchInput{Query: "ENG-412"})

	for _, group := range results.Groups {
		if len(group.Results) > 0 {
			t.Fatal(
				"an archived issue was pinned by reference. Archiving takes work out of " +
					"circulation, and the reference path bypasses the predicate that enforces that.",
			)
		}
	}
}

func TestFuzzyMatchingRunsOnlyWhenEveryExactLaneCameBackEmpty(t *testing.T) {
	h := newHarness(t)

	h.expectGroups(issueGroup(entity.SearchResult{Kind: entity.SearchKindIssue, ID: uuid.New()}))
	h.search.EXPECT().Fuzzy(gomock.Any(), gomock.Any()).Times(0)

	results, err := h.service.Search(context.Background(), h.workspaceID, service.SearchInput{Query: "payments"})
	if err != nil {
		t.Fatalf("searching failed: %v", err)
	}

	if results.Fuzzy {
		t.Fatal(
			"fuzzy results were mixed in alongside exact ones. A 0.35-similarity guess sitting " +
				"next to a real title match reads as a bug, not as helpfulness.",
		)
	}
}

func TestFuzzyMatchingRescuesATypo(t *testing.T) {
	h := newHarness(t)

	h.expectGroups(issueGroup())
	h.search.EXPECT().
		Fuzzy(gomock.Any(), gomock.Any()).
		Return([]entity.SearchGroup{issueGroup(entity.SearchResult{
			Kind: entity.SearchKindIssue, ID: uuid.New(), Title: "Payments retry twice",
		})}, nil)

	results, err := h.service.Search(context.Background(), h.workspaceID, service.SearchInput{Query: "paymnets"})
	if err != nil {
		t.Fatalf("searching failed: %v", err)
	}

	if !results.Fuzzy {
		t.Fatal("the results are not marked as fuzzy, so the screen cannot say why they are approximate")
	}

	if len(results.Groups[0].Results) != 1 {
		t.Fatal("the typo was not rescued")
	}
}

func TestAQueryWithNothingToMatchOnIsRefusedRatherThanRunAsAWildcard(t *testing.T) {
	h := newHarness(t)

	h.search.EXPECT().Search(gomock.Any(), gomock.Any()).Times(0)

	for _, raw := range []string{"", "   ", "!!!"} {
		if _, err := h.service.Search(
			context.Background(), h.workspaceID, service.SearchInput{Query: raw},
		); err == nil {
			t.Errorf(
				"searching for %q reached the repository. A query with no searchable token "+
					"matches every row the reader can see, which is a workspace dump, not a search.",
				raw,
			)
		}
	}
}

func TestAnUnknownKindIsDroppedRatherThanQueried(t *testing.T) {
	h := newHarness(t)

	h.search.EXPECT().
		Search(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request repository.SearchRequest) ([]entity.SearchGroup, error) {
			if len(request.Kinds) != 0 {
				t.Errorf("an unrecognised kind reached the repository as %v", request.Kinds)
			}

			return []entity.SearchGroup{}, nil
		})

	h.search.EXPECT().Fuzzy(gomock.Any(), gomock.Any()).Return([]entity.SearchGroup{}, nil)

	if _, err := h.service.Search(context.Background(), h.workspaceID, service.SearchInput{
		Query: "payments",
		Kinds: []entity.SearchKind{"wallet"},
	}); err != nil {
		t.Fatalf("an unknown kind produced an error rather than being ignored: %v", err)
	}
}

func TestSearchCarriesTheNarrowedTeamScopeToTheRepository(t *testing.T) {
	ctrl := gomock.NewController(t)

	searchRepo := searchrepo.NewMockSearch(ctrl)
	authorizer := authorizersvc.NewMockAuthorizer(ctrl)
	transactor := transactorrepo.NewMockTransactor(ctrl)

	workspaceID, grantedTeam := uuid.New(), uuid.New()

	authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Actor: entity.Actor{AccountID: uuid.New(), Kind: entity.ActorKindToken},
			Scope: entity.TeamScope{
				WorkspaceID: workspaceID,
				TeamIDs:     []uuid.UUID{grantedTeam},
			},
		}, nil)

	searchRepo.EXPECT().
		Search(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request repository.SearchRequest) ([]entity.SearchGroup, error) {
			if request.Scope.AllTeams {
				t.Error(
					"a narrowed actor searched with an all-teams scope; results would reach " +
						"beyond the teams its credential was narrowed to",
				)
			}

			if !request.Scope.Covers(grantedTeam) {
				t.Error("the granted team is missing from the search scope")
			}

			return []entity.SearchGroup{issueGroup(entity.SearchResult{
				Kind: entity.SearchKindIssue, ID: uuid.New(), Title: "payments",
			})}, nil
		})

	searches := searchsvc.New(searchRepo, issuerepo.NewMockIssue(ctrl), authorizer, transactor)

	if _, err := searches.Search(context.Background(), workspaceID, service.SearchInput{
		Query: "payments",
	}); err != nil {
		t.Fatalf("Search: %v", err)
	}
}
