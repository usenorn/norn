package telegrambot_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	messengerrepo "github.com/usenorn/norn/internal/repository/telegrammessenger"
)

func (h *harness) relaying(asked entity.IssueQuestion) {
	h.questions.EXPECT().GetByID(gomock.Any(), h.workspaceID, asked.ID).Return(asked, nil)
	h.bots.EXPECT().Get(gomock.Any(), h.workspaceID, h.agent.ID).Return(h.bot, nil)
	h.issued(asked.IssueID)
}

func (h *harness) issued(issueID uuid.UUID) {
	h.issues.EXPECT().
		GetVisible(gomock.Any(), h.workspaceID, issueID, gomock.Any()).
		Return(entity.Issue{ID: issueID, ReferenceKey: "NORN", Number: 7, Title: "Ship <the> release"}, nil).
		AnyTimes()
}

func (h *harness) decidedBy(issueID, maker uuid.UUID, channel entity.DecisionChannel) {
	h.delegations.EXPECT().
		Authority(gomock.Any(), h.workspaceID, issueID).
		Return(entity.DecisionAuthority{AssigneeAccountID: maker, AssigneeKind: entity.AccountKindPerson, AssigneeName: "Rae"}, nil).
		AnyTimes()
	h.settings.EXPECT().DecisionChannel(gomock.Any(), h.workspaceID, maker).Return(channel, nil).AnyTimes()
}

func (h *harness) running(state entity.ExecutionState) entity.Execution {
	execution := entity.Execution{
		ID:          "exec-01TG",
		WorkspaceID: h.workspaceID,
		IssueID:     uuid.New(),
		AgentID:     h.agent.ID,
		State:       state,
	}

	h.executions.EXPECT().GetByID(gomock.Any(), execution.ID).Return(execution, nil).AnyTimes()
	h.bots.EXPECT().Get(gomock.Any(), h.workspaceID, h.agent.ID).Return(h.bot, nil).AnyTimes()
	h.issued(execution.IssueID)

	return execution
}

func (h *harness) remembering(t *testing.T) *[]entity.TelegramDecisionMessage {
	t.Helper()

	var remembered []entity.TelegramDecisionMessage

	h.conversation.EXPECT().
		Remember(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, message entity.TelegramDecisionMessage) error {
			remembered = append(remembered, message)

			return nil
		}).
		AnyTimes()

	return &remembered
}

func chats(messages []entity.TelegramDecisionMessage) []int64 {
	sent := make([]int64, 0, len(messages))

	for _, message := range messages {
		sent = append(sent, message.ChatID)
	}

	return sent
}

func TestAQuestionGoesToTheAssigneeWhoPrefersTelegramAndEveryBoundGroupOnce(t *testing.T) {
	h := newHarness(t)
	asked := h.question("Friday", "Monday")
	assignee := uuid.New()
	h.relaying(asked)
	h.decidedBy(asked.IssueID, assignee, entity.DecisionChannelTelegram)

	h.audience.EXPECT().AccountFor(gomock.Any(), h.bot.ID, assignee).Return(h.linked(assignee, 900), nil)
	h.audience.EXPECT().Groups(gomock.Any(), h.bot.ID).Return([]entity.TelegramGroup{{ChatID: groupChat}, {ChatID: -55}}, nil)
	h.conversation.EXPECT().
		Posted(gomock.Any(), entity.TelegramQuestionDecision(asked)).
		Return([]entity.TelegramDecisionMessage{{
			BotID: h.bot.ID, ChatID: -55, MessageID: 3, Kind: entity.TelegramDecisionQuestion, QuestionID: asked.ID,
		}}, nil)

	remembered := h.remembering(t)

	if err := h.updatesService().Relay(context.Background(), entity.TelegramQuestionDecision(asked)); err != nil {
		t.Fatalf("Relay: %v", err)
	}

	if got := chats(*remembered); !slices.Equal(got, []int64{900, groupChat}) {
		t.Errorf("sent to %v, want the assignee then the unposted group", got)
	}

	for _, message := range *remembered {
		if message.Kind != entity.TelegramDecisionQuestion || message.QuestionID != asked.ID || message.MessageID == 0 {
			t.Errorf("remembered %+v", message)
		}
	}

	message := h.sent[0]
	if !slices.Equal(message.Buttons, entity.TelegramOptionButtons(asked.Options)) {
		t.Errorf("buttons = %+v", message.Buttons)
	}

	for _, want := range []string{
		`<a href="https://norn.example/acme/issues/NORN-7">NORN-7</a>`,
		"Ship &lt;the&gt; release",
		"Ship it on Friday?",
		"30 Sep 2026, 12:00 UTC",
		"<i>Monday</i>",
	} {
		if !strings.Contains(message.Text, want) {
			t.Errorf("message lacks %q:\n%s", want, message.Text)
		}
	}
}

