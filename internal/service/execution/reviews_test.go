package execution_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func (h *harness) underReview() entity.Execution {
	execution := h.execution(entity.ExecutionAwaitingReview)
	h.holding(execution)
	h.moving()
	h.states.EXPECT().ListByTeamID(gomock.Any(), gomock.Any()).Return(h.states34(), nil).AnyTimes()
	h.changed("api", "head-1")

	return execution
}

func (h *harness) drafted(t *testing.T, execution entity.Execution, body string) {
	t.Helper()

	if _, err := h.service.CommentOnReview(context.Background(), h.workspaceID, execution.ID,
		service.ReviewCommentDraft{
			Anchor: entity.ReviewAnchor{
				Repository: "api", Path: "internal/run.go", Side: entity.ReviewSideNew, Line: 12,
				Hunk: "+\treturn nil",
			},
			Body: body,
		},
	); err != nil {
		t.Fatalf("draft a comment: %v", err)
	}
}

func TestOnlyChangesWaitingForReviewCanBeReviewed(t *testing.T) {
	for _, state := range []entity.ExecutionState{
		entity.ExecutionRunning, entity.ExecutionAwaitingPlan, entity.ExecutionCompleted,
	} {
		t.Run(string(state), func(t *testing.T) {
			h := newHarness(t)
			execution := h.execution(state)
			h.holding(execution)

			_, err := h.service.SubmitReview(context.Background(), h.workspaceID, execution.ID,
				service.ReviewSubmission{Verdict: entity.VerdictApprove, Heads: entity.ReviewHeads{}},
			)
			if !errors.Is(err, entity.ErrReviewClosed) {
				t.Fatalf("reviewing a %s run returned %v", state, err)
			}
		})
	}
}

func TestAnAgentCannotApproveItsOwnWork(t *testing.T) {
	h := newHarness(t)
	execution := h.underReview()

	itself := execution.AgentID
	h.callerAgent = &itself

	_, err := h.service.SubmitReview(context.Background(), h.workspaceID, execution.ID,
		service.ReviewSubmission{Verdict: entity.VerdictApprove, Heads: entity.ReviewHeads{"api": "head-1"}},
	)
	if !errors.Is(err, entity.ErrExecutionSelfApproval) {
		t.Fatalf(
			"an agent approved its own changes and got %v; approving is what publishes them, so "+
				"leaving it open to a token with issue:manage makes review optional",
			err,
		)
	}
}

func TestAReviewOfChangesThatMovedOnIsRefused(t *testing.T) {
	h := newHarness(t)
	execution := h.underReview()

	_, err := h.service.SubmitReview(context.Background(), h.workspaceID, execution.ID,
		service.ReviewSubmission{Verdict: entity.VerdictApprove, Heads: entity.ReviewHeads{"api": "head-0"}},
	)
	if !errors.Is(err, entity.ErrReviewStale) {
		t.Fatalf("a review of an older head was accepted and got %v; it approves code nobody read", err)
	}

	if _, published := h.sent(entity.ChannelExecutionResume); published {
		t.Fatal("a stale approval let the machine publish")
	}
}

func TestApprovingTheReviewIsWhatLetsTheMachinePublish(t *testing.T) {
	h := newHarness(t)
	execution := h.underReview()

	if _, err := h.service.SubmitReview(context.Background(), h.workspaceID, execution.ID,
		service.ReviewSubmission{Verdict: entity.VerdictApprove, Heads: entity.ReviewHeads{"api": "head-1"}},
	); err != nil {
		t.Fatalf("approve: %v", err)
	}

	moved, ok := h.moved(entity.ExecutionApproved)
	if !ok {
		t.Fatal("an approved review left the run waiting")
	}

	if moved.Actor.AccountID != h.caller {
		t.Fatal("the timeline does not say who approved the changes")
	}

	if instruction := h.instruction(t); instruction.Reason != channelv1.ResumeApproved ||
		instruction.Stage != entity.StagePublication {
		t.Fatalf("the machine was told %+v, want approval to publish", instruction)
	}
}

