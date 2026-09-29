package execution_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func (h *harness) planned(body string) {
	h.proposed = append(h.proposed, entity.ExecutionPlan{
		ID:          uuid.New(),
		ExecutionID: "exec-01ABC",
		Revision:    len(h.proposed) + 1,
		Ref:         uuid.NewString(),
		Body:        body,
		ProposedAt:  time.Now().UTC(),
	})
}

func (h *harness) waitingOnPlan() entity.Execution {
	execution := h.execution(entity.ExecutionAwaitingPlan)
	execution.Stage = entity.StagePlanning
	h.holding(execution)
	h.moving()
	h.states.EXPECT().ListByTeamID(gomock.Any(), gomock.Any()).Return(h.states34(), nil).AnyTimes()

	return execution
}

func (h *harness) proposal(t *testing.T, ref, body string) entity.ChannelMessage {
	t.Helper()

	payload, err := json.Marshal(channelv1.Plan{Ref: ref, Body: body, Proposed: time.Now().UTC()})
	if err != nil {
		t.Fatalf("encode the plan: %v", err)
	}

	return entity.ChannelMessage{
		ID:          uuid.NewString(),
		Type:        entity.ChannelPlanProposed,
		ExecutionID: "exec-01ABC",
		Payload:     payload,
		IssuedAt:    time.Now().UTC(),
	}
}

func TestAProposedPlanIsKeptAsTheNextRevisionAndSaidOnTheTimeline(t *testing.T) {
	h := newHarness(t)

	execution := h.execution(entity.ExecutionRunning)
	execution.Stage = entity.StagePlanning
	h.holding(execution)

	for _, ref := range []string{"plan-1", "plan-1", "plan-2"} {
		if err := h.service.PlanProposed(
			context.Background(), h.runner, h.proposal(t, ref, "Add the column, then backfill it."),
		); err != nil {
			t.Fatalf("propose %s: %v", ref, err)
		}
	}

	if len(h.proposed) != 2 || h.proposed[1].Revision != 2 {
		t.Fatalf(
			"kept %d revisions; a replayed proposal must not become a second copy of the same plan",
			len(h.proposed),
		)
	}

	if _, ok := h.announced(entity.EventExecutionPlan); !ok {
		t.Fatal("nobody watching the run heard that a plan is waiting for them")
	}

	if len(h.recorded) == 0 || h.recorded[0].Kind != entity.ExecutionEventPhase {
		t.Fatalf("the timeline recorded %+v, want a phase entry naming the revision", h.recorded)
	}
}

func TestARunThatIsImplementingCannotProposeAnotherPlan(t *testing.T) {
	h := newHarness(t)

	execution := h.execution(entity.ExecutionRunning)
	execution.Stage = entity.StageImplementation
	h.holding(execution)

	err := h.service.PlanProposed(context.Background(), h.runner, h.proposal(t, "late", "Start over."))
	if !errors.Is(err, entity.ErrExecutionNotPlanning) {
		t.Fatalf("a plan arrived mid-implementation and got %v; it would reopen an approved plan", err)
	}
}

func TestARunCannotWaitForApprovalWithoutAPlanToApprove(t *testing.T) {
	h := newHarness(t)

	execution := h.execution(entity.ExecutionRunning)
	execution.Stage = entity.StagePlanning
	h.holding(execution)

	payload, _ := json.Marshal(channelv1.Report{State: string(entity.ExecutionAwaitingPlan)})

	err := h.service.Reported(context.Background(), h.runner, entity.ChannelMessage{
		ID: uuid.NewString(), Type: entity.ChannelExecutionState, ExecutionID: execution.ID,
		Payload: payload, IssuedAt: time.Now().UTC(),
	})
	if !errors.Is(err, entity.ErrExecutionPlanMissing) {
		t.Fatalf(
			"a run parked for approval with nothing proposed and got %v; the person asked to "+
				"approve would have nothing to read",
			err,
		)
	}
}

func TestApprovingThePlanResumesTheSameRunIntoImplementationWithThePlan(t *testing.T) {
	h := newHarness(t)
	execution := h.waitingOnPlan()
	h.planned("1. Add the migration.\n2. Backfill.")

	approved, err := h.service.ApprovePlan(context.Background(), h.workspaceID, execution.ID, 1)
	if err != nil {
		t.Fatalf("approve the plan: %v", err)
	}

	if approved.State != entity.ExecutionQueuedForResume || approved.Stage != entity.StageImplementation {
		t.Fatalf("the run is %s in %s, want queued_for_resume in implementation", approved.State, approved.Stage)
	}

	instruction := h.instruction(t)

	switch {
	case instruction.Reason != channelv1.ResumePlanApproved:
		t.Fatalf("the resume said %q, want %q", instruction.Reason, channelv1.ResumePlanApproved)
	case instruction.Stage != entity.StageImplementation:
		t.Fatalf("the machine was told to resume in %q, want implementation", instruction.Stage)
	case instruction.Instruction != h.proposed[0].Body:
		t.Fatalf("the machine was handed %q rather than the plan that was approved", instruction.Instruction)
	}

	if h.proposed[0].ApprovedByAccountID != h.caller {
		t.Fatal("the plan does not say who approved it")
	}
}

