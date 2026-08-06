package entity_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestNoSourceUserPassesThroughUndecided(t *testing.T) {
	decided := uuid.New()

	plan := entity.MappingPlan{Mappings: []entity.ImportMapping{
		{Kind: entity.ImportMapUser, SourceKey: "rae", Decision: entity.ImportDecisionMap, TargetID: decided},
		{Kind: entity.ImportMapUser, SourceKey: "sam", Decision: entity.ImportDecisionUnattributed},
		{Kind: entity.ImportMapUser, SourceKey: "kit"},
	}}

	if plan.Complete() {
		t.Fatal(
			"a plan with an undecided source user reports itself complete. Silently attributing " +
				"their work to whoever ran the import is exactly what the mapping stage exists " +
				"to prevent.",
		)
	}

	undecided := plan.Undecided()

	if len(undecided) != 1 || undecided[0].SourceKey != "kit" {
		t.Fatalf("undecided = %v, want exactly kit", undecided)
	}

	plan.Mappings[2].Decision = entity.ImportDecisionSkip

	if !plan.Complete() {
		t.Error("a plan where every source user has been decided still reports itself incomplete")
	}
}

func TestLeavingSomebodyUnattributedIsADecisionNotAnAbsence(t *testing.T) {
	plan := entity.MappingPlan{Mappings: []entity.ImportMapping{
		{Kind: entity.ImportMapUser, SourceKey: "sam", SourceLabel: "Sam Okafor", Decision: entity.ImportDecisionUnattributed},
		{Kind: entity.ImportMapUser, SourceKey: "rae", Decision: entity.ImportDecisionMap, TargetID: uuid.New()},
	}}

	unattributed := plan.Unattributed()

	if len(unattributed) != 1 || unattributed[0].SourceLabel != "Sam Okafor" {
		t.Fatalf(
			"unattributed = %v, want Sam Okafor. Every person whose work arrives without an "+
				"owner has to be nameable in the report.",
			unattributed,
		)
	}
}

func TestOnlyAPersonCanBeLeftUnattributed(t *testing.T) {
	mapping := entity.ImportMapping{
		Kind:     entity.ImportMapLabel,
		Decision: entity.ImportDecisionUnattributed,
	}

	if err := mapping.Validate(); err == nil {
		t.Error("a label can be left unattributed, which means nothing")
	}
}

func TestAMappingThatPointsNowhereIsRefused(t *testing.T) {
	cases := []struct {
		name    string
		mapping entity.ImportMapping
		wantErr bool
	}{
		{
			"mapped onto an account",
			entity.ImportMapping{Kind: entity.ImportMapUser, Decision: entity.ImportDecisionMap, TargetID: uuid.New()},
			false,
		},
		{
			"mapped onto a value",
			entity.ImportMapping{Kind: entity.ImportMapPriority, Decision: entity.ImportDecisionMap, TargetValue: "high"},
			false,
		},
		{
			"mapped onto nothing at all",
			entity.ImportMapping{Kind: entity.ImportMapState, Decision: entity.ImportDecisionMap},
			true,
		},
		{
			"a concept nobody declared",
			entity.ImportMapping{Kind: "sprint", Decision: entity.ImportDecisionSkip},
			true,
		},
		{
			"a decision nobody declared",
			entity.ImportMapping{Kind: entity.ImportMapState, Decision: "guess"},
			true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.mapping.Validate()

			if testCase.wantErr && err == nil {
				t.Error("accepted")
			}

			if !testCase.wantErr && err != nil {
				t.Errorf("refused: %v", err)
			}
		})
	}
}

func TestAnAddressMatchesBeforeANameDoes(t *testing.T) {
	byEmail, byName := uuid.New(), uuid.New()

	candidates := []entity.ImportCandidate{
		{AccountID: byName, DisplayName: "Rae Whitfield", Email: "someone.else@northwind.co"},
		{AccountID: byEmail, DisplayName: "R. Whitfield", Email: "rae@northwind.co"},
	}

	suggested := entity.SuggestAccount(entity.ImportMapping{
		Kind:        entity.ImportMapUser,
		SourceKey:   "rae",
		SourceLabel: "Rae Whitfield",
		SourceEmail: "RAE@Northwind.co",
	}, candidates)

	if suggested.SuggestedTargetID != byEmail {
		t.Fatalf(
			"suggested %v by %q. An address identifies a person and a display name merely "+
				"describes them, so two people called Rae Whitfield must not collide.",
			suggested.SuggestedTargetID, suggested.SuggestedReason,
		)
	}

	if suggested.SuggestedReason != entity.SuggestedByEmail {
		t.Errorf("reason = %q, want the address match", suggested.SuggestedReason)
	}
}