func TestRequestingChangesHandsEveryCommentToTheAgentOnItsLine(t *testing.T) {
	h := newHarness(t)
	execution := h.underReview()
	h.drafted(t, execution, "This swallows the error.")

	if _, err := h.service.SubmitReview(context.Background(), h.workspaceID, execution.ID,
		service.ReviewSubmission{
			Verdict: entity.VerdictRequestChanges,
			Summary: "Close, but the failures vanish.",
			Heads:   entity.ReviewHeads{"api": "head-1"},
		},
	); err != nil {
		t.Fatalf("request changes: %v", err)
	}

	instruction := h.instruction(t)

	if instruction.Reason != channelv1.ResumeFeedback || instruction.Stage != entity.StageImplementation {
		t.Fatalf("the machine was told %s in %s, want review feedback in implementation",
			instruction.Reason, instruction.Stage)
	}

	for _, want := range []string{
		"Close, but the failures vanish.", "api:internal/run.go line 12", "This swallows the error.",
	} {
		if !strings.Contains(instruction.Instruction, want) {
			t.Errorf("the feedback is missing %q:\n%s", want, instruction.Instruction)
		}
	}

	if h.comments[0].Pending() {
		t.Fatal("the reviewer's draft stayed a draft after they submitted the review")
	}
}

func TestAskingForChangesWithNothingToSayIsRefused(t *testing.T) {
	h := newHarness(t)
	execution := h.underReview()

	_, err := h.service.SubmitReview(context.Background(), h.workspaceID, execution.ID,
		service.ReviewSubmission{Verdict: entity.VerdictRequestChanges, Heads: entity.ReviewHeads{"api": "head-1"}},
	)
	if !errors.Is(err, entity.ErrReviewEmpty) {
		t.Fatalf("an empty request for changes got %v; the agent would be sent back with no idea why", err)
	}
}

func TestADraftCommentIsHiddenFromEveryoneButItsAuthor(t *testing.T) {
	h := newHarness(t)
	execution := h.underReview()
	h.drafted(t, execution, "Not sure yet.")

	author := h.caller

	state, err := h.service.Review(context.Background(), h.workspaceID, execution.ID, 0)
	if err != nil || len(state.Comments) != 1 {
		t.Fatalf("the author sees %d comments (%v), want their own draft", len(state.Comments), err)
	}

	h.caller = uuid.New()

	state, err = h.service.Review(context.Background(), h.workspaceID, execution.ID, 0)
	if err != nil {
		t.Fatalf("read the review: %v", err)
	}

	if len(state.Comments) != 0 {
		t.Fatalf("a colleague sees %d of %s's unsubmitted drafts", len(state.Comments), author)
	}
}

func TestACommentIsPinnedToTheHeadItWasLeftOn(t *testing.T) {
	h := newHarness(t)
	execution := h.underReview()
	h.drafted(t, execution, "Why here?")

	if h.comments[0].Anchor.HeadSHA != "head-1" {
		t.Fatalf("the comment is pinned to %q, want the head the reviewer was reading", h.comments[0].Anchor.HeadSHA)
	}

	_, err := h.service.CommentOnReview(context.Background(), h.workspaceID, execution.ID,
		service.ReviewCommentDraft{
			Anchor: entity.ReviewAnchor{Repository: "web", Path: "x.ts", Side: entity.ReviewSideNew, Line: 1},
			Body:   "This repository did not change.",
		},
	)
	if !errors.Is(err, entity.ErrReviewCommentAnchor) {
		t.Fatalf("a comment on a repository the run never touched got %v", err)
	}
}

func TestOnlyTheAssigneeOrAnAdminApprovesOrRequestsChanges(t *testing.T) {
	for _, verdict := range []entity.ExecutionReviewVerdict{entity.VerdictApprove, entity.VerdictRequestChanges} {
		h := newHarness(t)
		h.authority.AssigneeAccountID = uuid.New()
		execution := h.underReview()

		_, err := h.service.SubmitReview(context.Background(), h.workspaceID, execution.ID,
			service.ReviewSubmission{Verdict: verdict, Summary: "Looks off.", Heads: entity.ReviewHeads{"api": "head-1"}},
		)
		if !errors.Is(err, entity.ErrIssueDecisionForbidden) {
			t.Fatalf("a member who is not the assignee submitted %s and got %v", verdict, err)
		}

		if _, published := h.sent(entity.ChannelExecutionResume); published {
			t.Fatalf("a refused %s still moved the run", verdict)
		}
	}
}