func TestAnAssigneeWhoPrefersNornGetsNoTelegramMessage(t *testing.T) {
	h := newHarness(t)
	asked := h.question()
	assignee := uuid.New()
	h.relaying(asked)
	h.decidedBy(asked.IssueID, assignee, entity.DecisionChannelNorn)

	h.audience.EXPECT().Groups(gomock.Any(), h.bot.ID).Return(nil, nil)
	h.conversation.EXPECT().Posted(gomock.Any(), gomock.Any()).Return(nil, nil)

	if err := h.updatesService().Relay(context.Background(), entity.TelegramQuestionDecision(asked)); err != nil {
		t.Fatalf("Relay: %v", err)
	}

	if len(h.sent) != 0 {
		t.Fatalf("sent %+v to somebody who asked to decide in Norn", h.sent)
	}
}

func TestAnAgentWithoutABotRelaysNothing(t *testing.T) {
	h := newHarness(t)
	asked := h.question()
	h.questions.EXPECT().GetByID(gomock.Any(), h.workspaceID, asked.ID).Return(asked, nil)
	h.bots.EXPECT().Get(gomock.Any(), h.workspaceID, h.agent.ID).Return(entity.TelegramBot{}, entity.ErrTelegramBotNotFound)

	if err := h.updatesService().Relay(context.Background(), entity.TelegramQuestionDecision(asked)); err != nil {
		t.Fatalf("Relay: %v", err)
	}
}

func TestAQuestionAskedByAPersonRelaysNothing(t *testing.T) {
	h := newHarness(t)
	asked := h.question()
	asked.AskedByAccountID = uuid.New()
	h.questions.EXPECT().GetByID(gomock.Any(), h.workspaceID, asked.ID).Return(asked, nil)
	h.agents.EXPECT().GetByAccountID(gomock.Any(), asked.AskedByAccountID).Return(entity.Agent{}, entity.ErrAgentNotFound)

	if err := h.updatesService().Relay(context.Background(), entity.TelegramQuestionDecision(asked)); err != nil {
		t.Fatalf("Relay: %v", err)
	}
}

func TestASettledQuestionIsEditedToItsOutcomeEverywhereItWasPosted(t *testing.T) {
	h := newHarness(t)
	asked := h.question("Friday", "Monday")
	answeredAt := time.Now()
	asked.State = entity.QuestionAnswered
	asked.Answer = "Friday"
	asked.AnsweredByName = "Rae"
	asked.AnsweredAt = &answeredAt
	h.relaying(asked)

	posted := []entity.TelegramDecisionMessage{
		{BotID: h.bot.ID, ChatID: 900, MessageID: 1, Kind: entity.TelegramDecisionQuestion, QuestionID: asked.ID},
		{BotID: h.bot.ID, ChatID: groupChat, MessageID: 2, Kind: entity.TelegramDecisionQuestion, QuestionID: asked.ID, Settled: true},
	}
	h.conversation.EXPECT().Posted(gomock.Any(), entity.TelegramQuestionDecision(asked)).Return(posted, nil)
	h.messenger.EXPECT().
		Edit(gomock.Any(), botToken, int64(900), int64(1), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, _, _ int64, text string) error {
			if !strings.Contains(text, "Answered by Rae: <i>Friday</i>") {
				t.Errorf("edited to:\n%s", text)
			}

			return nil
		})
	h.conversation.EXPECT().MarkSettled(gomock.Any(), posted[0], gomock.Any()).Return(nil)

	if err := h.updatesService().Settle(context.Background(), entity.TelegramQuestionDecision(asked)); err != nil {
		t.Fatalf("Settle: %v", err)
	}
}

