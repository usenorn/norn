package entity_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/usenorn/norn/internal/entity"
)

func TestACommentBodyAcceptsTheSameRichTextADescriptionDoes(t *testing.T) {
	body := "Reproduced it:\n\n```go\nfunc main() {\n\tprintln(\"hi\")\n}\n```\n\n> and the log agrees"

	if got := entity.ValidateCommentBody("body", body); got.Code != "" {
		t.Fatalf(
			"a fenced code block was rejected with %q. Comments render through the same markdown "+
				"pipeline as descriptions, so anything a description accepts a comment accepts.",
			got.Code,
		)
	}
}

func TestAnEmptyCommentIsRefusedWhereAnEmptyDescriptionIsNot(t *testing.T) {
	for name, body := range map[string]string{
		"nothing at all": "",
		"only spaces":    "   ",
		"only newlines":  "\n\n\t\n",
	} {
		t.Run(name, func(t *testing.T) {
			got := entity.ValidateCommentBody("body", body)

			if got.Code != entity.ValidationCodeRequired {
				t.Fatalf(
					"a comment of %q was accepted with %q. A description may be blank because the "+
						"issue carries it; a comment that says nothing is not a comment.",
					body, got.Code,
				)
			}
		})
	}
}

func TestACommentIsMeasuredInRunesAndRejectsNulBytes(t *testing.T) {
	long := strings.Repeat("é", entity.CommentBodyMaxLen)

	if got := entity.ValidateCommentBody("body", long); got.Code != "" {
		t.Fatalf(
			"%d two-byte runes were rejected with %q — the limit is being counted in bytes",
			entity.CommentBodyMaxLen, got.Code,
		)
	}

	if got := entity.ValidateCommentBody("body", long+"é"); got.Code != entity.ValidationCodeTooLong {
		t.Fatalf("one rune over the limit was accepted with %q", got.Code)
	}

	if got := entity.ValidateCommentBody("body", "hello\x00there"); got.Code != entity.ValidationCodeMalformed {
		t.Fatalf("a NUL byte was accepted with %q", got.Code)
	}
}

func TestOnlyTheAuthorMayEditAndOnlyTheAuthorOrAnAdminMayDelete(t *testing.T) {
	author := uuid.New()
	other := uuid.New()
	comment := entity.IssueComment{AuthorAccountID: author}

	cases := map[string]struct {
		account  uuid.UUID
		role     entity.MembershipRole
		editable bool
		delible  bool
	}{
		"the author":                     {author, entity.MembershipRoleMember, true, true},
		"an author who is also an admin": {author, entity.MembershipRoleAdmin, true, true},
		"another member":                 {other, entity.MembershipRoleMember, false, false},
		"another viewer":                 {other, entity.MembershipRoleViewer, false, false},
		"a workspace admin":              {other, entity.MembershipRoleAdmin, false, true},
	}

	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			if got := comment.EditableBy(want.account); got != want.editable {
				t.Fatalf(
					"editable = %t, want %t. Editing rewrites what somebody said; nobody but them "+
						"may do that, an administrator included.",
					got, want.editable,
				)
			}

			if got := comment.DeletableBy(want.account, want.role); got != want.delible {
				t.Fatalf("deletable = %t, want %t", got, want.delible)
			}
		})
	}
}

func TestAnAuthorlessCommentBelongsToNobodyRatherThanToEveryNobody(t *testing.T) {
	orphaned := entity.IssueComment{}

	if orphaned.EditableBy(uuid.Nil) {
		t.Fatal(
			"a comment whose author left the workspace became editable by the zero account. " +
				"The author column is nulled on departure, so it must never match anything.",
		)
	}

	if orphaned.DeletableBy(uuid.Nil, entity.MembershipRoleMember) {
		t.Fatal("a comment whose author left the workspace became deletable by the zero account")
	}

	if !orphaned.DeletableBy(uuid.Nil, entity.MembershipRoleAdmin) {
		t.Fatal("an admin cannot clean up a comment whose author has gone")
	}
}

func TestATombstoneIsNeitherEditableNorDeletableAgain(t *testing.T) {
	author := uuid.New()
	at := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	tombstone := entity.IssueComment{AuthorAccountID: author, DeletedAt: &at}

	if tombstone.EditableBy(author) {
		t.Fatal("a deleted comment could be edited back into existence")
	}

	if tombstone.DeletableBy(author, entity.MembershipRoleAdmin) {
		t.Fatal("a deleted comment could be deleted twice, which would record two activity entries")
	}
}

