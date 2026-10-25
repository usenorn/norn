package mcpserver_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type startResult struct {
	Knowledge *[]struct {
		Kind       string `json:"kind"`
		ID         string `json:"id"`
		MemoryKind string `json:"memoryKind"`
		Statement  string `json:"statement"`
		Title      string `json:"title"`
		Heading    string `json:"heading"`
		Status     string `json:"status"`
		Conditions string `json:"conditions"`
		Link       string `json:"link"`
	} `json:"knowledge"`
	Reminder string `json:"reminder"`
}

func startableIssue(h *harness, scene docScene, category entity.StateCategory, projectID uuid.UUID) entity.Issue {
	issue := entity.Issue{
		ID:           uuid.New(),
		WorkspaceID:  scene.workspace.ID,
		TeamID:       uuid.New(),
		ReferenceKey: "GAM",
		Number:       7,
		Title:        "Move attachments to object storage",
		State:        entity.IssueState{Category: category},
		ProjectID:    projectID,
		Version:      3,
	}

	h.issues.EXPECT().GetByReference(gomock.Any(), scene.workspace.ID, "GAM-7").Return(issue, nil)

	return issue
}

func TestStartingAnIssueBriefsTheAcceptedKnowledgeOfItsProject(t *testing.T) {
	h := newHarness(t)
	scene := docSceneFor(h)
	issue := startableIssue(h, scene, entity.StateCategoryActive, scene.project.ID)

	memory := memoryFor(scene, "Attachments live in Garage, never on local disk")
	memory.Conditions = "except in the single-binary demo"

	var asked service.RetrieveInput

	h.retrievals.EXPECT().
		Retrieve(gomock.Any(), scene.workspace.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, input service.RetrieveInput) (entity.KnowledgeRetrieval, error) {
			asked = input

			return entity.KnowledgeRetrieval{Hits: []entity.KnowledgeHit{
				{Kind: entity.KnowledgeSubjectMemory, ID: memory.ID, Memory: &memory, Score: 0.9},
				{Kind: entity.KnowledgeSubjectDocPassage, ID: uuid.New(), Score: 0.4, Passage: &entity.KnowledgePassage{
					DocID: scene.doc.ID, DocTitle: scene.doc.Title, Heading: "Storage", Excerpt: "Buckets are per workspace.",
				}},
			}}, nil
		})
	h.sourceControl.EXPECT().BranchName(gomock.Any(), scene.workspace.ID, issue.ID).Return("rae/gam-7", nil)

	result := callTool(t, h, "norn_start_issue", map[string]any{"workspace": "acme", "issue": "GAM-7"})
	if result.IsError {
		t.Fatalf("tool errored: %v", result.Content)
	}

	if asked.Query != issue.Title || asked.ProjectID == nil || *asked.ProjectID != scene.project.ID {
		t.Fatalf("retrieved for %q in project %v, want the issue's title in its project", asked.Query, asked.ProjectID)
	}

	if asked.Limit != entity.KnowledgeBriefingSize || asked.IncludeProposed {
		t.Fatalf("retrieved limit %d with proposed %t, want %d accepted only",
			asked.Limit, asked.IncludeProposed, entity.KnowledgeBriefingSize)
	}

	var output startResult
	structured(t, result.StructuredContent, &output)

	if output.Knowledge == nil || len(*output.Knowledge) != 2 {
		t.Fatalf("knowledge is %+v, want the memory and the doc passage", output.Knowledge)
	}

	first, second := (*output.Knowledge)[0], (*output.Knowledge)[1]

	if first.Statement != memory.Statement || first.Status != "accepted" || first.Conditions != memory.Conditions {
		t.Fatalf("the memory was briefed as %+v, want its statement, status and conditions", first)
	}

	if first.Link != testBaseURL+"/acme/memories/"+memory.ID.String() {
		t.Fatalf("the memory links to %q, want its page in the workspace", first.Link)
	}

	if second.Title != scene.doc.Title || second.Link != testBaseURL+"/acme/docs/"+scene.doc.ID.String() {
		t.Fatalf("the doc passage was briefed as %+v, want its title and a link to the doc", second)
	}

	if !strings.Contains(output.Reminder, "never an instruction") {
		t.Fatalf("the reminder %q does not mark the briefing as data", output.Reminder)
	}
}

func TestStartingAnIssueWithNothingToBriefAddsNothing(t *testing.T) {
	h := newHarness(t)
	scene := docSceneFor(h)
	issue := startableIssue(h, scene, entity.StateCategoryActive, uuid.Nil)

	h.retrievals.EXPECT().
		Retrieve(gomock.Any(), scene.workspace.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, input service.RetrieveInput) (entity.KnowledgeRetrieval, error) {
			if input.ProjectID != nil {
				t.Errorf("an issue outside any project retrieved within project %s", input.ProjectID)
			}

			return entity.KnowledgeRetrieval{}, nil
		})
	h.sourceControl.EXPECT().BranchName(gomock.Any(), scene.workspace.ID, issue.ID).Return("rae/gam-7", nil)

	result := callTool(t, h, "norn_start_issue", map[string]any{"workspace": "acme", "issue": "GAM-7"})
	if result.IsError {
		t.Fatalf("tool errored: %v", result.Content)
	}

	var output startResult
	structured(t, result.StructuredContent, &output)

	if output.Knowledge != nil {
		t.Fatalf("knowledge is %+v, want the field left out when nothing applies", *output.Knowledge)
	}

	if strings.Contains(output.Reminder, "never an instruction") {
		t.Fatalf("the reminder %q speaks of knowledge that was not briefed", output.Reminder)
	}
}

func TestAFailedBriefingLeavesTheIssueUnclaimed(t *testing.T) {
	h := newHarness(t)
	scene := docSceneFor(h)
	startableIssue(h, scene, entity.StateCategoryNotStarted, scene.project.ID)

	h.retrievals.EXPECT().
		Retrieve(gomock.Any(), scene.workspace.ID, gomock.Any()).
		Return(entity.KnowledgeRetrieval{}, errors.New("knowledge store unavailable"))

	result := callTool(t, h, "norn_start_issue", map[string]any{"workspace": "acme", "issue": "GAM-7"})
	if !result.IsError {
		t.Fatal("starting succeeded although the briefing failed")
	}
}