func TestASuggestionIsNeverADecision(t *testing.T) {
	suggested := entity.SuggestAccount(
		entity.ImportMapping{Kind: entity.ImportMapUser, SourceEmail: "rae@northwind.co"},
		[]entity.ImportCandidate{{AccountID: uuid.New(), Email: "rae@northwind.co"}},
	)

	if !suggested.Undecided() {
		t.Fatal(
			"a matched suggestion counts as decided. A confident guess is still a guess, and " +
				"attribution is the operator's to confirm.",
		)
	}

	if _, resolved := (entity.MappingPlan{Mappings: []entity.ImportMapping{suggested}}).
		Target(entity.ImportMapUser, suggested.SourceKey); resolved {
		t.Error("a suggestion resolves to a target, so an unconfirmed guess would be applied")
	}
}

func TestNobodyIsSuggestedWhenNothingMatches(t *testing.T) {
	suggested := entity.SuggestAccount(
		entity.ImportMapping{Kind: entity.ImportMapUser, SourceEmail: "gone@elsewhere.example"},
		[]entity.ImportCandidate{{AccountID: uuid.New(), Email: "rae@northwind.co", DisplayName: "Rae"}},
	)

	if suggested.SuggestedTargetID != uuid.Nil {
		t.Errorf("suggested %v for somebody with no counterpart here", suggested.SuggestedTargetID)
	}
}

func TestASkippedConceptIsDistinctFromAnUnmappedOne(t *testing.T) {
	plan := entity.MappingPlan{Mappings: []entity.ImportMapping{
		{Kind: entity.ImportMapLabel, SourceKey: "wontfix", Decision: entity.ImportDecisionSkip},
	}}

	if !plan.Skips(entity.ImportMapLabel, "wontfix") {
		t.Error("a skipped label does not report itself skipped")
	}

	if plan.Skips(entity.ImportMapLabel, "bug") {
		t.Error("a label nobody mentioned reports itself deliberately skipped")
	}
}

func TestAnImportedColourBecomesTheNearestOneNornHas(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  entity.LabelColor
	}{
		{"an exact swatch", "#58b4c4", entity.LabelColorCyan},
		{"near enough to magenta", "#d07ab8", entity.LabelColorMagenta},
		{"a grey", "#8b93a5", entity.LabelColorNeutral},
		{"shorthand", "#6b9", entity.LabelColorCyan},
		{"no hash", "8e86d9", entity.LabelColorViolet},
		{"not a colour at all", "cornflower", entity.LabelColorNeutral},
		{"empty", "", entity.LabelColorNeutral},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := entity.NearestLabelColor(testCase.value)

			if got != testCase.want {
				t.Errorf("colour for %q = %q, want %q", testCase.value, got, testCase.want)
			}

			if !got.Valid() {
				t.Errorf("colour for %q = %q, which is not one this instance can store", testCase.value, got)
			}
		})
	}
}

func TestEveryLabelColourIsReachableFromItsOwnSwatch(t *testing.T) {
	swatches := map[entity.LabelColor]string{
		entity.LabelColorNeutral: "#8a93a6",
		entity.LabelColorCyan:    "#58b4c4",
		entity.LabelColorBlue:    "#6b93d6",
		entity.LabelColorViolet:  "#8e86d9",
		entity.LabelColorOrchid:  "#b07ad0",
		entity.LabelColorMagenta: "#cb77b4",
	}

	for _, colour := range entity.LabelColors() {
		hex, known := swatches[colour]

		if !known {
			t.Errorf(
				"%q has no swatch here, so an imported colour can never become it and the "+
					"palette has drifted from the one the dashboard paints",
				colour,
			)

			continue
		}

		if got := entity.NearestLabelColor(hex); got != colour {
			t.Errorf("its own swatch %s maps to %q rather than %q", hex, got, colour)
		}
	}
}