func TestAPlanCannotBeApprovedWhileTheRunStillWaitsOnAQuestion(t *testing.T) {
	h := newHarness(t)
	execution := h.waitingOnPlan()
	h.planned("Rename the table.")
	h.asked = []entity.IssueQuestion{{
		ExecutionID: execution.ID, Blocking: true, State: entity.QuestionAsked, Question: "Keep the old name?",
	}}

	_, err := h.service.ApprovePlan(context.Background(), h.workspaceID, execution.ID, 1)
	if !errors.Is(err, entity.ErrExecutionQuestionsOpen) {
		t.Fatalf("a plan was approved over an unanswered blocking question and got %v", err)
	}

	if _, moved := h.sent(entity.ChannelExecutionResume); moved {
		t.Fatal("the run was told to implement while a question it is blocked on is still open")
	}
}

func TestOnlyTheNewestRevisionOfAPlanCanBeApproved(t *testing.T) {
	h := newHarness(t)
	execution := h.waitingOnPlan()
	h.planned("First idea.")
	h.planned("Second idea.")

	_, err := h.service.ApprovePlan(context.Background(), h.workspaceID, execution.ID, 1)
	if !errors.Is(err, entity.ErrExecutionPlanStale) {
		t.Fatalf(
			"revision 1 was approved after revision 2 replaced it and got %v; the approver would "+
				"authorise a plan the run no longer means to follow",
			err,
		)
	}
}

func TestAnAgentCannotApproveItsOwnPlan(t *testing.T) {
	h := newHarness(t)
	execution := h.waitingOnPlan()
	h.planned("Trust me.")

	itself := execution.AgentID
	h.callerAgent = &itself

	if _, err := h.service.ApprovePlan(
		context.Background(), h.workspaceID, execution.ID, 1,
	); !errors.Is(err, entity.ErrExecutionSelfApproval) {
		t.Fatalf("an agent approved its own plan and got %v", err)
	}
}

func TestAskingForARevisionKeepsTheRunPlanning(t *testing.T) {
	h := newHarness(t)
	execution := h.waitingOnPlan()
	h.planned("Drop the table.")

	revising, err := h.service.RevisePlan(
		context.Background(), h.workspaceID, execution.ID, 1, "Keep the data; archive it instead.",
	)
	if err != nil {
		t.Fatalf("ask for a revision: %v", err)
	}

	if revising.Stage != entity.StagePlanning {
		t.Fatalf("asking for a revision moved the run to %s; nothing was approved", revising.Stage)
	}

	instruction := h.instruction(t)
	if instruction.Reason != channelv1.ResumePlanRevision ||
		instruction.Instruction != "Keep the data; archive it instead." {
		t.Fatalf("the machine was told %+v, want the feedback verbatim as a revision", instruction)
	}

	if h.proposed[0].Approved() {
		t.Fatal("asking for a revision approved the plan")
	}
}

func TestAnsweringAQuestionWhileAPlanWaitsApprovesNothing(t *testing.T) {
	h := newHarness(t)
	execution := h.waitingOnPlan()
	h.planned("Ship it.")

	answeredAt := time.Now().UTC()
	question := entity.IssueQuestion{
		ID: uuid.New(), ExecutionID: execution.ID, Kind: entity.QuestionApproval,
		State: entity.QuestionAnswered, Question: "Is this plan fine?", Answer: "Yes, approved",
		AnsweredAt: &answeredAt, SettledAt: &answeredAt, SettledBy: h.caller,
	}

	if err := h.service.Answered(context.Background(), question); err != nil {
		t.Fatalf("answer: %v", err)
	}

	if _, resumed := h.sent(entity.ChannelExecutionResume); resumed {
		t.Fatal("answering a question sent the run on; only approving the plan may do that")
	}

	if h.proposed[0].Approved() {
		t.Fatal("answering \"approved\" to a question approved the plan")
	}
}