func TestABlockedChatDoesNotStopTheOthers(t *testing.T) {
	h := newHarness(t)
	asked := h.question()
	h.relaying(asked)
	h.delegations.EXPECT().Authority(gomock.Any(), h.workspaceID, asked.IssueID).Return(entity.DecisionAuthority{}, nil)

	h.audience.EXPECT().Groups(gomock.Any(), h.bot.ID).Return([]entity.TelegramGroup{{ChatID: -1}, {ChatID: -2}}, nil)
	h.conversation.EXPECT().Posted(gomock.Any(), gomock.Any()).Return(nil, nil)

	h.messenger = messengerrepo.NewMockTelegramMessenger(gomock.NewController(t))
	h.messenger.EXPECT().Send(gomock.Any(), botToken, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, message entity.TelegramOutgoing) (int64, error) {
			if message.ChatID == -1 {
				return 0, entity.ErrTelegramChatUnavailable
			}

			return 5, nil
		}).
		Times(2)
	h.conversation.EXPECT().
		Remember(gomock.Any(), entity.TelegramDecisionMessage{
			BotID: h.bot.ID, ChatID: -2, MessageID: 5, Kind: entity.TelegramDecisionQuestion, QuestionID: asked.ID,
		}).
		Return(nil)

	if err := h.updatesService().Relay(context.Background(), entity.TelegramQuestionDecision(asked)); err != nil {
		t.Fatalf("Relay: %v", err)
	}
}

func TestAPlanIsSentWithApprovalButtonsAgainForEachNewRevision(t *testing.T) {
	h := newHarness(t)
	execution := h.running(entity.ExecutionAwaitingPlan)
	h.decidedBy(execution.IssueID, uuid.New(), entity.DecisionChannelNorn)
	h.audience.EXPECT().Groups(gomock.Any(), h.bot.ID).Return([]entity.TelegramGroup{{ChatID: groupChat}}, nil)

	h.plans.EXPECT().ListByExecution(gomock.Any(), execution.ID).Return([]entity.ExecutionPlan{
		{Revision: 1, Body: "First idea.", RevisionRequestedAt: new(time.Now())},
		{Revision: 2, Body: "1. Add the migration.\n2. Backfill."},
	}, nil)

	decision := entity.TelegramPlanDecision(execution, 2)
	h.conversation.EXPECT().Posted(gomock.Any(), decision).Return([]entity.TelegramDecisionMessage{{
		BotID: h.bot.ID, ChatID: groupChat, MessageID: 9, Kind: entity.TelegramDecisionPlan,
		ExecutionID: execution.ID, PlanRevision: 1, Settled: true,
	}}, nil)

	remembered := h.remembering(t)

	if err := h.updatesService().Relay(context.Background(), decision); err != nil {
		t.Fatalf("Relay: %v", err)
	}

	if len(*remembered) != 1 || (*remembered)[0].PlanRevision != 2 || (*remembered)[0].ExecutionID != execution.ID {
		t.Fatalf("remembered %+v; want revision 2 posted to the group that only saw revision 1", *remembered)
	}

	message := h.sent[0]
	if data := []string{message.Buttons[0].Data, message.Buttons[1].Data}; !slices.Equal(
		data, []string{entity.TelegramCallbackApprove, entity.TelegramCallbackChanges},
	) {
		t.Errorf("buttons = %+v, want approve then ask for changes", message.Buttons)
	}

	for _, want := range []string{
		"2. Backfill.",
		`<a href="https://norn.example/acme/executions/exec-01TG#plan">`,
		"reply to this message with what should change",
	} {
		if !strings.Contains(message.Text, want) {
			t.Errorf("plan message lacks %q:\n%s", want, message.Text)
		}
	}
}

