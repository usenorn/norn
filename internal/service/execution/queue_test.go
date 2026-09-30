package execution_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
)

func TestTheReviewQueueHoldsEveryKindOfDecisionAndSaysWhichAreTheCallers(t *testing.T) {
	h := newHarness(t)
	theirs := uuid.New()
	someoneElses := uuid.New()

	h.questions.EXPECT().
		ListWaiting(gomock.Any(), gomock.Any(), entity.QuestionWaitingMax).
		Return([]entity.IssueQuestion{
			{ID: uuid.New(), IssueID: theirs, Stage: entity.QuestionStagePlanning},
			{ID: uuid.New(), IssueID: someoneElses, Stage: entity.QuestionStageImplementation},
		}, nil)
	h.executions.EXPECT().
		ListVisible(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ entity.TeamScope, page entity.ExecutionPage) ([]entity.ExecutionListing, error) {
			if len(page.States) != 2 {
				t.Errorf("asked for runs in %v, want the two waiting states", page.States)
			}

			return []entity.ExecutionListing{
				{Execution: entity.Execution{ID: "exec-plan", IssueID: theirs, State: entity.ExecutionAwaitingPlan}},
				{Execution: entity.Execution{ID: "exec-review", IssueID: someoneElses, State: entity.ExecutionAwaitingReview}},
			}, nil
		})
	h.delegates.EXPECT().
		Authorities(gomock.Any(), h.workspaceID, gomock.Any()).
		Return(map[uuid.UUID]entity.DecisionAuthority{
			theirs:       {AssigneeAccountID: h.caller, AssigneeKind: entity.AccountKindPerson, AssigneeName: "Rae"},
			someoneElses: {AssigneeAccountID: uuid.New(), AssigneeKind: entity.AccountKindPerson, AssigneeName: "Sam"},
		}, nil)

	queue, err := h.service.Queue(context.Background(), h.workspaceID)
	if err != nil {
		t.Fatalf("read the queue: %v", err)
	}

	if len(queue.Questions) != 2 || len(queue.Plans) != 1 || len(queue.Changes) != 1 {
		t.Fatalf("queue %+v, want two questions, one plan and one set of changes", queue)
	}

	if queue.Plans[0].Listing.Execution.ID != "exec-plan" || queue.Changes[0].Listing.Execution.ID != "exec-review" {
		t.Fatalf("plans %+v and changes %+v are in the wrong sections", queue.Plans, queue.Changes)
	}

	if !queue.Questions[0].Right.CanDecide || !queue.Plans[0].Right.CanDecide {
		t.Error("the assignee is not offered the decisions on their own issue")
	}

	if queue.Questions[1].Right.CanDecide || queue.Changes[0].Right.CanDecide {
		t.Error("a member is offered a decision on somebody else's issue")
	}

	if queue.Changes[0].Right.MakerName != "Sam" {
		t.Errorf("the queue says %q decides, want Sam", queue.Changes[0].Right.MakerName)
	}
}
