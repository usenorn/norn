package dashboard_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	agentsvc "github.com/usenorn/norn/internal/service/agent"
	docsvc "github.com/usenorn/norn/internal/service/doc"
)

func docEdge(t *testing.T) (*docsvc.MockDocs, http.Handler) {
	t.Helper()

	ctrl := gomock.NewController(t)
	docs := docsvc.NewMockDocs(ctrl)

	return docs, newEdge(ctrl, edgeServices{docs: docs})
}

func sendDoc(t *testing.T, routes http.Handler, method, path, body string) (int, map[string]any) {
	t.Helper()

	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	routes.ServeHTTP(recorder, request)

	var decoded map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode %s %s: %v (%s)", method, path, err, recorder.Body.String())
	}

	return recorder.Code, decoded
}

func TestAStaleDocEditAnswersAConflictNamingTheCurrentVersion(t *testing.T) {
	docs, routes := docEdge(t)
	workspaceID, docID := uuid.New(), uuid.New()
	title := "Runbook v2"

	docs.EXPECT().
		Update(gomock.Any(), workspaceID, docID, service.UpdateDocInput{ExpectedVersion: 3, Title: &title}).
		Return(entity.Doc{}, entity.DocStaleError{Version: 5})

	status, problem := sendDoc(t, routes, http.MethodPatch,
		"/workspaces/"+workspaceID.String()+"/docs/"+docID.String(),
		`{"expectedVersion":3,"title":"Runbook v2"}`,
	)

	if status != http.StatusConflict || problem["code"] != "doc_stale" || problem["version"] != float64(5) {
		t.Fatalf("stale edit = %d %v, want 409 doc_stale at version 5", status, problem)
	}
}

func TestADocEditByAnOutsiderIsForbiddenWithItsReason(t *testing.T) {
	docs, routes := docEdge(t)
	workspaceID, docID := uuid.New(), uuid.New()

	docs.EXPECT().
		Archive(gomock.Any(), workspaceID, docID, 2).
		Return(entity.Doc{}, entity.ErrKnowledgeNotEditable)

	status, problem := sendDoc(t, routes, http.MethodDelete,
		"/workspaces/"+workspaceID.String()+"/docs/"+docID.String()+"?expectedVersion=2", "",
	)

	if status != http.StatusForbidden || problem["reason"] != "knowledge_not_editable" {
		t.Fatalf("outsider archive = %d %v, want 403 knowledge_not_editable", status, problem)
	}
}

func TestADocReadsOutWithItsAudienceAndContent(t *testing.T) {
	docs, routes := docEdge(t)
	workspaceID, projectID := uuid.New(), uuid.New()
	content := entity.DocumentFromMarkdown("Step one.")
	doc := entity.Doc{
		DocSummary: entity.DocSummary{
			ID:          uuid.New(),
			WorkspaceID: workspaceID,
			Title:       "Release checklist",
			Audience:    entity.ProjectAudience(projectID),
			AuthorID:    uuid.New(),
			AuthorName:  "Rae",
			Status:      entity.DocStatusActive,
			Version:     4,
			CreatedAt:   time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC),
		},
		Content:  content,
		Markdown: content.Markdown(),
	}

	docs.EXPECT().Create(gomock.Any(), workspaceID, gomock.Any()).
		DoAndReturn(func(_ any, _ uuid.UUID, input service.CreateDocInput) (entity.Doc, error) {
			if input.Audience != entity.ProjectAudience(projectID) || input.Markdown != "Step one." || input.Title != doc.Title {
				t.Fatalf("the create request reached the service as %+v", input)
			}

			return doc, nil
		})

	status, body := sendDoc(t, routes, http.MethodPost,
		"/workspaces/"+workspaceID.String()+"/docs",
		`{"title":"Release checklist","markdown":"Step one.","audience":{"visibility":"project","projectId":"`+projectID.String()+`"}}`,
	)

	if status != http.StatusCreated {
		t.Fatalf("create = %d %v", status, body)
	}

	audience, _ := body["audience"].(map[string]any)
	contentBody, _ := body["content"].(map[string]any)

	if audience["visibility"] != "project" || audience["projectId"] != projectID.String() ||
		contentBody["type"] != "doc" || body["markdown"] != "Step one." || body["version"] != float64(4) ||
		body["authorName"] != "Rae" {
		t.Fatalf("created doc read out as %v", body)
	}

	if contributors, ok := body["contributorIds"].([]any); !ok || len(contributors) != 0 {
		t.Fatalf("contributorIds = %v, want an empty list rather than null", body["contributorIds"])
	}
}

func TestAHeldDocEditReadsOutBesideTheDocAsItIsNow(t *testing.T) {
	ctrl := gomock.NewController(t)
	agents := agentsvc.NewMockAgents(ctrl)
	routes := newEdge(ctrl, edgeServices{agents: agents})
	workspaceID, docID := uuid.New(), uuid.New()

	agents.EXPECT().Waiting(gomock.Any(), workspaceID).Return([]service.WaitingProposal{{
		Proposal: entity.AgentProposal{
			ID:          uuid.New(),
			WorkspaceID: workspaceID,
			AgentID:     uuid.New(),
			DocID:       docID,
			Action:      entity.AgentActionDocEdit,
			Status:      entity.AgentProposalPending,
			Change: entity.AgentChange{
				ExpectedVersion: 2,
				Doc:             &entity.AgentDocChange{Title: "Runbook", Markdown: "Pin the sha tag."},
			},
		},
		Doc: entity.Doc{DocSummary: entity.DocSummary{ID: docID, Title: "Runbook"}, Markdown: "Push the tag."},
	}}, nil)

	request := httptest.NewRequest(http.MethodGet, "/workspaces/"+workspaceID.String()+"/agent-proposals", nil)
	recorder := httptest.NewRecorder()
	routes.ServeHTTP(recorder, request)

	var proposals []map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &proposals); err != nil || len(proposals) != 1 {
		t.Fatalf("decode proposals: %v (%s)", err, recorder.Body.String())
	}

	proposal := proposals[0]
	held, _ := proposal["held"].(map[string]any)

	if proposal["docId"] != docID.String() || proposal["markdown"] != "Pin the sha tag." ||
		proposal["action"] != "doc_edit" || held["markdown"] != "Push the tag." {
		t.Fatalf("the held doc edit reads out as %v", proposal)
	}

	if _, hasTeam := proposal["teamId"]; hasTeam {
		t.Fatalf("a doc proposal claims a team: %v", proposal)
	}
}
