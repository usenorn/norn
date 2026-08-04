package entity_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestEveryOfferedColourIsValidAndNothingElseIs(t *testing.T) {
	for _, color := range entity.LabelColors() {
		if !color.Valid() {
			t.Errorf("offered colour %q is not valid", color)
		}
	}

	signal := []entity.LabelColor{"red", "amber", "green", "grey", "gray", "ink", "", "Cyan"}

	for _, color := range signal {
		if color.Valid() {
			t.Errorf("colour %q is representable, but status and priority own that family", color)
		}
	}
}

func cssHex(t *testing.T, prefixes ...string) map[string]string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the working directory")
		}

		dir = parent
	}

	source, err := os.ReadFile(filepath.Join(dir, "web", "src", "routes", "layout.css"))
	if err != nil {
		t.Fatalf("read layout.css: %v", err)
	}

	declaration := regexp.MustCompile(`--([a-z0-9-]+):\s*([^;]+);`)
	raw := map[string]string{}

	for _, match := range declaration.FindAllStringSubmatch(string(source), -1) {
		raw[match[1]] = strings.TrimSpace(match[2])
	}

	resolve := func(value string) string {
		for range 8 {
			inner := regexp.MustCompile(`^var\(--([a-z0-9-]+)\)$`).FindStringSubmatch(value)
			if inner == nil {
				return strings.ToLower(value)
			}

			value = raw[inner[1]]
		}

		return strings.ToLower(value)
	}

	resolved := map[string]string{}

	for name, value := range raw {
		for _, prefix := range prefixes {
			if strings.HasPrefix(name, prefix) {
				resolved[name] = resolve(value)
			}
		}
	}

	if len(resolved) == 0 {
		t.Fatalf("no tokens in layout.css matched %v", prefixes)
	}

	return resolved
}

func TestTheOfferedColoursExcludeEveryStatusAndPriorityColour(t *testing.T) {
	signal := cssHex(t, "state-", "priority-")
	labels := cssHex(t, "label-")

	byHex := map[string]string{}
	for name, hex := range signal {
		byHex[hex] = name
	}

	for _, color := range entity.LabelColors() {
		token := "label-" + string(color)

		hex, ok := labels[token]
		if !ok {
			t.Errorf("offered colour %q has no --%s token in layout.css", color, token)

			continue
		}

		if clash, taken := byHex[hex]; taken {
			t.Errorf(
				"label colour %q resolves to %s, which --%s already uses for status or priority",
				color, hex, clash,
			)
		}
	}
}

func TestEveryLabelColourTokenIsOffered(t *testing.T) {
	offered := map[string]struct{}{}
	for _, color := range entity.LabelColors() {
		offered["label-"+string(color)] = struct{}{}
	}

	for token := range cssHex(t, "label-") {
		if _, ok := offered[token]; !ok {
			t.Errorf("--%s exists in layout.css but is not offered by entity.LabelColors()", token)
		}
	}
}

func TestAWorkspaceLabelAppliesToEveryTeamAndATeamLabelOnlyToItsOwn(t *testing.T) {
	mobile := uuid.New()
	platform := uuid.New()

	workspaceWide := entity.Label{Name: "Bug"}
	teamScoped := entity.Label{Name: "Crash", TeamID: mobile}

	cases := []struct {
		label entity.Label
		team  uuid.UUID
		want  bool
	}{
		{workspaceWide, mobile, true},
		{workspaceWide, platform, true},
		{teamScoped, mobile, true},
		{teamScoped, platform, false},
	}

	for _, tc := range cases {
		if got := tc.label.AppliesTo(tc.team); got != tc.want {
			t.Errorf("%q.AppliesTo(%v) = %t, want %t", tc.label.Name, tc.team, got, tc.want)
		}
	}
}

func TestAMergeTargetMustCoverTheSourceScope(t *testing.T) {
	workspaceID := uuid.New()
	mobile := uuid.New()
	platform := uuid.New()

	wide := entity.Label{WorkspaceID: workspaceID}
	onMobile := entity.Label{WorkspaceID: workspaceID, TeamID: mobile}
	onPlatform := entity.Label{WorkspaceID: workspaceID, TeamID: platform}
	elsewhere := entity.Label{WorkspaceID: uuid.New()}

	cases := map[string]struct {
		target entity.Label
		source entity.Label
		want   bool
	}{
		"widening a team label to the workspace": {wide, onMobile, true},
		"within one team":                        {onMobile, onMobile, true},
		"two workspace labels":                   {wide, wide, true},
		"narrowing to a team strands issues":     {onMobile, wide, false},
		"across two teams strands issues":        {onMobile, onPlatform, false},
		"across workspaces":                      {wide, elsewhere, false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := tc.target.Covers(tc.source); got != tc.want {
				t.Errorf("Covers = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestASubmittedSetMayCarryAtMostOneLabelPerGroup(t *testing.T) {
	severity := uuid.New()
	area := uuid.New()

	blocker := entity.Label{Name: "Blocker", GroupID: severity}
	major := entity.Label{Name: "Major", GroupID: severity}
	backend := entity.Label{Name: "Backend", GroupID: area}
	loose := entity.Label{Name: "Needs spec"}
	alsoLoose := entity.Label{Name: "Regression"}

	cases := map[string]struct {
		labels []entity.Label
		want   bool
	}{
		"two from one group":          {[]entity.Label{blocker, major}, true},
		"one from each group":         {[]entity.Label{blocker, backend}, false},
		"grouped and ungrouped":       {[]entity.Label{blocker, loose}, false},
		"several ungrouped":           {[]entity.Label{loose, alsoLoose}, false},
		"empty":                       {nil, false},
		"two from one group and more": {[]entity.Label{backend, blocker, loose, major}, true},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := entity.GroupedLabelConflict(tc.labels); got != tc.want {
				t.Errorf("GroupedLabelConflict = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestLabelNamesAreBoundedAndTrimmed(t *testing.T) {
	cases := map[string]struct {
		name string
		code string
	}{
		"empty":            {"", entity.ValidationCodeRequired},
		"only whitespace":  {"   ", entity.ValidationCodeRequired},
		"at the limit":     {strings.Repeat("a", entity.LabelNameMaxLen), ""},
		"over the limit":   {strings.Repeat("a", entity.LabelNameMaxLen+1), entity.ValidationCodeTooLong},
		"ordinary":         {"Needs spec", ""},
		"counts runes":     {strings.Repeat("é", entity.LabelNameMaxLen), ""},
		"runes over limit": {strings.Repeat("é", entity.LabelNameMaxLen+1), entity.ValidationCodeTooLong},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := entity.ValidateLabelName("name", tc.name).Code; got != tc.code {
				t.Errorf("ValidateLabelName code = %q, want %q", got, tc.code)
			}
		})
	}
}
