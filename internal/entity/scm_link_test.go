package entity_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestOnlyAChangeReachingAStateTheTeamRoutedDrivesTheIssue(t *testing.T) {
	inReview, done := uuid.New(), uuid.New()

	rules := entity.SCMTransitionRules{
		{Trigger: entity.CodeChangeReviewRequested, StateID: inReview},
		{Trigger: entity.CodeChangeMerged, StateID: done},
	}

	cases := []struct {
		name string
		link entity.CodeLink
		want uuid.UUID
	}{
		{
			name: "a merged change on a team that routed merges",
			link: entity.CodeLink{
				Kind:      entity.CodeLinkChange,
				State:     entity.CodeChangeMerged,
				Resolving: true,
			},
			want: done,
		},
		{
			name: "the same change entering review",
			link: entity.CodeLink{
				Kind:      entity.CodeLinkChange,
				State:     entity.CodeChangeReviewRequested,
				Resolving: true,
			},
			want: inReview,
		},
		{
			name: "a state the team routed nothing for",
			link: entity.CodeLink{
				Kind:      entity.CodeLinkChange,
				State:     entity.CodeChangeClosed,
				Resolving: true,
			},
			want: uuid.Nil,
		},
		{
			name: "a branch, which nobody merges",
			link: entity.CodeLink{
				Kind:      entity.CodeLinkBranch,
				State:     entity.CodeChangeMerged,
				Resolving: true,
			},
			want: uuid.Nil,
		},
		{
			name: "a commit landing on the default branch",
			link: entity.CodeLink{
				Kind:      entity.CodeLinkCommit,
				State:     entity.CodeChangeMerged,
				Resolving: true,
			},
			want: uuid.Nil,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			rule, ok := rules.For(testCase.link)

			if testCase.want == uuid.Nil {
				if ok {
					t.Fatalf("a rule fired for %s, and none should have", testCase.link.State)
				}

				return
			}

			if !ok {
				t.Fatalf("no rule fired for %s", testCase.link.State)
			}

			if rule.StateID != testCase.want {
				t.Fatalf("rule sent the issue to %s, want %s", rule.StateID, testCase.want)
			}
		})
	}
}

func TestARuleSendsTheIssueOnlyToTheStateItNames(t *testing.T) {
	shipped := entity.WorkflowState{ID: uuid.New(), Name: "Shipped"}
	done := entity.WorkflowState{ID: uuid.New(), Name: "Done", IsCompletion: true}
	states := []entity.WorkflowState{shipped, done}

	rule := entity.SCMTransitionRule{Trigger: entity.CodeChangeMerged, StateID: shipped.ID}

	if state, ok := rule.TargetState(states); !ok || state.ID != shipped.ID {
		t.Fatalf("TargetState = %v, %v; want the named state", state.Name, ok)
	}

	removed := entity.SCMTransitionRule{Trigger: entity.CodeChangeMerged, StateID: uuid.New()}

	if state, ok := removed.TargetState(states); ok {
		t.Fatalf(
			"TargetState fell back to %q after the named state was gone; a team that chose where "+
				"merged work goes must not silently have it sent somewhere else",
			state.Name,
		)
	}
}

func TestEveryStateAChangeCanReachIsAValidTrigger(t *testing.T) {
	for _, state := range entity.CodeChangeStates() {
		if !state.Valid() {
			t.Errorf("%s is listed as a state but does not validate", state)
		}
	}

	if entity.CodeChangeState("in_review").Valid() {
		t.Error("in_review was replaced by the three review states and must no longer validate")
	}
}

func TestALinkOutlivesTheRepositoryThatFoundIt(t *testing.T) {
	link := entity.CodeLink{
		Provider:       entity.SCMProviderGitHub,
		RepositoryName: "acme/api",
		Kind:           entity.CodeLinkChange,
		Number:         41,
		URL:            "https://github.com/acme/api/pull/41",
		Title:          "Drop the cache on write",
		State:          entity.CodeChangeMerged,
	}

	if !link.Disconnected() {
		t.Fatal("a link with no repository must report itself disconnected")
	}

	if link.URL == "" || link.Title == "" || link.RepositoryName == "" {
		t.Fatal(
			"a disconnected link still has to render on the issue, so what it needs to be " +
				"readable cannot live on the repository row",
		)
	}
}

