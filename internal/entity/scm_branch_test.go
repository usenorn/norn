package entity_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/usenorn/norn/internal/entity"
)

func TestABranchNameIsSomethingGitWillAccept(t *testing.T) {
	settings := entity.SCMTeamSettings{}

	cases := map[string]string{
		"Drop the cache on write":    "rae/eng-12-drop-the-cache-on-write",
		"Fix: the importer (again!)": "rae/eng-12-fix-the-importer-again",
		"  leading and trailing   ":  "rae/eng-12-leading-and-trailing",
		"..dots..and::colons":        "rae/eng-12-dots-and-colons",
		"кириллица тоже":             "rae/eng-12-кириллица-тоже",
	}

	for title, want := range cases {
		t.Run(title, func(t *testing.T) {
			got := settings.BranchName("Rae", "ENG-12", title)

			if got != want {
				t.Fatalf("BranchName(%q) = %q, want %q", title, got, want)
			}

			for _, refused := range []string{" ", ":", "..", "~", "^", "?", "*", "["} {
				if strings.Contains(got, refused) {
					t.Errorf("%q contains %q, which git refuses in a ref", got, refused)
				}
			}

			if strings.HasSuffix(got, ".") || strings.HasSuffix(got, "/") ||
				strings.HasSuffix(got, "-") {
				t.Errorf("%q ends in a character git refuses at the end of a ref", got)
			}
		})
	}
}

func TestATitleThatIsAllPunctuationDoesNotLeaveADanglingBranch(t *testing.T) {
	settings := entity.SCMTeamSettings{}

	got := settings.BranchName("rae", "ENG-12", "!!! ??? ...")

	if got != "rae/eng-12" {
		t.Fatalf("BranchName = %q, want the reference alone with no trailing dash", got)
	}
}

func TestALongTitleIsCutRatherThanHandedToGitWhole(t *testing.T) {
	settings := entity.SCMTeamSettings{}

	got := settings.BranchName("rae", "ENG-12", strings.Repeat("word ", 80))

	if len(got) > entity.SCMBranchNameMax {
		t.Fatalf("BranchName is %d characters, over the %d limit", len(got), entity.SCMBranchNameMax)
	}

	if strings.HasSuffix(got, "-") {
		t.Fatalf("BranchName = %q; cutting mid-word must not leave a trailing dash", got)
	}
}

func TestCuttingALongTitleNeverSplitsAMultibyteCharacter(t *testing.T) {
	settings := entity.SCMTeamSettings{}

	cases := map[string]string{
		"the slug limit lands inside a cyrillic letter":        "tes: базовое покрытие — бакетинг, эвалюация, авторизация, API",
		"the slug limit lands inside the next cyrillic letter": "testt: базовое покрытие — бакетинг, эвалюация, авторизация, API",
		"a cyrillic title long enough to reach the name limit": strings.Repeat("длинный заголовок ", 40),
		"three-byte characters":                                strings.Repeat("日本語", 200),
	}

	for name, title := range cases {
		t.Run(name, func(t *testing.T) {
			slug := entity.SlugifyBranchPart(title)

			if !utf8.ValidString(slug) {
				t.Errorf(
					"SlugifyBranchPart(%q) = %q, which is not valid UTF-8; the cut landed "+
						"inside a character and left a stray byte in the branch name",
					title, slug,
				)
			}

			branch := settings.BranchName("Рае", "ENG-12", title)

			if !utf8.ValidString(branch) {
				t.Errorf("BranchName(%q) = %q, which is not valid UTF-8", title, branch)
			}

			if len(branch) > entity.SCMBranchNameMax {
				t.Errorf("BranchName is %d bytes, over the %d limit", len(branch), entity.SCMBranchNameMax)
			}
		})
	}
}

func TestATeamTemplateReplacesTheDefault(t *testing.T) {
	settings := entity.SCMTeamSettings{BranchTemplate: "feature/{reference}"}

	if got := settings.BranchName("rae", "ENG-12", "anything"); got != "feature/eng-12" {
		t.Fatalf("BranchName = %q, want the team's own template", got)
	}

	empty := entity.SCMTeamSettings{BranchTemplate: "   "}

	if empty.Template() != entity.SCMBranchTemplateDefault {
		t.Fatal("a blank template must fall back to the default rather than produce a blank branch")
	}
}

func TestATemplateThatNamesNoIssueIsRefused(t *testing.T) {
	if field := entity.ValidateSCMBranchTemplate("branchTemplate", "feature/{title}"); field.Field == "" {
		t.Fatal(
			"a template with no {reference} was accepted; the branch would name no issue and " +
				"the link it exists to create could never form",
		)
	}

	if field := entity.ValidateSCMBranchTemplate("branchTemplate", ""); field.Field != "" {
		t.Fatal("an empty template means the default, not an error")
	}

	if field := entity.ValidateSCMBranchTemplate("branchTemplate", "{reference}"); field.Field != "" {
		t.Fatal("a template naming the reference is valid")
	}
}
