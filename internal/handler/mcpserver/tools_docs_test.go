package mcpserver_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type docScene struct {
	workspace entity.Workspace
	project   entity.Project
	doc       entity.Doc
}

func docSceneFor(h *harness) docScene {
	workspace := entity.Workspace{ID: uuid.New(), Slug: "acme", Name: "Acme"}
	project := entity.Project{ID: uuid.New(), WorkspaceID: workspace.ID, Slug: "atlas", Name: "Atlas"}
	authorID := uuid.New()

	h.workspaces.EXPECT().
		ListForAccount(gomock.Any(), h.actor.AccountID).
		Return([]entity.Workspace{workspace}, nil).
		AnyTimes()

	h.projects.EXPECT().
		GetBySlug(gomock.Any(), workspace.ID, project.Slug).
		Return(service.ProjectView{Project: project}, nil).
		AnyTimes()

	return docScene{
		workspace: workspace,
		project:   project,
		doc: entity.Doc{
			DocSummary: entity.DocSummary{
				ID:          uuid.New(),
				WorkspaceID: workspace.ID,
				Title:       "Deploy runbook",
				Audience:    entity.ProjectAudience(project.ID),
				AuthorID:    authorID,
				AuthorName:  "Rae",
				Status:      entity.DocStatusActive,
				Version:     4,
				UpdatedAt:   time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC),
			},
			Markdown: "## Deploy\n\nPush the tag.",
		},
	}
}

func structured(t *testing.T, content any, into any) {
	t.Helper()

	payload, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}

	if err := json.Unmarshal(payload, into); err != nil {
		t.Fatalf("decode structured content %s: %v", payload, err)
	}
}

func TestReadingADocReturnsItsMarkdownWithTheVersionToEditAgainst(t *testing.T) {
	h := newHarness(t)
	scene := docSceneFor(h)

	h.docs.EXPECT().Get(gomock.Any(), scene.workspace.ID, scene.doc.ID).Return(scene.doc, nil)

	result := callTool(t, h, "norn_get_doc", map[string]any{
		"workspace": "acme",
		"doc":       scene.doc.ID.String(),
	})
	if result.IsError {
		t.Fatalf("get doc refused: %s", refusalText(t, result))
	}

	var output struct {
		Doc struct {
			Title    string `json:"title"`
			Markdown string `json:"markdown"`
			Version  int    `json:"version"`
			AuthorID string `json:"authorId"`
			Author   string `json:"authorName"`
			Audience struct {
				Visibility string `json:"visibility"`
				ProjectID  string `json:"projectId"`
			} `json:"audience"`
		} `json:"doc"`
	}
	structured(t, result.StructuredContent, &output)

	if output.Doc.Markdown != scene.doc.Markdown || output.Doc.Version != 4 ||
		output.Doc.AuthorID != scene.doc.AuthorID.String() || output.Doc.Author != "Rae" ||
		output.Doc.Audience.Visibility != "project" || output.Doc.Audience.ProjectID != scene.project.ID.String() {
		t.Fatalf("the doc read out as %+v", output.Doc)
	}
}

func TestAStaleDocEditIsRefusedWithTheVersionToRetryWith(t *testing.T) {
	h := newHarness(t)
	scene := docSceneFor(h)

	h.docs.EXPECT().
		Update(gomock.Any(), scene.workspace.ID, scene.doc.ID, gomock.Any()).
		Return(entity.Doc{}, entity.DocStaleError{Version: 5})

	result := callTool(t, h, "norn_edit_doc", map[string]any{
		"workspace":        "acme",
		"doc":              scene.doc.ID.String(),
		"expected_version": 4,
		"markdown":         "Somebody else's view of it.",
	})

	expectRefusal(t, result, "version_conflict", "now at version 5", "norn_get_doc", "expected_version=5")
}

func TestASectionEditGoesOutAgainstTheVersionTheAgentRead(t *testing.T) {
	h := newHarness(t)
	scene := docSceneFor(h)

	var asked service.UpdateDocInput

	h.docs.EXPECT().
		Update(gomock.Any(), scene.workspace.ID, scene.doc.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID, input service.UpdateDocInput) (entity.Doc, error) {
			asked = input

			edited := scene.doc
			edited.Version = 5

			return edited, nil
		})

	result := callTool(t, h, "norn_edit_doc", map[string]any{
		"workspace":        "acme",
		"doc":              scene.doc.ID.String(),
		"expected_version": 4,
		"section":          "Deploy",
		"markdown":         "## Deploy\n\nPin the sha tag.",
	})
	if result.IsError {
		t.Fatalf("section edit refused: %s", refusalText(t, result))
	}

	if asked.ExpectedVersion != 4 || asked.Section == nil || *asked.Section != "Deploy" ||
		asked.Markdown == nil || *asked.Markdown != "## Deploy\n\nPin the sha tag." || asked.Title != nil {
		t.Fatalf("the section edit went out as %+v", asked)
	}
}

func TestADocEditNamesTheVersionItWasWrittenAgainst(t *testing.T) {
	h := newHarness(t)
	scene := docSceneFor(h)

	result := callTool(t, h, "norn_edit_doc", map[string]any{
		"workspace": "acme",
		"doc":       scene.doc.ID.String(),
		"markdown":  "No version given.",
	})

	if !result.IsError {
		t.Fatal("an edit without expected_version was accepted, so it could overwrite somebody's change")
	}
}