func TestAPlanThatIsNoLongerWaitingIsNotSent(t *testing.T) {
	h := newHarness(t)
	execution := h.running(entity.ExecutionRunning)

	if err := h.updatesService().Relay(context.Background(), entity.TelegramPlanDecision(execution, 1)); err != nil {
		t.Fatalf("Relay: %v", err)
	}

	if len(h.sent) != 0 {
		t.Fatalf("sent %+v for a run that is not waiting on its plan", h.sent)
	}
}

func TestAReviewMessageRemembersTheHeadsItAsksAbout(t *testing.T) {
	h := newHarness(t)
	execution := h.running(entity.ExecutionAwaitingReview)
	h.decidedBy(execution.IssueID, uuid.New(), entity.DecisionChannelNorn)
	h.audience.EXPECT().Groups(gomock.Any(), h.bot.ID).Return([]entity.TelegramGroup{{ChatID: groupChat}}, nil)

	h.snapshots.EXPECT().Latest(gomock.Any(), execution.ID).Return(entity.ExecutionSnapshot{
		ExecutionID:  execution.ID,
		Revision:     2,
		Summary:      "Split the handler.",
		Repositories: []entity.SnapshotRepository{{Repository: "api", HeadSHA: "abc123"}},
	}, nil)
	h.conversation.EXPECT().Posted(gomock.Any(), gomock.Any()).Return(nil, nil)

	remembered := h.remembering(t)
	heads := entity.ReviewHeads{"api": "abc123"}

	if err := h.updatesService().Relay(context.Background(), entity.TelegramReviewDecision(execution, heads)); err != nil {
		t.Fatalf("Relay: %v", err)
	}

	if len(*remembered) != 1 || !(*remembered)[0].ReviewHeads.Matches(heads) {
		t.Fatalf("remembered %+v, want the heads the reviewer is shown", *remembered)
	}

	for _, want := range []string{"Split the handler.", `<a href="https://norn.example/acme/executions/exec-01TG/review">`} {
		if !strings.Contains(h.sent[0].Text, want) {
			t.Errorf("review message lacks %q:\n%s", want, h.sent[0].Text)
		}
	}
}

func TestADecidedPlanIsEditedAndTheOpenRevisionIsLeftAlone(t *testing.T) {
	h := newHarness(t)
	execution := h.running(entity.ExecutionAwaitingPlan)
	approvedAt := time.Now()

	h.plans.EXPECT().ListByExecution(gomock.Any(), execution.ID).Return([]entity.ExecutionPlan{
		{Revision: 1, Body: "First idea.", RevisionRequestedAt: &approvedAt, RevisionRequestedByName: "Rae", RevisionFeedback: "Smaller."},
		{Revision: 2, Body: "Second idea."},
	}, nil)

	posted := []entity.TelegramDecisionMessage{
		{BotID: h.bot.ID, ChatID: groupChat, MessageID: 1, Kind: entity.TelegramDecisionPlan, ExecutionID: execution.ID, PlanRevision: 1},
		{BotID: h.bot.ID, ChatID: groupChat, MessageID: 2, Kind: entity.TelegramDecisionPlan, ExecutionID: execution.ID, PlanRevision: 2},
	}
	decision := entity.TelegramDecision{WorkspaceID: h.workspaceID, Kind: entity.TelegramDecisionPlan, ExecutionID: execution.ID}
	h.conversation.EXPECT().Posted(gomock.Any(), decision).Return(posted, nil)
	h.messenger.EXPECT().
		Edit(gomock.Any(), botToken, int64(groupChat), int64(1), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, _, _ int64, text string) error {
			if !strings.Contains(text, "Rae asked for changes: <i>Smaller.</i>") {
				t.Errorf("edited to:\n%s", text)
			}

			return nil
		})
	h.conversation.EXPECT().MarkSettled(gomock.Any(), posted[0], gomock.Any()).Return(nil)

	if err := h.updatesService().Settle(context.Background(), decision); err != nil {
		t.Fatalf("Settle: %v", err)
	}
}