func TestAPlanWaitingForApprovalTellsWhoeverDecidesForTheIssue(t *testing.T) {
	for _, tc := range []struct {
		name     string
		assignee entity.AccountKind
		want     func(h *harness) uuid.UUID
	}{
		{name: "the person assigned", assignee: entity.AccountKindPerson, want: func(h *harness) uuid.UUID { return h.caller }},
		{name: "the delegator when an agent is assigned", assignee: entity.AccountKindAgent, want: func(h *harness) uuid.UUID { return h.delegator }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.authority.AssigneeKind = tc.assignee

			execution := h.execution(entity.ExecutionRunning)
			execution.Stage = entity.StagePlanning
			h.holding(execution)
			h.moving()
			h.planned("Split the handler in two.")

			payload, _ := json.Marshal(channelv1.Report{State: string(entity.ExecutionAwaitingPlan)})

			if err := h.service.Reported(context.Background(), h.runner, entity.ChannelMessage{
				ID: uuid.NewString(), Type: entity.ChannelExecutionState, ExecutionID: execution.ID,
				Payload: payload, IssuedAt: time.Now().UTC(),
			}); err != nil {
				t.Fatalf("park for approval: %v", err)
			}

			if len(h.notified) != 1 || h.notified[0].Target != tc.want(h) ||
				h.notified[0].Kind != entity.NotificationKindDecisionWaiting {
				t.Fatalf(
					"notified %+v; nobody would know a plan is waiting, and the run sits idle until "+
						"somebody happens to look",
					h.notified,
				)
			}
		})
	}
}

func TestOnlyTheAssigneeOrAnAdminDecidesAPlan(t *testing.T) {
	for _, tc := range []struct {
		name      string
		assignee  bool
		role      entity.MembershipRole
		forbidden bool
	}{
		{name: "the assignee", assignee: true, role: entity.MembershipRoleMember},
		{name: "a workspace admin", role: entity.MembershipRoleAdmin},
		{name: "another member who manages the issue", role: entity.MembershipRoleMember, forbidden: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.role = tc.role

			if !tc.assignee {
				h.authority.AssigneeAccountID = uuid.New()
			}

			execution := h.waitingOnPlan()
			h.planned("Rename the table.")

			_, approveErr := h.service.ApprovePlan(context.Background(), h.workspaceID, execution.ID, 1)

			if !tc.forbidden {
				if approveErr != nil {
					t.Fatalf("approve the plan: %v", approveErr)
				}

				return
			}

			_, reviseErr := h.service.RevisePlan(context.Background(), h.workspaceID, execution.ID, 1, "Smaller steps.")

			if !errors.Is(approveErr, entity.ErrIssueDecisionForbidden) ||
				!errors.Is(reviseErr, entity.ErrIssueDecisionForbidden) {
				t.Fatalf("approve %v, revise %v; want both refused as not theirs to decide", approveErr, reviseErr)
			}

			if _, moved := h.sent(entity.ChannelExecutionResume); moved {
				t.Fatal("a refused decision still resumed the run")
			}
		})
	}
}

func TestAPlanWaitingForApprovalIsSentToTelegram(t *testing.T) {
	h := newHarness(t)

	execution := h.execution(entity.ExecutionRunning)
	execution.Stage = entity.StagePlanning
	h.holding(execution)
	h.moving()
	h.planned("Split the handler in two.")

	payload, _ := json.Marshal(channelv1.Report{State: string(entity.ExecutionAwaitingPlan)})

	if err := h.service.Reported(context.Background(), h.runner, entity.ChannelMessage{
		ID: uuid.NewString(), Type: entity.ChannelExecutionState, ExecutionID: execution.ID,
		Payload: payload, IssuedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("park for approval: %v", err)
	}

	want := entity.TelegramPlanDecision(execution, 1)
	if len(h.relayed) != 1 || h.relayed[0] != want {
		t.Fatalf("queued for telegram %+v, want revision 1 of the plan once", h.relayed)
	}
}

func TestADecidedPlanIsSettledOnTelegram(t *testing.T) {
	for _, decide := range []func(h *harness, id string) error{
		func(h *harness, id string) error {
			_, err := h.service.ApprovePlan(context.Background(), h.workspaceID, id, 1)
			return err
		},
		func(h *harness, id string) error {
			_, err := h.service.RevisePlan(context.Background(), h.workspaceID, id, 1, "Smaller steps.")
			return err
		},
	} {
		h := newHarness(t)
		execution := h.waitingOnPlan()
		h.planned("Rename the table.")

		if err := decide(h, execution.ID); err != nil {
			t.Fatalf("decide the plan: %v", err)
		}

		if len(h.settled) != 1 || h.settled[0].Kind != entity.TelegramDecisionPlan ||
			h.settled[0].ExecutionID != execution.ID {
			t.Fatalf("queued settlements %+v; the telegram message would keep offering a decision already made", h.settled)
		}
	}
}
