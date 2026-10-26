package mcpserver_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func handoffScene(t *testing.T, h *harness) (issueScene, entity.Issue, map[string]entity.WorkflowState) {
	t.Helper()

	scene := sceneFor(t, h)
	states := entity.DefaultWorkflowStates(scene.workspace.ID, scene.team.ID)
	byName := map[string]entity.WorkflowState{}

	for i := range states {
		states[i].ID = uuid.New()
		byName[states[i].Name] = states[i]
	}

	inProgress := byName["In progress"]
	issue := entity.Issue{
		ID: uuid.New(), WorkspaceID: scene.workspace.ID, TeamID: scene.team.ID,
		ReferenceKey: "GAM", Number: 12, Version: 5, Title: "Move attachments",
		State: entity.IssueState{
			ID: inProgress.ID, Name: inProgress.Name, Category: inProgress.Category, Position: inProgress.Position,
		},
	}

	h.issues.EXPECT().GetByReference(gomock.Any(), scene.workspace.ID, "GAM-12").Return(issue, nil)
	h.states.EXPECT().List(gomock.Any(), scene.workspace.ID, scene.team.ID).Return(states, nil).AnyTimes()

	return scene, issue, byName
}

func movingTo(h *harness, scene issueScene, issue entity.Issue, states map[string]entity.WorkflowState) {
	h.issues.EXPECT().
		Update(gomock.Any(), scene.workspace.ID, issue.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID, in service.UpdateIssueInput) (entity.Issue, error) {
			moved := issue
			moved.Version++

			if in.StateID != nil {
				for _, state := range states {
					if state.ID == *in.StateID {
						moved.State = entity.IssueState{
							ID: state.ID, Name: state.Name, Category: state.Category, Position: state.Position,
						}
					}
				}
			}

			return moved, nil
		})
}

func reminderOf(t *testing.T, h *harness, tool string, arguments map[string]any) string {
	t.Helper()

	result := callTool(t, h, tool, arguments)
	if result.IsError {
		t.Fatalf("%s errored: %s", tool, refusalText(t, result))
	}

	var output struct {
		Reminder string `json:"reminder"`
	}
	structured(t, result.StructuredContent, &output)

	return output.Reminder
}

func TestHandingWorkOnToReviewOrDoneAsksWhatItEstablished(t *testing.T) {
	cases := map[string]bool{
		"In review":   true,
		"Done":        true,
		"Todo":        false,
		"In progress": false,
		"Canceled":    false,
	}

	for target, reminds := range cases {
		t.Run(target, func(t *testing.T) {
			h := newHarness(t)
			scene, issue, states := handoffScene(t, h)
			movingTo(h, scene, issue, states)

			reminder := reminderOf(t, h, "norn_change_issue_state", map[string]any{
				"workspace": "acme", "issue": "GAM-12", "state": target,
			})

			if got := strings.Contains(reminder, "norn_remember"); got != reminds {
				t.Fatalf("moving to %s answered with reminder %q, want a capture reminder %t", target, reminder, reminds)
			}
		})
	}
}

func TestAnUpdateThatClosesTheIssueAsksWhatItEstablished(t *testing.T) {
	h := newHarness(t)
	scene, issue, states := handoffScene(t, h)
	movingTo(h, scene, issue, states)

	reminder := reminderOf(t, h, "norn_update_issue", map[string]any{
		"workspace": "acme", "issue": "GAM-12", "state": "Done",
	})

	if !strings.Contains(reminder, "norn_remember") {
		t.Fatalf("closing through an update answered with reminder %q, want the capture reminder", reminder)
	}
}

func TestAnUpdateThatLeavesTheStateAloneAsksNothing(t *testing.T) {
	h := newHarness(t)
	scene, issue, states := handoffScene(t, h)
	movingTo(h, scene, issue, states)

	reminder := reminderOf(t, h, "norn_update_issue", map[string]any{
		"workspace": "acme", "issue": "GAM-12", "title": "Move attachments to Garage",
	})

	if reminder != "" {
		t.Fatalf("retitling answered with reminder %q, want none", reminder)
	}
}