func TestAnApprovedReviewIsEditedToSayWhoApproved(t *testing.T) {
	h := newHarness(t)
	execution := h.running(entity.ExecutionApproved)
	heads := entity.ReviewHeads{"api": "abc123"}

	h.reviews.EXPECT().ListReviews(gomock.Any(), execution.ID).Return([]entity.ExecutionReview{
		{Verdict: entity.VerdictComment, Heads: heads, AuthorName: "Sam"},
		{Verdict: entity.VerdictApprove, Heads: heads, AuthorName: "Rae"},
	}, nil)
	h.snapshots.EXPECT().Latest(gomock.Any(), execution.ID).Return(entity.ExecutionSnapshot{}, nil)

	posted := []entity.TelegramDecisionMessage{{
		BotID: h.bot.ID, ChatID: 900, MessageID: 4, Kind: entity.TelegramDecisionReview, ExecutionID: execution.ID, ReviewHeads: heads,
	}}
	decision := entity.TelegramDecision{WorkspaceID: h.workspaceID, Kind: entity.TelegramDecisionReview, ExecutionID: execution.ID}
	h.conversation.EXPECT().Posted(gomock.Any(), decision).Return(posted, nil)
	h.messenger.EXPECT().
		Edit(gomock.Any(), botToken, int64(900), int64(4), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, _, _ int64, text string) error {
			if !strings.Contains(text, "Changes approved by Rae.") {
				t.Errorf("edited to:\n%s", text)
			}

			return nil
		})
	h.conversation.EXPECT().MarkSettled(gomock.Any(), posted[0], gomock.Any()).Return(nil)

	if err := h.updatesService().Settle(context.Background(), decision); err != nil {
		t.Fatalf("Settle: %v", err)
	}
}

func TestAStalledPublicationAsksWhetherToRetryAndSaysWhatFailed(t *testing.T) {
	h := newHarness(t)
	execution := h.running(entity.ExecutionApproved)
	h.decidedBy(execution.IssueID, uuid.New(), entity.DecisionChannelNorn)
	h.audience.EXPECT().Groups(gomock.Any(), h.bot.ID).Return([]entity.TelegramGroup{{ChatID: groupChat}}, nil)

	h.changesets.EXPECT().Get(gomock.Any(), execution.ID).Return(entity.ExecutionChangeSet{
		ExecutionID: execution.ID,
		Changes: []entity.ExecutionChange{
			{
				Repository: "api", PullRequestURL: "https://github.com/acme/api/pull/7",
				Publication: entity.ExecutionPublication{State: entity.PublicationPublished},
			},
			{
				Repository: "web",
				Publication: entity.ExecutionPublication{
					State: entity.PublicationFailed, Step: entity.PublicationStepPush, Error: "protected branch",
				},
			},
		},
	}, nil)
	h.conversation.EXPECT().Posted(gomock.Any(), gomock.Any()).Return(nil, nil)

	remembered := h.remembering(t)
	decision := entity.TelegramPublicationDecision(execution, 2, 1)

	if err := h.updatesService().Relay(context.Background(), decision); err != nil {
		t.Fatalf("Relay: %v", err)
	}

	if len(*remembered) != 1 || (*remembered)[0].Round != decision.Round {
		t.Fatalf("remembered %+v, want the attempt that stalled", *remembered)
	}

	for _, want := range []string{
		`<b>api</b>: <a href="https://github.com/acme/api/pull/7">pull request</a>`,
		"<b>web</b>: pushing failed: <i>protected branch</i>",
	} {
		if !strings.Contains(h.sent[0].Text, want) {
			t.Errorf("publication message lacks %q:\n%s", want, h.sent[0].Text)
		}
	}

	if len(h.sent[0].Buttons) != 2 || h.sent[0].Buttons[0].Data != entity.TelegramCallbackRetry ||
		h.sent[0].Buttons[1].Data != entity.TelegramCallbackAbandon {
		t.Fatalf("publication message offers %+v, want retry and give up", h.sent[0].Buttons)
	}
}
