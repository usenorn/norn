package entity_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestStateCategoryAcceptsOnlyTheFourKnownCategories(t *testing.T) {
	cases := map[entity.StateCategory]bool{
		entity.StateCategoryNotStarted: true,
		entity.StateCategoryActive:     true,
		entity.StateCategoryComplete:   true,
		entity.StateCategoryAbandoned:  true,
		"":                             false,
		"NOT_STARTED":                  false,
		"blocked":                      false,
		"done":                         false,
	}

	for category, want := range cases {
		if got := category.Valid(); got != want {
			t.Errorf("StateCategory(%q).Valid() = %t, want %t", category, got, want)
		}
	}
}

func TestValidateWorkflowStateName(t *testing.T) {
	cases := map[string]struct {
		name string
		code string
	}{
		"empty":           {"", entity.ValidationCodeRequired},
		"whitespace only": {"   ", entity.ValidationCodeRequired},
		"ordinary":        {"In review", ""},
		"at the limit":    {strings.Repeat("a", entity.WorkflowStateNameMaxLen), ""},
		"over the limit":  {strings.Repeat("a", entity.WorkflowStateNameMaxLen+1), entity.ValidationCodeTooLong},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			if got := entity.ValidateWorkflowStateName("name", testCase.name).Code; got != testCase.code {
				t.Fatalf("code = %q, want %q", got, testCase.code)
			}
		})
	}
}

func TestANewTeamsDefaultStatesCoverEveryCategory(t *testing.T) {
	states := entity.DefaultWorkflowStates(uuid.New(), uuid.New())

	present := make(map[entity.StateCategory]bool, len(states))

	for _, state := range states {
		present[state.Category] = true
	}

	for _, category := range entity.StateCategories() {
		if !present[category] {
			t.Errorf("a new team has no %q state, so the system has nowhere to put such an issue", category)
		}
	}
}

func TestANewTeamHasExactlyOneDefaultAndOneCompletionState(t *testing.T) {
	states := entity.DefaultWorkflowStates(uuid.New(), uuid.New())

	var defaults, completions int

	for _, state := range states {
		if state.IsDefault {
			defaults++
		}

		if state.IsCompletion {
			completions++
		}
	}

	if defaults != 1 {
		t.Errorf("a new team has %d default states, want exactly 1", defaults)
	}

	if completions != 1 {
		t.Errorf("a new team has %d completion states, want exactly 1", completions)
	}
}

func TestTheCompletionStateIsAlwaysInTheCompleteCategory(t *testing.T) {
	for _, state := range entity.DefaultWorkflowStates(uuid.New(), uuid.New()) {
		if state.IsCompletion && state.Category != entity.StateCategoryComplete {
			t.Fatalf("completion state %q is in category %q, want %q", state.Name, state.Category, entity.StateCategoryComplete)
		}
	}
}

func TestDefaultStatePositionsAreOneThroughNWithoutGaps(t *testing.T) {
	states := entity.DefaultWorkflowStates(uuid.New(), uuid.New())

	for i, state := range states {
		if state.Position != i+1 {
			t.Fatalf("state %q has position %d, want %d", state.Name, state.Position, i+1)
		}
	}
}

func TestDefaultStatesBelongToTheTeamTheyWereBuiltFor(t *testing.T) {
	workspaceID := uuid.New()
	teamID := uuid.New()

	for _, state := range entity.DefaultWorkflowStates(workspaceID, teamID) {
		if state.WorkspaceID != workspaceID || state.TeamID != teamID {
			t.Fatalf("state %q belongs to %v/%v, want %v/%v", state.Name, state.WorkspaceID, state.TeamID, workspaceID, teamID)
		}
	}
}

func TestTwoTeamsWithDifferentStateNamesMapOntoTheSameFourCategories(t *testing.T) {
	platform := entity.DefaultWorkflowStates(uuid.New(), uuid.New())

	design := []entity.WorkflowState{
		{Name: "Icebox", Category: entity.StateCategoryNotStarted},
		{Name: "Ready", Category: entity.StateCategoryNotStarted},
		{Name: "Sketching", Category: entity.StateCategoryActive},
		{Name: "Shipped", Category: entity.StateCategoryComplete},
		{Name: "Dropped", Category: entity.StateCategoryAbandoned},
	}

	categoriesOf := func(states []entity.WorkflowState) map[entity.StateCategory]bool {
		present := make(map[entity.StateCategory]bool, len(states))

		for _, state := range states {
			present[state.Category] = true
		}

		return present
	}

	platformCategories := categoriesOf(platform)
	designCategories := categoriesOf(design)

	for _, category := range entity.StateCategories() {
		if !platformCategories[category] || !designCategories[category] {
			t.Fatalf(
				"category %q is missing from one of the teams, so combined progress could not be computed",
				category,
			)
		}
	}

	shared := 0

	for _, platformState := range platform {
		for _, designState := range design {
			if strings.EqualFold(platformState.Name, designState.Name) {
				shared++
			}
		}
	}

	if shared != 0 {
		t.Fatalf("the two teams share %d state names, so the test is not proving name-independence", shared)
	}
}