func TestACursorSurvivesTheRoundTripAndARottenOneIsRefused(t *testing.T) {
	want := entity.CommentCursor{
		CreatedAt: time.Date(2026, 8, 4, 9, 15, 30, 123456789, time.UTC),
		CommentID: uuid.New(),
	}

	got, err := entity.DecodeCommentCursor(want.Encode())
	if err != nil {
		t.Fatalf("a cursor we produced would not decode: %v", err)
	}

	if got.CommentID != want.CommentID || !got.CreatedAt.Equal(want.CreatedAt) {
		t.Fatalf(
			"cursor round trip lost precision: %s/%s became %s/%s. Two comments posted in the same "+
				"second must still page apart.",
			want.CommentID, want.CreatedAt, got.CommentID, got.CreatedAt,
		)
	}

	for name, raw := range map[string]string{
		"not base64":   "!!!!",
		"empty":        "",
		"truncated":    want.Encode()[:8],
		"no timestamp": entity.CommentCursor{CommentID: want.CommentID}.Encode()[:48],
		"not a uuid":   "bm90LWEtdXVpZC1hdC1hbGwtbm90LWEtdXVpZDIwMjYtMDgtMDRUMDk6MDA6MDBa",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := entity.DecodeCommentCursor(raw); err == nil {
				t.Fatal("a malformed cursor decoded rather than being refused")
			}
		})
	}
}

func TestAPageIsBoundedAndTheLookaheadAsksForOneExtraRow(t *testing.T) {
	if got := (entity.CommentPage{}).Normalized().Limit; got != entity.CommentPageDefaultSize {
		t.Fatalf("an unset limit became %d, want the default %d", got, entity.CommentPageDefaultSize)
	}

	if got := (entity.CommentPage{Limit: 5000}).Normalized().Limit; got != entity.CommentPageMaxSize {
		t.Fatalf("a limit of 5000 was honoured as %d — a thread page is unbounded", got)
	}

	if got := (entity.CommentPage{Limit: 20}).Lookahead().Limit; got != 21 {
		t.Fatalf("lookahead asked for %d rows, want 21 — one extra is how we know there is more", got)
	}
}

func TestOnlyTheSixNamedReactionsAreValid(t *testing.T) {
	for _, reaction := range entity.CommentReactions() {
		if !reaction.Valid() {
			t.Fatalf("%q is offered by the product but rejected as invalid", reaction)
		}
	}

	for _, reaction := range []entity.CommentReaction{"", "🎉", "rocket", "UP", "thumbsup"} {
		if reaction.Valid() {
			t.Fatalf(
				"%q was accepted. The set is fixed so nothing unvalidated reaches a column the "+
					"screen has to render.",
				reaction,
			)
		}
	}
}

func TestAMentionResolvesToWhicheverTargetItsKindNames(t *testing.T) {
	account := uuid.New()
	team := uuid.New()

	person := entity.CommentMention{Kind: entity.MentionKindAccount, AccountID: account}
	if person.TargetID() != account {
		t.Fatal("an account mention did not resolve to its account")
	}

	group := entity.CommentMention{Kind: entity.MentionKindTeam, TeamID: team}
	if group.TargetID() != team {
		t.Fatal("a team mention did not resolve to its team")
	}
}

func TestTheMentionCapIsEnforcedAtTheBoundaryNotBeyondIt(t *testing.T) {
	if got := entity.ValidateMentionCount("mentions", entity.CommentMaxMentions); got.Code != "" {
		t.Fatalf("exactly %d mentions was refused with %q", entity.CommentMaxMentions, got.Code)
	}

	got := entity.ValidateMentionCount("mentions", entity.CommentMaxMentions+1)
	if got.Code != entity.ValidationCodeOutOfRange {
		t.Fatalf(
			"%d mentions was accepted with %q. Nothing in this repository rate-limits an "+
				"authenticated write, so the fan-out of one comment is bounded here or nowhere.",
			entity.CommentMaxMentions+1, got.Code,
		)
	}
}

func TestBothCommentActivityKindsAreRecognised(t *testing.T) {
	for _, kind := range []entity.ActivityKind{
		entity.ActivityKindCommented,
		entity.ActivityKindCommentDeleted,
	} {
		if !kind.Valid() {
			t.Fatalf(
				"%q is written by the comment service but is not a recognised activity kind, so "+
					"the database CHECK will refuse it",
				kind,
			)
		}
	}
}