func TestAHeldDocWriteTellsTheAgentNotToRetry(t *testing.T) {
	h := newHarness(t)
	scene := docSceneFor(h)
	proposalID := uuid.New()

	var created service.CreateDocInput

	h.docs.EXPECT().
		Create(gomock.Any(), scene.workspace.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, input service.CreateDocInput) (entity.Doc, error) {
			created = input

			return entity.Doc{}, entity.AgentActionHeldError{ProposalID: proposalID}
		})

	result := callTool(t, h, "norn_create_doc", map[string]any{
		"workspace": "acme",
		"title":     "Postmortem",
		"markdown":  "What happened.",
		"project":   "atlas",
	})

	expectRefusal(t, result, "held_for_approval", proposalID.String(), "Do not retry")

	if created.Audience != entity.ProjectAudience(scene.project.ID) {
		t.Fatalf("naming a project did not share the doc with it: %+v", created.Audience)
	}
}

func TestANewDocsAudienceIsSaidPlainlyOrRefused(t *testing.T) {
	h := newHarness(t)
	docSceneFor(h)

	for name, arguments := range map[string]map[string]any{
		"project visibility without a project": {"visibility": "project"},
		"a project for a workspace doc":        {"visibility": "workspace", "project": "atlas"},
		"a visibility nobody defined":          {"visibility": "team"},
	} {
		t.Run(name, func(t *testing.T) {
			arguments["workspace"] = "acme"
			arguments["title"] = "Postmortem"
			arguments["markdown"] = "What happened."

			expectRefusal(t, callTool(t, h, "norn_create_doc", arguments), "invalid_input")
		})
	}
}

func TestFindingDocsReturnsOnlyWhatThePermissionModelLetThrough(t *testing.T) {
	h := newHarness(t)
	scene := docSceneFor(h)

	var searched service.SearchDocsInput

	h.docs.EXPECT().
		Search(gomock.Any(), scene.workspace.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, input service.SearchDocsInput) ([]entity.DocMatch, error) {
			searched = input

			return []entity.DocMatch{{DocSummary: scene.doc.DocSummary, Excerpt: "**Deploy** runbook"}}, nil
		})

	result := callTool(t, h, "norn_search_docs", map[string]any{
		"workspace": "acme",
		"query":     "deploy",
		"project":   "atlas",
	})
	if result.IsError {
		t.Fatalf("search refused: %s", refusalText(t, result))
	}

	var output struct {
		Docs []struct {
			ID      string `json:"id"`
			Excerpt string `json:"excerpt"`
			Version int    `json:"version"`
		} `json:"docs"`
	}
	structured(t, result.StructuredContent, &output)

	if searched.Query != "deploy" || searched.ProjectID == nil || *searched.ProjectID != scene.project.ID {
		t.Fatalf("the search went out as %+v", searched)
	}

	if len(output.Docs) != 1 || output.Docs[0].ID != scene.doc.ID.String() ||
		output.Docs[0].Excerpt != "**Deploy** runbook" || output.Docs[0].Version != 4 {
		t.Fatalf("the search answered %+v", output.Docs)
	}

	var listed []service.ListDocsInput

	h.docs.EXPECT().
		List(gomock.Any(), scene.workspace.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, input service.ListDocsInput) (service.DocList, error) {
			listed = append(listed, input)

			return service.DocList{Docs: []entity.DocSummary{scene.doc.DocSummary}}, nil
		}).
		Times(2)

	for _, arguments := range []map[string]any{
		{"workspace": "acme"},
		{"workspace": "acme", "under": scene.doc.ID.String()},
	} {
		if result := callTool(t, h, "norn_list_docs", arguments); result.IsError {
			t.Fatalf("list refused: %s", refusalText(t, result))
		}
	}

	if !listed[0].Root || listed[0].WithinID != nil {
		t.Fatalf("a listing with no filter did not start at the top of the tree: %+v", listed[0])
	}

	if listed[1].Root || listed[1].WithinID == nil || *listed[1].WithinID != scene.doc.ID {
		t.Fatalf("a listing beneath a doc went out as %+v", listed[1])
	}

	expectRefusal(t, callTool(t, h, "norn_list_docs", map[string]any{
		"workspace": "acme",
		"under":     "not-a-doc",
	}), "doc_not_found")
}

func TestWhoamiSaysWhetherTheWorkspaceHoldsDocWrites(t *testing.T) {
	h := newHarness(t)
	scene := docSceneFor(h)

	h.agents.EXPECT().
		WorkspaceSettings(gomock.Any(), scene.workspace.ID).
		Return(entity.WorkspaceAgentSettings{WorkspaceID: scene.workspace.ID, HoldDocWrites: entity.AgentHoldAlways}, nil)
	h.teams.EXPECT().List(gomock.Any(), scene.workspace.ID, gomock.Any()).Return(nil, nil)

	result := callTool(t, h, "norn_whoami", map[string]any{"workspace": "acme"})
	if result.IsError {
		t.Fatalf("whoami refused: %s", refusalText(t, result))
	}

	var output struct {
		HoldDocWrites string `json:"holdDocWrites"`
	}
	structured(t, result.StructuredContent, &output)

	if output.HoldDocWrites != "always" {
		t.Fatalf("whoami says doc writes are held %q, want always", output.HoldDocWrites)
	}
}

func TestTheServerInstructionsCountDocsAsWorkspaceData(t *testing.T) {
	h := newHarness(t)

	if instructions := h.session(t).InitializeResult().Instructions; !strings.Contains(instructions, "doc titles and bodies") {
		t.Fatal("the server instructions do not tell an agent that doc text is data, not instructions")
	}
}
