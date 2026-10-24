package dashboard_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	doclinksvc "github.com/usenorn/norn/internal/service/doclink"
)

func docLinkEdge(t *testing.T) (*doclinksvc.MockDocLinks, http.Handler) {
	t.Helper()

	ctrl := gomock.NewController(t)
	links := doclinksvc.NewMockDocLinks(ctrl)

	return links, newEdge(ctrl, edgeServices{docLinks: links})
}

func TestLinkingADocTwiceToTheSameIssueAnswersAConflict(t *testing.T) {
	links, routes := docLinkEdge(t)
	workspaceID, docID, issueID := uuid.New(), uuid.New(), uuid.New()

	links.EXPECT().
		Link(gomock.Any(), workspaceID, docID, service.LinkDocInput{IssueID: issueID}).
		Return(entity.DocLink{}, entity.ErrDocLinkExists)

	status, problem := sendDoc(t, routes, http.MethodPost,
		"/workspaces/"+workspaceID.String()+"/docs/"+docID.String()+"/links",
		`{"issueId":"`+issueID.String()+`"}`,
	)

	if status != http.StatusConflict || problem["code"] != "doc_link_exists" {
		t.Fatalf("second link = %d %v, want 409 doc_link_exists", status, problem)
	}
}

func TestALinkToAProjectAnswersWithTheProjectAndNoIssue(t *testing.T) {
	links, routes := docLinkEdge(t)
	workspaceID, docID, projectID := uuid.New(), uuid.New(), uuid.New()

	links.EXPECT().
		Link(gomock.Any(), workspaceID, docID, service.LinkDocInput{ProjectID: projectID}).
		Return(entity.DocLink{
			ID:      uuid.New(),
			DocID:   docID,
			Target:  entity.DocLinkTargetProject,
			Project: entity.Project{ID: projectID, Slug: "handbook", Name: "Handbook"},
		}, nil)

	status, link := sendDoc(t, routes, http.MethodPost,
		"/workspaces/"+workspaceID.String()+"/docs/"+docID.String()+"/links",
		`{"projectId":"`+projectID.String()+`"}`,
	)

	project, _ := link["project"].(map[string]any)
	if status != http.StatusCreated || link["target"] != "project" || link["issue"] != nil ||
		project["slug"] != "handbook" || project["archived"] != false {
		t.Fatalf("project link = %d %v", status, link)
	}
}

func TestAnIssueWithNoReadableDocsAnswersAnEmptyList(t *testing.T) {
	links, routes := docLinkEdge(t)
	workspaceID, issueID := uuid.New(), uuid.New()

	links.EXPECT().ForIssue(gomock.Any(), workspaceID, issueID).Return([]entity.DocSummary{}, nil)

	status, body := sendDoc(t, routes, http.MethodGet,
		"/workspaces/"+workspaceID.String()+"/issues/"+issueID.String()+"/docs", "",
	)

	docs, ok := body["docs"].([]any)
	if status != http.StatusOK || !ok || len(docs) != 0 {
		t.Fatalf("issue docs = %d %v, want 200 with an empty list", status, body)
	}
}
