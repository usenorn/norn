package telegrambot_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

func (h *harness) tapping(data string, decision entity.TelegramDecisionMessage) uuid.UUID {
	id := h.applying(entity.TelegramIncoming{UpdateID: 20, Callback: &entity.TelegramCallback{
		ID: "cb-plan", Sender: entity.TelegramSender{ID: senderID}, ChatID: groupChat, MessageID: 88, Data: data,
	}})
	h.conversation.EXPECT().DecisionAt(gomock.Any(), h.bot.ID, int64(groupChat), int64(88)).Return(decision, nil)

	return id
}

func (h *harness) planMessage(execution entity.Execution, revision int) entity.TelegramDecisionMessage {
	return entity.TelegramDecisionMessage{
		BotID: h.bot.ID, ChatID: groupChat, MessageID: 88, Kind: entity.TelegramDecisionPlan,
		ExecutionID: execution.ID, PlanRevision: revision,
	}
}

func (h *harness) noticed(t *testing.T, id uuid.UUID, want string) {
	t.Helper()

	h.messenger.EXPECT().AnswerCallback(gomock.Any(), botToken, "cb-plan", want).Return(nil)
	h.settles(id, entity.TelegramUpdateApplied)

	if err := h.updatesService().Apply(context.Background(), id); err != nil {
		t.Fatalf("Apply: %v", err)
	}
}

func TestApprovingAPlanFromTelegramApprovesTheRevisionThatWasSentAsTheLinkedPerson(t *testing.T) {
	h := newHarness(t)
	execution := h.running(entity.ExecutionAwaitingPlan)
	accountID := uuid.New()

	id := h.tapping(entity.TelegramCallbackApprove, h.planMessage(execution, 2))
	h.audience.EXPECT().AccountOf(gomock.Any(), h.bot.ID, int64(senderID)).Return(h.linked(accountID, senderID), nil)
	h.decisions.EXPECT().
		ApprovePlan(gomock.Any(), h.workspaceID, execution.ID, 2).
		DoAndReturn(func(ctx context.Context, _ uuid.UUID, _ string, _ int) (entity.Execution, error) {
			if actor, ok := identity.Actor(ctx); !ok || actor.AccountID != accountID {
				t.Errorf("approved as %+v, want the linked account", actor)
			}

			return execution, nil
		})

	h.noticed(t, id, "Plan approved.")
}

func TestAskingForChangesFromTelegramWaitsForTheReply(t *testing.T) {
	h := newHarness(t)
	execution := h.running(entity.ExecutionAwaitingPlan)

	id := h.tapping(entity.TelegramCallbackChanges, h.planMessage(execution, 1))
	h.audience.EXPECT().AccountOf(gomock.Any(), h.bot.ID, int64(senderID)).Return(h.linked(uuid.New(), senderID), nil)

	h.noticed(t, id, "Reply to this message with what should change.")
}

func TestAGroupMemberWhoIsNotTheAssigneeIsToldWhoDecides(t *testing.T) {
	h := newHarness(t)
	execution := h.running(entity.ExecutionAwaitingPlan)
	h.decidedBy(execution.IssueID, uuid.New(), entity.DecisionChannelNorn)

	id := h.tapping(entity.TelegramCallbackApprove, h.planMessage(execution, 1))
	h.audience.EXPECT().AccountOf(gomock.Any(), h.bot.ID, int64(senderID)).Return(h.linked(uuid.New(), senderID), nil)
	h.decisions.EXPECT().
		ApprovePlan(gomock.Any(), h.workspaceID, execution.ID, 1).
		Return(entity.Execution{}, entity.ErrIssueDecisionForbidden)

	h.noticed(t, id, "Only Rae or a workspace admin can decide this.")
}

func TestAnOutdatedPlanRevisionIsRefusedFromTelegram(t *testing.T) {
	h := newHarness(t)
	execution := h.running(entity.ExecutionAwaitingPlan)

	id := h.tapping(entity.TelegramCallbackApprove, h.planMessage(execution, 1))
	h.audience.EXPECT().AccountOf(gomock.Any(), h.bot.ID, int64(senderID)).Return(h.linked(uuid.New(), senderID), nil)
	h.decisions.EXPECT().
		ApprovePlan(gomock.Any(), h.workspaceID, execution.ID, 1).
		Return(entity.Execution{}, entity.ErrExecutionPlanStale)

	h.noticed(t, id, "This is out of date. Open it in Norn to see the latest.")
}