func TestAnyoneWhoManagesTheIssueMayStillCommentOnAReview(t *testing.T) {
	h := newHarness(t)
	h.authority.AssigneeAccountID = uuid.New()
	execution := h.underReview()

	if _, err := h.service.SubmitReview(context.Background(), h.workspaceID, execution.ID,
		service.ReviewSubmission{Verdict: entity.VerdictComment, Summary: "Why two queries?", Heads: entity.ReviewHeads{"api": "head-1"}},
	); err != nil {
		t.Fatalf("comment on the review: %v", err)
	}
}

func TestAFinalReviewIsSettledOnTelegramWhateverItsVerdict(t *testing.T) {
	for _, verdict := range []entity.ExecutionReviewVerdict{entity.VerdictApprove, entity.VerdictRequestChanges} {
		h := newHarness(t)
		execution := h.underReview()

		if _, err := h.service.SubmitReview(context.Background(), h.workspaceID, execution.ID,
			service.ReviewSubmission{Verdict: verdict, Summary: "Rename the flag.", Heads: entity.ReviewHeads{"api": "head-1"}},
		); err != nil {
			t.Fatalf("submit %s: %v", verdict, err)
		}

		if len(h.settled) != 1 || h.settled[0].Kind != entity.TelegramDecisionReview {
			t.Fatalf("after %s queued settlements %+v, want the review once", verdict, h.settled)
		}
	}
}

func (h *harness) reply(t *testing.T, commentID, body string) entity.ChannelMessage {
	t.Helper()

	payload, err := json.Marshal(channelv1.ReviewReply{CommentID: commentID, Body: body, Occurred: time.Now().UTC()})
	if err != nil {
		t.Fatalf("encode the reply: %v", err)
	}

	return entity.ChannelMessage{
		ID:          uuid.NewString(),
		Type:        entity.ChannelReviewReplied,
		ExecutionID: "exec-01ABC",
		Payload:     payload,
		IssuedAt:    time.Now().UTC(),
	}
}

func (h *harness) askedForChanges(t *testing.T) (entity.Execution, entity.ExecutionReviewComment) {
	t.Helper()

	execution := h.underReview()
	h.drafted(t, execution, "This swallows the error.")

	if _, err := h.service.SubmitReview(context.Background(), h.workspaceID, execution.ID,
		service.ReviewSubmission{Verdict: entity.VerdictRequestChanges, Heads: entity.ReviewHeads{"api": "head-1"}},
	); err != nil {
		t.Fatalf("request changes: %v", err)
	}

	return execution, h.comments[0]
}

func TestACommentIsTiedToTheRevisionItWasLeftOnAndItsRepliesFollowIt(t *testing.T) {
	h := newHarness(t)
	execution := h.underReview()
	h.revision = 2
	h.drafted(t, execution, "Why here?")

	if h.comments[0].Revision != 2 {
		t.Fatalf("the comment was tied to revision %d, want the latest, 2", h.comments[0].Revision)
	}

	h.revision = 3

	if _, err := h.service.CommentOnReview(context.Background(), h.workspaceID, execution.ID,
		service.ReviewCommentDraft{ParentID: h.comments[0].ID, Body: "Still wondering."},
	); err != nil {
		t.Fatalf("reply: %v", err)
	}

	if h.comments[1].Revision != 2 {
		t.Fatalf("the reply moved to revision %d; a thread stays on the revision it started on", h.comments[1].Revision)
	}
}

func TestAnEarlierRevisionCanBeReadButOneThatNeverExistedCannot(t *testing.T) {
	h := newHarness(t)
	execution := h.underReview()
	h.revision = 2

	state, err := h.service.Review(context.Background(), h.workspaceID, execution.ID, 1)
	if err != nil {
		t.Fatalf("read revision 1: %v", err)
	}

	if state.Snapshot.Revision != 1 || state.Latest.Revision != 2 || len(state.Revisions) != 2 {
		t.Fatalf("read revision %d of %d with %d listed", state.Snapshot.Revision, state.Latest.Revision, len(state.Revisions))
	}

	if _, err := h.service.Review(context.Background(), h.workspaceID, execution.ID, 7); !errors.Is(
		err, entity.ErrExecutionSnapshotNotFound,
	) {
		t.Fatalf("revision 7 answered %v", err)
	}
}

