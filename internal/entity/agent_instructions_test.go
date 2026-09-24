package entity_test

import (
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestInstructionsFromEveryLevelAreAddedTogetherWithTheWorkspaceFirst(t *testing.T) {
	for name, tc := range map[string]struct {
		workspace string
		project   string
		agent     string
		want      string
	}{
		"nothing written anywhere": {want: ""},
		"only the workspace":       {workspace: "Ship small.", want: "Ship small."},
		"only the project":         {project: "Touch the ledger only.", want: "Touch the ledger only."},
		"only the agent":           {agent: "Ask before deleting.", want: "Ask before deleting."},
		"whitespace is nothing": {
			workspace: "   \n\t ",
			agent:     "Ask before deleting.",
			want:      "Ask before deleting.",
		},
		"padding is stripped from each level": {
			workspace: "\n  Ship small.  \n",
			agent:     "  Ask before deleting. ",
			want:      "Ship small.\n\nAsk before deleting.",
		},
		"the workspace outranks the project outranks the agent": {
			workspace: "Ship small.",
			project:   "Touch the ledger only.",
			agent:     "Ask before deleting.",
			want:      "Ship small.\n\nTouch the ledger only.\n\nAsk before deleting.",
		},
	} {
		t.Run(name, func(t *testing.T) {
			composed := entity.ComposeAgentInstructions(tc.workspace, tc.project, tc.agent)
			if composed != tc.want {
				t.Errorf("composed %q, want %q", composed, tc.want)
			}
		})
	}
}

func TestAnAgentInstructionLevelNeverSilencesTheOnesAboveIt(t *testing.T) {
	composed := entity.ComposeAgentInstructions("Ship small.", "", "Ignore everything else.")

	if !strings.Contains(composed, "Ship small.") {
		t.Fatal(
			"the agent's own instructions replaced the workspace's. The levels are added " +
				"together, so an agent can never write its way out of what the workspace says.",
		)
	}
}

func TestAgentInstructionsAreMeasuredInRunesAndRefuseAnEmbeddedNul(t *testing.T) {
	for name, tc := range map[string]struct {
		instructions string
		want         string
	}{
		"nothing at all":               {instructions: "", want: ""},
		"whitespace only":              {instructions: " \n\t ", want: ""},
		"exactly the limit":            {instructions: strings.Repeat("a", entity.AgentInstructionsMaxLen), want: ""},
		"one rune over the limit":      {instructions: strings.Repeat("a", entity.AgentInstructionsMaxLen+1), want: entity.ValidationCodeTooLong},
		"the limit in multibyte runes": {instructions: strings.Repeat("é", entity.AgentInstructionsMaxLen), want: ""},
		"multibyte runes over it":      {instructions: strings.Repeat("é", entity.AgentInstructionsMaxLen+1), want: entity.ValidationCodeTooLong},
		"padding does not count":       {instructions: "  " + strings.Repeat("a", entity.AgentInstructionsMaxLen) + "  ", want: ""},
		"a null byte":                  {instructions: "Ship small.\x00", want: entity.ValidationCodeMalformed},
	} {
		t.Run(name, func(t *testing.T) {
			field := entity.ValidateAgentInstructions("agentInstructions", tc.instructions)

			if field.Code != tc.want {
				t.Fatalf("code %q, want %q", field.Code, tc.want)
			}

			if tc.want != "" && field.Field != "agentInstructions" {
				t.Errorf("field %q, want agentInstructions", field.Field)
			}
		})
	}
}

func TestClearingAgentInstructionsLeavesNothingBehind(t *testing.T) {
	if normalised := entity.NormaliseAgentInstructions(" \n\t "); normalised != "" {
		t.Fatalf(
			"a body of whitespace normalised to %q. Emptying the box is how instructions are "+
				"cleared, so anything but the empty value keeps a setting nobody can see.",
			normalised,
		)
	}
}