func TestATapOnADecisionAlreadySettledDecidesNothing(t *testing.T) {
	h := newHarness(t)
	execution := h.running(entity.ExecutionApproved)
	settled := h.planMessage(execution, 1)
	settled.Settled = true

	id := h.tapping(entity.TelegramCallbackApprove, settled)

	h.noticed(t, id, "This was already decided.")
}

func TestAReplyToAReviewRequestsChangesOnTheHeadsItWasSentFor(t *testing.T) {
	h := newHarness(t)
	execution := h.running(entity.ExecutionAwaitingReview)
	accountID := uuid.New()
	heads := entity.ReviewHeads{"api": "abc123"}

	incoming := groupChatter("Rename the flag before merging")
	incoming.Message.ReplyToID = 88
	incoming.Message.ReplyToSenderID = botUserID
	id := h.applying(incoming)

	h.audience.EXPECT().Group(gomock.Any(), h.bot.ID, int64(groupChat)).Return(entity.TelegramGroup{}, nil)
	h.audience.EXPECT().AccountOf(gomock.Any(), h.bot.ID, int64(senderID)).Return(h.linked(accountID, senderID), nil)
	h.conversation.EXPECT().DecisionAt(gomock.Any(), h.bot.ID, int64(groupChat), int64(88)).Return(entity.TelegramDecisionMessage{
		BotID: h.bot.ID, ChatID: groupChat, MessageID: 88, Kind: entity.TelegramDecisionReview,
		ExecutionID: execution.ID, ReviewHeads: heads,
	}, nil)
	h.decisions.EXPECT().
		SubmitReview(gomock.Any(), h.workspaceID, execution.ID, service.ReviewSubmission{
			Verdict: entity.VerdictRequestChanges,
			Summary: "Rename the flag before merging",
			Heads:   heads,
		}).
		Return(entity.ExecutionReview{}, nil)
	h.settles(id, entity.TelegramUpdateApplied)

	if err := h.updatesService().Apply(context.Background(), id); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if len(h.sent) != 1 || h.sent[0].Text != "Changes requested." {
		t.Errorf("sent = %+v", h.sent)
	}
}

func (h *harness) publicationMessage(execution entity.Execution) entity.TelegramDecisionMessage {
	return entity.TelegramDecisionMessage{
		BotID: h.bot.ID, ChatID: groupChat, MessageID: 88, Kind: entity.TelegramDecisionPublication,
		ExecutionID: execution.ID, Round: "2.1",
	}
}

func TestRetryingAStalledPublicationFromTelegramHandsItBackToTheMachine(t *testing.T) {
	h := newHarness(t)
	execution := h.running(entity.ExecutionApproved)

	id := h.tapping(entity.TelegramCallbackRetry, h.publicationMessage(execution))
	h.audience.EXPECT().AccountOf(gomock.Any(), h.bot.ID, int64(senderID)).Return(h.linked(uuid.New(), senderID), nil)
	h.decisions.EXPECT().RetryPublication(gomock.Any(), h.workspaceID, execution.ID).Return(execution, nil)

	h.noticed(t, id, "Retrying publication.")
}

func TestGivingUpOnAPublicationThatAlreadyFinishedDecidesNothing(t *testing.T) {
	h := newHarness(t)
	execution := h.running(entity.ExecutionApproved)

	id := h.tapping(entity.TelegramCallbackAbandon, h.publicationMessage(execution))
	h.audience.EXPECT().AccountOf(gomock.Any(), h.bot.ID, int64(senderID)).Return(h.linked(uuid.New(), senderID), nil)
	h.decisions.EXPECT().
		AbandonPublication(gomock.Any(), h.workspaceID, execution.ID).
		Return(entity.Execution{}, entity.ErrPublicationNotPending)

	h.noticed(t, id, "This was already decided.")
}
