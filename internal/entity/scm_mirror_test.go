package entity_test

import (
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

func TestOnlyAFieldBothSidesTouchedIsAConflict(t *testing.T) {
	earlier := time.Date(2026, 8, 7, 9, 0, 0, 0, time.UTC)
	later := earlier.Add(time.Hour)

	cases := []struct {
		name                       string
		nornChanged, sourceChanged bool
		nornAt, sourceAt           time.Time
		want                       entity.MirrorWinner
	}{
		{
			name: "neither side moved, so nothing is written anywhere",
			want: entity.MirrorWinnerNeither,
		},
		{
			name:        "only norn moved, so norn is pushed out",
			nornChanged: true,
			nornAt:      later,
			sourceAt:    earlier,
			want:        entity.MirrorWinnerNorn,
		},
		{
			name:          "only the platform moved, so the platform is pulled in",
			sourceChanged: true,
			nornAt:        later,
			sourceAt:      earlier,
			want:          entity.MirrorWinnerSource,
		},
		{
			name:          "both moved and the platform moved last",
			nornChanged:   true,
			sourceChanged: true,
			nornAt:        earlier,
			sourceAt:      later,
			want:          entity.MirrorWinnerSource,
		},
		{
			name:          "both moved and norn moved last",
			nornChanged:   true,
			sourceChanged: true,
			nornAt:        later,
			sourceAt:      earlier,
			want:          entity.MirrorWinnerNorn,
		},
		{
			name:          "both moved at the same instant, and norn is not made the follower",
			nornChanged:   true,
			sourceChanged: true,
			nornAt:        later,
			sourceAt:      later,
			want:          entity.MirrorWinnerNorn,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := entity.ArbitrateMirror(
				testCase.nornChanged,
				testCase.sourceChanged,
				testCase.nornAt,
				testCase.sourceAt,
			)

			if got != testCase.want {
				t.Fatalf("ArbitrateMirror = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestAFieldNobodyTouchedInNornIsNotReadAsChangedBecauseAnotherFieldWas(t *testing.T) {
	mirror := entity.IssueMirror{SyncedVersion: 4}

	issue := entity.Issue{
		Version:       6,
		FieldVersions: map[string]int{entity.IssueFieldTitle: 6},
	}

	if !mirror.NornChanged(entity.IssueFieldTitle, issue) {
		t.Error("the title was stamped above the synced version and must read as changed")
	}

	if mirror.NornChanged(entity.IssueFieldDescription, issue) {
		t.Error(
			"the description carries no stamp of its own, so reading the issue version alone " +
				"would make every field look edited whenever any one of them was",
		)
	}
}

func TestAnIssueNeverSyncedCountsAsOursToPush(t *testing.T) {
	mirror := entity.IssueMirror{SyncedVersion: 0}

	issue := entity.Issue{Version: 1}

	for _, field := range []string{
		entity.IssueFieldTitle,
		entity.IssueFieldDescription,
		entity.IssueFieldState,
	} {
		if !mirror.NornChanged(field, issue) {
			t.Errorf("%s must be pushed on the first sync, when nothing has been agreed yet", field)
		}
	}
}

func TestWhatWeLastWroteComingBackIsNotAChange(t *testing.T) {
	const title = "Drop the cache on write"

	mirror := entity.IssueMirror{TitleHash: entity.MirrorHash(title)}

	if mirror.SourceChanged(entity.IssueFieldTitle, title) {
		t.Fatal(
			"the platform echoed back exactly what we pushed and it read as a platform edit; " +
				"that is the loop where two systems overwrite each other forever",
		)
	}

	if !mirror.SourceChanged(entity.IssueFieldTitle, title+" again") {
		t.Fatal("a genuinely different title must read as a platform edit")
	}
}

func TestABodyComingBackWithTheForgesLineEndingsIsNotAnEdit(t *testing.T) {
	stored := "First line\nSecond line\n\nFourth line"
	returned := "First line\r\nSecond line\r\n\r\nFourth line"

	mirror := entity.IssueMirror{BodyHash: entity.MirrorHash(stored)}

	if mirror.SourceChanged(entity.IssueFieldDescription, returned) {
		t.Fatal(
			"the forge handed back the body we pushed with its own line endings and it read as " +
				"a foreign edit. Norn would apply it, push the result, and be handed it back " +
				"again — the two systems overwrite each other for as long as both are running",
		)
	}

	if !mirror.SourceChanged(entity.IssueFieldDescription, returned+"\r\nFifth line") {
		t.Fatal("normalising line endings must not blind the digest to a real edit")
	}
}

func TestSurroundingWhitespaceIsNotAnEditEither(t *testing.T) {
	mirror := entity.IssueMirror{TitleHash: entity.MirrorHash("Drop the cache on write")}

	if mirror.SourceChanged(entity.IssueFieldTitle, "  Drop the cache on write\n") {
		t.Fatal("a trimmed-and-padded round trip must not read as an edit")
	}
}

func TestContentTheConnectionsOwnTokenWroteIsNotAForeignChange(t *testing.T) {
	connection := entity.SCMConnection{IdentityLogin: "norn-bot"}

	if !connection.Wrote("norn-bot") {
		t.Error("the token's own login must be recognised however the forge cases it")
	}

	if !connection.Wrote("Norn-Bot") {
		t.Error("forges do not agree on the case of a login, so the comparison cannot either")
	}

	if connection.Wrote("octocat") {
		t.Error("a person's own comment must not be mistaken for the integration's voice")
	}

	if (entity.SCMConnection{}).Wrote("") {
		t.Error(
			"a connection that never learned its own login must not claim it wrote content " +
				"whose author is unknown; that would silently drop every anonymous change",
		)
	}
}

func TestAHashIsHeldPerFieldSoOneFieldCannotSpeakForAnother(t *testing.T) {
	mirror := entity.IssueMirror{
		TitleHash: entity.MirrorHash("a title"),
		BodyHash:  entity.MirrorHash("a body"),
		StateHash: entity.MirrorHash("open"),
	}

	if mirror.HashFor(entity.IssueFieldTitle) == mirror.HashFor(entity.IssueFieldDescription) {
		t.Fatal("title and description must not share a hash")
	}

	if mirror.HashFor(entity.IssueFieldPriority) != "" {
		t.Fatal("a field the mirror does not carry must not answer with another field's hash")
	}
}
