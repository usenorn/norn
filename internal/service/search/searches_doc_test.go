package search_test

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

func (h *harness) captureRequest() *repository.SearchRequest {
	captured := &repository.SearchRequest{}

	h.search.EXPECT().
		Search(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request repository.SearchRequest) ([]entity.SearchGroup, error) {
			*captured = request

			return []entity.SearchGroup{issueGroup(entity.SearchResult{
				Kind: entity.SearchKindIssue, ID: uuid.New(), Title: "Failover runbook",
			})}, nil
		})

	return captured
}

func TestDocsAreSearchedWithinTheReadersOwnProjectsAndPersonalDocs(t *testing.T) {
	h := newHarness(t)
	project := uuid.New()
	h.reader.ProjectIDs = []uuid.UUID{project}

	request := h.captureRequest()

	if _, err := h.service.Search(context.Background(), h.workspaceID, service.SearchInput{Query: "runbook"}); err != nil {
		t.Fatalf("Search: %v", err)
	}

	if !slices.Contains(request.Kinds, entity.SearchKindDoc) {
		t.Fatalf("an unfiltered search left docs out: %v", request.Kinds)
	}

	if request.Audience.AllProjects || !slices.Equal(request.Audience.ProjectIDs, []uuid.UUID{project}) {
		t.Errorf("the doc audience reached %+v, want exactly the reader's own project", request.Audience)
	}

	if request.Audience.PersonalAuthorID != h.accountID {
		t.Errorf("a person searching did not reach their own personal docs: %+v", request.Audience)
	}
}

func TestASharedIdentityNeverReachesAnyonesPersonalDocs(t *testing.T) {
	h := newHarness(t)
	h.reader.Identity = entity.KnowledgeIdentity{Principal: h.accountID, Shared: true}

	request := h.captureRequest()

	if _, err := h.service.Search(context.Background(), h.workspaceID, service.SearchInput{Query: "runbook"}); err != nil {
		t.Fatalf("Search: %v", err)
	}

	if request.Audience.PersonalAuthorID != uuid.Nil {
		t.Errorf(
			"a workspace-shared identity searched as personal author %s. Personal docs belong to one "+
				"person; an agent everyone can drive would hand them to whoever asked it.",
			request.Audience.PersonalAuthorID,
		)
	}
}

func TestTheKnowledgeAudienceIsNotResolvedWhenDocsAreNotSearched(t *testing.T) {
	h := newHarness(t)
	h.captureRequest()

	if _, err := h.service.Search(context.Background(), h.workspaceID, service.SearchInput{
		Query: "runbook",
		Kinds: []entity.SearchKind{entity.SearchKindIssue},
	}); err != nil {
		t.Fatalf("Search: %v", err)
	}

	if h.readerCalls != 0 {
		t.Errorf("an issue-only search listed the reader's projects %d times for nothing", h.readerCalls)
	}
}

func TestADeniedKnowledgeReaderDropsDocsAndKeepsTheRestOfTheSearch(t *testing.T) {
	h := newHarness(t)
	h.readerErr = entity.AccessDeniedError{Reason: entity.DenyReasonRoleLacksAction, Resource: entity.ResourceKnowledge}

	request := h.captureRequest()

	results, err := h.service.Search(context.Background(), h.workspaceID, service.SearchInput{Query: "runbook"})
	if err != nil {
		t.Fatalf("a caller who may not read docs lost the whole search: %v", err)
	}

	if slices.Contains(request.Kinds, entity.SearchKindDoc) {
		t.Error("docs were still searched for a caller denied knowledge access")
	}

	if !slices.Contains(request.Kinds, entity.SearchKindIssue) || results.Empty() {
		t.Errorf("the other kinds went missing alongside docs: %v", request.Kinds)
	}
}
