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
			link: entity.CodeLink{Kind: entity.CodeLinkChange, State: entity.CodeChangeMerged},
			want: done,
		},
		{
			name: "the same change entering review",
			link: entity.CodeLink{Kind: entity.CodeLinkChange, State: entity.CodeChangeReviewRequested},
			want: inReview,
		},
		{
			name: "a state the team routed nothing for",
			link: entity.CodeLink{Kind: entity.CodeLinkChange, State: entity.CodeChangeClosed},
			want: uuid.Nil,
		},
		{
			name: "a branch, which nobody merges",
			link: entity.CodeLink{Kind: entity.CodeLinkBranch, State: entity.CodeChangeMerged},
			want: uuid.Nil,
		},
		{
			name: "a commit landing on the default branch",
			link: entity.CodeLink{Kind: entity.CodeLinkCommit, State: entity.CodeChangeMerged},
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
