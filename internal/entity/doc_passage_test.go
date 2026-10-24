package entity_test

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/usenorn/norn/internal/entity"
)

func passagesOf(title, markdown string) []entity.DocPassage {
	return entity.Doc{
		DocSummary: entity.DocSummary{Title: title},
		Content:    entity.DocumentFromMarkdown(markdown),
	}.Passages()
}

func TestAPassageCarriesTheHeadingsAboveIt(t *testing.T) {
	got := passagesOf("Runbook", "Intro.\n\n# Deploys\n\nShip on Tuesdays.\n\n## Rollback\n\nRevert the tag.\n\n# Alerts\n\nPage the owner.\n")

	want := []entity.DocPassage{
		{Position: 0, Heading: "Runbook", Body: "Intro."},
		{Position: 1, Heading: "Runbook › Deploys", Body: "Ship on Tuesdays."},
		{Position: 2, Heading: "Runbook › Deploys › Rollback", Body: "Revert the tag."},
		{Position: 3, Heading: "Runbook › Alerts", Body: "Page the owner."},
	}

	if !slices.Equal(got, want) {
		t.Fatalf("passages = %#v\nwant %#v", got, want)
	}
}

func TestAHeadingWithNothingUnderItIsStillFindable(t *testing.T) {
	got := passagesOf("Runbook", "# Deploys\n\n# Alerts\n\nPage the owner.\n")

	want := []entity.DocPassage{
		{Position: 0, Heading: "Runbook › Deploys", Body: ""},
		{Position: 1, Heading: "Runbook › Alerts", Body: "Page the owner."},
	}

	if !slices.Equal(got, want) {
		t.Fatalf("passages = %#v\nwant %#v", got, want)
	}
}

func TestAnEmptyDocIsOnePassageHoldingItsTitle(t *testing.T) {
	got := passagesOf("Empty plan", "")

	if !slices.Equal(got, []entity.DocPassage{{Heading: "Empty plan"}}) {
		t.Fatalf("passages = %#v, want the title alone so the doc is still found by name", got)
	}
}

func TestALongSectionIsSplitWithoutLosingText(t *testing.T) {
	paragraph := strings.TrimSpace(strings.Repeat("deploys need a second reviewer ", 30))
	markdown := "# Policy\n\n" + strings.Repeat(paragraph+"\n\n", 4) + strings.Repeat("x", entity.DocPassageMaxRunes+10) + "\n"

	got := passagesOf("Handbook", markdown)

	if len(got) < 3 {
		t.Fatalf("got %d passages, want the section split into several", len(got))
	}

	var words int

	for index, passage := range got {
		if passage.Position != index || passage.Heading != "Handbook › Policy" {
			t.Errorf("passage %d = position %d heading %q", index, passage.Position, passage.Heading)
		}

		if size := utf8.RuneCountInString(passage.Body); size > entity.DocPassageMaxRunes {
			t.Errorf("passage %d holds %d runes, over the %d cap", index, size, entity.DocPassageMaxRunes)
		}

		words += strings.Count(passage.Body, "reviewer")
	}

	if words != 4*30 {
		t.Errorf("found %d copies of the phrase across passages, want %d", words, 4*30)
	}
}
