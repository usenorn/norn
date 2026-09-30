package entity_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestTheLatestPlanIsTheHighestRevision(t *testing.T) {
	plans := []entity.ExecutionPlan{{Revision: 2}, {Revision: 3}, {Revision: 1}}

	latest, ok := entity.LatestPlan(plans)
	if !ok || latest.Revision != 3 {
		t.Fatalf("latest is %d (%v), want revision 3", latest.Revision, ok)
	}

	if _, ok := entity.LatestPlan(nil); ok {
		t.Fatal("a run that proposed nothing reports a latest plan")
	}
}

func TestOnlyUnsettledBlockingQuestionsHoldAPlanBack(t *testing.T) {
	open := entity.IssueQuestion{Question: "open", Blocking: true, State: entity.QuestionAsked}
	questions := []entity.IssueQuestion{
		open,
		{Question: "answered", Blocking: true, State: entity.QuestionAnswered},
		{Question: "not blocking", Blocking: false, State: entity.QuestionAsked},
		{Question: "dismissed", Blocking: true, State: entity.QuestionDismissed},
	}

	got := entity.BlockingQuestionsOpen(questions)
	if len(got) != 1 || got[0].Question != open.Question {
		t.Fatalf("open blocking questions are %+v, want only %q", got, open.Question)
	}
}

func TestACommentIsOutdatedOnceItsRepositoryMovesOn(t *testing.T) {
	comment := entity.ExecutionReviewComment{
		Anchor: entity.ReviewAnchor{Repository: "api", HeadSHA: "aaa"},
	}

	if comment.Outdated(entity.ReviewHeads{"api": "aaa", "web": "bbb"}) {
		t.Fatal("a comment on the current head reads as outdated")
	}

	if !comment.Outdated(entity.ReviewHeads{"api": "ccc"}) {
		t.Fatal("a comment on a head the agent has since replaced still reads as current")
	}
}

func TestAPendingCommentIsOnlyVisibleToItsAuthor(t *testing.T) {
	author, other := uuid.New(), uuid.New()
	pending := entity.ExecutionReviewComment{AuthorAccountID: author}

	if !pending.VisibleTo(author) || pending.VisibleTo(other) {
		t.Fatal("a draft comment leaked before its review was submitted")
	}

	pending.ReviewID = uuid.New()
	if !pending.VisibleTo(other) {
		t.Fatal("a submitted comment is hidden from the rest of the workspace")
	}
}

func TestAnAnchorMustNameALineOnOneSideOfAFile(t *testing.T) {
	valid := entity.ReviewAnchor{Repository: "api", Path: "main.go", Side: entity.ReviewSideNew, Line: 4}
	if err := entity.ValidateReviewAnchor(valid); err != nil {
		t.Fatalf("a well-formed anchor was refused: %v", err)
	}

	for name, anchor := range map[string]entity.ReviewAnchor{
		"no path":   {Repository: "api", Side: entity.ReviewSideNew, Line: 4},
		"no side":   {Repository: "api", Path: "main.go", Line: 4},
		"line zero": {Repository: "api", Path: "main.go", Side: entity.ReviewSideOld},
	} {
		var invalid entity.ValidationError
		if err := entity.ValidateReviewAnchor(anchor); !errors.As(err, &invalid) {
			t.Errorf("%s: anchor accepted, want a validation error", name)
		}
	}
}

func TestRequestedChangesReachTheAgentAnchoredToTheirLines(t *testing.T) {
	thread := uuid.MustParse("5b0c7a3e-2d1f-4e7a-9c1b-0f6d2a8e4b11")

	feedback := entity.ComposeReviewFeedback("Tighten the error handling.", []entity.ExecutionReviewComment{{
		ID: thread,
		Anchor: entity.ReviewAnchor{
			Repository: "api", Path: "internal/run.go", Side: entity.ReviewSideNew, Line: 42,
			HeadSHA: "0123456789abcdef", Hunk: "+\treturn nil",
		},
		Body: "This swallows the error.",
	}})

	for _, want := range []string{
		"Tighten the error handling.",
		"api:internal/run.go line 42 (new side, at 0123456, thread " + thread.String() + ")",
		"+\treturn nil",
		"This swallows the error.",
		"reply_to_review",
	} {
		if !strings.Contains(feedback, want) {
			t.Errorf("the feedback does not carry %q:\n%s", want, feedback)
		}
	}
}

func TestAReplyIsAnsweredOnTheThreadItBelongsTo(t *testing.T) {
	root := uuid.New()
	reply := uuid.New()

	threads := entity.ReviewThreads([]entity.ExecutionReviewComment{
		{ID: root},
		{ID: reply, ParentID: root},
	})

	if len(threads) != 1 || threads[0] != root.String() {
		t.Fatalf("the threads read %v; a reply must point the agent at its thread, not itself", threads)
	}
}

func TestAQuotedHunkKeepsOnlyTheLinesClosestToTheComment(t *testing.T) {
	lines := make([]string, entity.ReviewHunkMaxLines+5)
	for index := range lines {
		lines[index] = "+line"
	}

	lines[len(lines)-1] = "+commented"

	trimmed := strings.Split(entity.TrimReviewHunk(strings.Join(lines, "\n")), "\n")
	if len(trimmed) != entity.ReviewHunkMaxLines || trimmed[len(trimmed)-1] != "+commented" {
		t.Fatalf("kept %d lines ending %q", len(trimmed), trimmed[len(trimmed)-1])
	}
}
