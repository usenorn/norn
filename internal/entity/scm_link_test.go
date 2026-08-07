package entity_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestOnlyAMergedChangeOnATeamThatAskedForItAdvancesAnIssue(t *testing.T) {
	merged := entity.CodeLink{Kind: entity.CodeLinkChange, State: entity.CodeChangeMerged}

	cases := []struct {
		name     string
		settings entity.SCMTeamSettings
		link     entity.CodeLink
		want     bool
	}{
		{
			name:     "a merged change on a team that asked",
			settings: entity.SCMTeamSettings{AdvanceOnMerge: true},
			link:     merged,
			want:     true,
		},
		{
			name:     "the same change on a team that did not ask",
			settings: entity.SCMTeamSettings{},
			link:     merged,
			want:     false,
		},
		{
			name:     "a change that closed without merging",
			settings: entity.SCMTeamSettings{AdvanceOnMerge: true},
			link:     entity.CodeLink{Kind: entity.CodeLinkChange, State: entity.CodeChangeClosed},
			want:     false,
		},
		{
			name:     "a branch, which nobody merges",
			settings: entity.SCMTeamSettings{AdvanceOnMerge: true},
			link:     entity.CodeLink{Kind: entity.CodeLinkBranch, State: entity.CodeChangeMerged},
			want:     false,
		},
		{
			name:     "a commit landing on the default branch",
			settings: entity.SCMTeamSettings{AdvanceOnMerge: true},
			link:     entity.CodeLink{Kind: entity.CodeLinkCommit, State: entity.CodeChangeMerged},
			want:     false,
		},
		{
			name:     "a merge already acted on, redelivered",
			settings: entity.SCMTeamSettings{AdvanceOnMerge: true},
			link: entity.CodeLink{
				Kind:          entity.CodeLinkChange,
				State:         entity.CodeChangeMerged,
				AdvancedIssue: true,
			},
			want: false,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := testCase.settings.Advances(testCase.link); got != testCase.want {
				t.Fatalf("Advances = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestTheTargetStateIsTheOneNamedOrTheTeamsOwnCompletion(t *testing.T) {
	shipped := entity.WorkflowState{ID: uuid.New(), Name: "Shipped"}
	done := entity.WorkflowState{ID: uuid.New(), Name: "Done", IsCompletion: true}
	states := []entity.WorkflowState{shipped, done}

	named := entity.SCMTeamSettings{MergedStateID: shipped.ID}

	if state, ok := named.TargetState(states); !ok || state.ID != shipped.ID {
		t.Fatalf("TargetState = %v, %v; want the named state", state.Name, ok)
	}

	unnamed := entity.SCMTeamSettings{}

	if state, ok := unnamed.TargetState(states); !ok || state.ID != done.ID {
		t.Fatalf("TargetState = %v, %v; want the team's completion state", state.Name, ok)
	}
}

func TestAStateTheTeamNoLongerHasStopsTheAdvanceRatherThanPickingAnother(t *testing.T) {
	removed := entity.SCMTeamSettings{MergedStateID: uuid.New()}

	states := []entity.WorkflowState{
		{ID: uuid.New(), Name: "Todo"},
		{ID: uuid.New(), Name: "Done", IsCompletion: true},
	}

	if state, ok := removed.TargetState(states); ok {
		t.Fatalf(
			"TargetState fell back to %q after the named state was deleted; a team that chose "+
				"where merged work goes must not silently have it sent somewhere else",
			state.Name,
		)
	}
}

func TestALinkOutlivesTheConnectionThatFoundIt(t *testing.T) {
	link := entity.CodeLink{
		Provider:   entity.SCMProviderGitHub,
		Repository: "acme/api",
		Kind:       entity.CodeLinkChange,
		Number:     41,
		URL:        "https://github.com/acme/api/pull/41",
		Title:      "Drop the cache on write",
		State:      entity.CodeChangeMerged,
	}

	if !link.Disconnected() {
		t.Fatal("a link with no connection must report itself disconnected")
	}

	if link.URL == "" || link.Title == "" || link.Repository == "" {
		t.Fatal(
			"a disconnected link still has to render on the issue, so what it needs to be " +
				"readable cannot live on the connection row",
		)
	}
}