func TestTheReviewRecordsTheRevisionItDecided(t *testing.T) {
	h := newHarness(t)
	execution := h.underReview()
	h.revision = 4

	if _, err := h.service.SubmitReview(context.Background(), h.workspaceID, execution.ID,
		service.ReviewSubmission{Verdict: entity.VerdictApprove, Heads: entity.ReviewHeads{"api": "head-1"}},
	); err != nil {
		t.Fatalf("approve: %v", err)
	}

	if h.submitted[0].Revision != 4 {
		t.Fatalf("the review recorded revision %d, want 4", h.submitted[0].Revision)
	}
}

func TestRequestingChangesNamesEachThreadTheAgentShouldAnswer(t *testing.T) {
	h := newHarness(t)
	_, thread := h.askedForChanges(t)

	instruction := h.instruction(t)

	if len(instruction.Threads) != 1 || instruction.Threads[0] != thread.ID.String() {
		t.Fatalf("the machine was handed threads %v, want %s", instruction.Threads, thread.ID)
	}
}

func TestTheAgentAnswersAThreadInItsOwnName(t *testing.T) {
	h := newHarness(t)
	_, thread := h.askedForChanges(t)
	before := len(h.published)

	if err := h.service.ReviewReplied(
		context.Background(), h.runner, h.reply(t, thread.ID.String(), "Returned the error instead."),
	); err != nil {
		t.Fatalf("answer the thread: %v", err)
	}

	answer := h.comments[len(h.comments)-1]

	if answer.ParentID != thread.ID || answer.AuthorAccountID != h.agentAccount {
		t.Fatalf("the answer hangs off %s by %s, want the thread by the agent", answer.ParentID, answer.AuthorAccountID)
	}

	if answer.Pending() || answer.ReviewID != thread.ReviewID {
		t.Fatal("the agent's answer is a draft nobody but the agent could see")
	}

	if answer.Anchor != thread.Anchor || answer.Revision != thread.Revision {
		t.Fatal("the agent's answer left the line and revision its thread is on")
	}

	if len(h.published) == before {
		t.Fatal("nobody watching the review was told the agent answered")
	}
}

func TestAnAnswerTheReviewNoLongerTakesIsDroppedWithoutClosingTheChannel(t *testing.T) {
	h := newHarness(t)
	_, thread := h.askedForChanges(t)

	if _, err := h.service.CommentOnReview(context.Background(), h.workspaceID, "exec-01ABC",
		service.ReviewCommentDraft{ParentID: thread.ID, Body: "Any news?", Publish: true},
	); err != nil {
		t.Fatalf("reply as the reviewer: %v", err)
	}

	count := len(h.comments)
	replyID := h.comments[count-1].ID.String()

	for name, commentID := range map[string]string{
		"unknown thread": uuid.NewString(),
		"not an id":      "thread-1",
		"a reply":        replyID,
	} {
		if err := h.service.ReviewReplied(context.Background(), h.runner, h.reply(t, commentID, "Done.")); err != nil {
			t.Errorf("%s: answered %v; an error here would close the machine's channel and redeliver forever", name, err)
		}
	}

	if len(h.comments) != count {
		t.Fatalf("%d answers the review never asked for were stored", len(h.comments)-count)
	}
}

func TestAFinishedRunsAgentCannotAnswerTheReview(t *testing.T) {
	h := newHarness(t)
	h.reviewing()

	execution := h.execution(entity.ExecutionCompleted)
	h.holding(execution)

	thread := entity.ExecutionReviewComment{ID: uuid.New(), ReviewID: uuid.New(), ExecutionID: execution.ID}
	h.comments = append(h.comments, thread)

	if err := h.service.ReviewReplied(context.Background(), h.runner, h.reply(t, thread.ID.String(), "Late.")); err != nil {
		t.Fatalf("a late answer answered %v", err)
	}

	if len(h.comments) != 1 {
		t.Fatal("an answer from a finished run was stored")
	}
}