func TestChangesRequestedOutranksAnApprovalHoweverRecent(t *testing.T) {
	cases := []struct {
		name      string
		reviewers entity.CodeReviewers
		want      entity.CodeChangeState
	}{
		{
			name:      "nobody has been asked",
			reviewers: entity.CodeReviewers{},
			want:      "",
		},
		{
			name:      "asked, nobody has answered",
			reviewers: entity.CodeReviewers{{Verdict: entity.ReviewRequested}},
			want:      entity.CodeChangeReviewRequested,
		},
		{
			name:      "one approval",
			reviewers: entity.CodeReviewers{{Verdict: entity.ReviewApproved}},
			want:      entity.CodeChangeApproved,
		},
		{
			name: "two approvals and one asking for changes",
			reviewers: entity.CodeReviewers{
				{Verdict: entity.ReviewApproved},
				{Verdict: entity.ReviewApproved},
				{Verdict: entity.ReviewChangesRequested},
			},
			want: entity.CodeChangeChangesRequested,
		},
		{
			name: "a comment is not an answer",
			reviewers: entity.CodeReviewers{
				{Verdict: entity.ReviewCommented},
				{Verdict: entity.ReviewDismissed},
			},
			want: "",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, found := testCase.reviewers.Outcome()

			if testCase.want == "" {
				if found {
					t.Fatalf("Outcome = %s, want no opinion at all", got)
				}

				return
			}

			if !found || got != testCase.want {
				t.Fatalf("Outcome = %s (%t), want %s", got, found, testCase.want)
			}
		})
	}
}

func TestAChangeThatOnlyMentionsAnIssueNeverMovesIt(t *testing.T) {
	rules := entity.SCMTransitionRules{
		{Trigger: entity.CodeChangeMerged, StateID: uuid.New()},
	}

	mentioning := entity.CodeLink{
		Kind:      entity.CodeLinkChange,
		State:     entity.CodeChangeMerged,
		Resolving: false,
	}

	if _, fired := rules.For(mentioning); fired {
		t.Fatal(
			"a change that names an issue for context moved it. Only fixes, closes or resolves " +
				"claims to settle an issue, and a mention must link and nothing more",
		)
	}

	resolving := mentioning
	resolving.Resolving = true

	if _, fired := rules.For(resolving); !fired {
		t.Fatal("a change that says it fixes the issue must drive the rule")
	}
}

func TestWhatTheChangeIsOutranksWhatItsReviewersThink(t *testing.T) {
	approved := entity.CodeReviewers{{Verdict: entity.ReviewApproved}}
	blocking := entity.CodeReviewers{{Verdict: entity.ReviewChangesRequested}}

	cases := []struct {
		name      string
		base      entity.CodeChangeState
		reviewers entity.CodeReviewers
		want      entity.CodeChangeState
	}{
		{"a merged change is finished however it was reviewed", entity.CodeChangeMerged, blocking, entity.CodeChangeMerged},
		{"a closed change is finished too", entity.CodeChangeClosed, approved, entity.CodeChangeClosed},
		{"a draft is not up for review yet", entity.CodeChangeDraft, approved, entity.CodeChangeDraft},
		{"a change that will not merge is blocked on something no reviewer can fix", entity.CodeChangeConflicted, approved, entity.CodeChangeConflicted},
		{"an open change takes its reviewers' answer", entity.CodeChangeOpen, approved, entity.CodeChangeApproved},
		{"and their refusal", entity.CodeChangeOpen, blocking, entity.CodeChangeChangesRequested},
		{"an open change nobody reviewed stays open", entity.CodeChangeOpen, entity.CodeReviewers{}, entity.CodeChangeOpen},
		{"a reopened change nobody reviewed stays reopened", entity.CodeChangeReopened, entity.CodeReviewers{}, entity.CodeChangeReopened},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := entity.ResolveChangeState(testCase.base, testCase.reviewers)

			if got != testCase.want {
				t.Fatalf("ResolveChangeState(%s) = %s, want %s", testCase.base, got, testCase.want)
			}
		})
	}
}
