package scm

import (
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestWhatWePushedComingBackUnchangedIsNotPushedAgain(t *testing.T) {
	issue := entity.Issue{
		Title:       "Payments time out under load",
		Description: "Under 500rps the gateway drops.",
	}

	agreed := entity.IssueMirror{
		TitleHash: entity.MirrorHash(issue.Title),
		BodyHash:  entity.MirrorHash(issue.Description),
	}

	if _, changed := outboundPatch(agreed, issue); changed {
		t.Fatal(
			"the sweep would send a title neither side had changed. Every cycle would rewrite " +
				"the platform issue and burn the rate limit for nothing",
		)
	}
}

func TestTheForgesOwnLineEndingsComingBackAreNotAChangeToPush(t *testing.T) {
	issue := entity.Issue{
		Title:       "Payments time out",
		Description: "First line\nSecond line",
	}

	agreed := entity.IssueMirror{
		TitleHash: entity.MirrorHash(issue.Title),
		BodyHash:  entity.MirrorHash("First line\r\nSecond line"),
	}

	if _, changed := outboundPatch(agreed, issue); changed {
		t.Fatal(
			"a body differing only in line endings read as an edit to push. Norn would send it, " +
				"the forge would hand it back with its own endings, and the two would rewrite each " +
				"other for as long as both are running",
		)
	}
}

func TestAnEditMadeInNornIsSentAndOnlyTheFieldThatMoved(t *testing.T) {
	agreed := entity.IssueMirror{
		TitleHash: entity.MirrorHash("Payments time out"),
		BodyHash:  entity.MirrorHash("Under 500rps the gateway drops."),
	}

	edited := entity.Issue{
		Title:       "Payments time out under load",
		Description: "Under 500rps the gateway drops.",
	}

	patch, changed := outboundPatch(agreed, edited)

	if !changed || patch.Title == nil {
		t.Fatal("a title edited in Norn was not sent, so the two sides stay out of step")
	}

	if *patch.Title != edited.Title {
		t.Fatalf("patch title = %q, want %q", *patch.Title, edited.Title)
	}

	if patch.Body != nil {
		t.Fatal(
			"an untouched description was sent alongside the title. Rewriting a body nobody " +
				"changed loses whatever the forge did to it in the meantime",
		)
	}
}

func TestAnIssuePairedButNeverSyncedIsSentInFull(t *testing.T) {
	fresh := entity.IssueMirror{}

	issue := entity.Issue{Title: "Payments time out", Description: "Under load."}

	patch, changed := outboundPatch(fresh, issue)

	if !changed || patch.Title == nil || patch.Body == nil {
		t.Fatal("nothing has been agreed yet, so both fields have to go out on the first pass")
	}
}
