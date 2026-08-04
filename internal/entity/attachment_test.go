package entity_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/usenorn/norn/internal/entity"
)

func TestOnlyTheFourSniffableRastersAreEverServedInline(t *testing.T) {
	for _, contentType := range []string{"image/png", "image/jpeg", "image/gif", "image/webp"} {
		if got := entity.AttachmentServedType(contentType); got != contentType {
			t.Fatalf("%s is served as %q, so a screenshot would download instead of rendering", contentType, got)
		}
	}

	dangerous := map[string]string{
		"an svg":         "image/svg+xml",
		"a page":         "text/html; charset=utf-8",
		"a script":       "text/javascript",
		"a pdf":          "application/pdf",
		"an xml doc":     "text/xml",
		"a bare unknown": "application/x-msdownload",
	}

	for name, contentType := range dangerous {
		t.Run(name, func(t *testing.T) {
			if got := entity.AttachmentServedType(contentType); got != entity.AttachmentGenericType {
				t.Fatalf(
					"%s would be served as %q. An SVG carries script and a PDF runs a JavaScript "+
						"engine; anything the sniff cannot prove is a raster image downloads.",
					contentType, got,
				)
			}
		})
	}
}

func TestTheServedTypeIgnoresWhateverFollowsTheSemicolon(t *testing.T) {
	if got := entity.AttachmentServedType("image/png; charset=utf-8"); got != "image/png" {
		t.Fatalf(
			"a sniffed type with parameters became %q. http.DetectContentType appends charset to "+
				"text types, so the allow-list has to compare the media type alone.",
			got,
		)
	}
}

func TestTheDispositionFollowsTheServedTypeAndNothingElse(t *testing.T) {
	image := entity.Attachment{ContentType: "image/png"}
	if image.Disposition() != entity.AttachmentDispositionIn {
		t.Fatal("an allow-listed image would download rather than render")
	}

	svg := entity.Attachment{ContentType: "image/svg+xml"}
	if svg.Disposition() != entity.AttachmentDispositionAt {
		t.Fatal(
			"an SVG would be served inline. A browser will not run script from <img src>, but a " +
				"direct navigation to the URL will, and we do not control what a person pastes.",
		)
	}
}

func TestAFileNameIsStrippedOfAnyPathTheBrowserSent(t *testing.T) {
	for name, sent := range map[string]string{
		"a posix path":   "/etc/passwd",
		"a windows path": `C:\Users\rae\Desktop\shot.png`,
		"a traversal":    "../../../../etc/shadow",
	} {
		t.Run(name, func(t *testing.T) {
			got := entity.AttachmentFileName(sent)

			if strings.ContainsAny(got, "/\\") {
				t.Fatalf(
					"%q became %q, which still carries a path. The name is only ever a "+
						"Content-Disposition filename, never a key.",
					sent, got,
				)
			}
		})
	}
}

func TestANameIsRefusedWhenItCouldForgeAHeader(t *testing.T) {
	if got := entity.ValidateAttachmentName("fileName", "shot\r\nX-Evil: 1.png"); got.Code != entity.ValidationCodeMalformed {
		t.Fatalf(
			"a name carrying CRLF was accepted with %q. It lands in Content-Disposition, where a "+
				"newline splits the response.",
			got.Code,
		)
	}

	if got := entity.ValidateAttachmentName("fileName", "   "); got.Code != entity.ValidationCodeRequired {
		t.Fatalf("a blank name was accepted with %q", got.Code)
	}

	long := strings.Repeat("é", entity.AttachmentNameMaxLen)
	if got := entity.ValidateAttachmentName("fileName", long); got.Code != "" {
		t.Fatalf("%d two-byte runes were refused with %q — the cap is counted in bytes", entity.AttachmentNameMaxLen, got.Code)
	}

	if got := entity.ValidateAttachmentName("fileName", long+"é"); got.Code != entity.ValidationCodeTooLong {
		t.Fatalf("one rune over the cap was accepted with %q", got.Code)
	}
}

func TestABlobKeyCannotClimbOutOfItsRoot(t *testing.T) {
	workspace, attachment := uuid.New(), uuid.New()

	if key := entity.AttachmentKey(workspace, attachment); !entity.ValidBlobKey(key) {
		t.Fatalf("the keys we mint ourselves are refused: %q", key)
	}

	for name, key := range map[string]string{
		"empty":           "",
		"climbing":        "attachments/../../etc/passwd",
		"a bare dotdot":   "..",
		"absolute":        "/etc/passwd",
		"a backslash":     `attachments\..\..\etc`,
		"an empty middle": "attachments//passwd",
		"a nul byte":      "attachments/\x00",
		"a space":         "attachments/pass wd",
	} {
		t.Run(name, func(t *testing.T) {
			if entity.ValidBlobKey(key) {
				t.Fatalf(
					"%q was accepted. Keys are server-minted today, so this guard exists for the "+
						"day that stops being true.",
					key,
				)
			}
		})
	}
}

func TestAnUnlimitedWorkspaceReportsNoRemainingRatherThanANegativeOne(t *testing.T) {
	unlimited := entity.WorkspaceStorage{StoredBytes: 5_000_000}

	if !unlimited.Unlimited() {
		t.Fatal("a zero cap is the unlimited case and must read as one")
	}

	if unlimited.Remaining() != 0 {
		t.Fatalf("an unlimited workspace claims %d bytes remaining", unlimited.Remaining())
	}

	full := entity.WorkspaceStorage{StoredBytes: 120, MaxBytes: 100}
	if full.Remaining() != 0 {
		t.Fatalf("an over-full workspace reports %d remaining, which would render as a negative", full.Remaining())
	}

	room := entity.WorkspaceStorage{StoredBytes: 40, MaxBytes: 100}
	if room.Remaining() != 60 {
		t.Fatalf("remaining = %d, want 60", room.Remaining())
	}
}

func TestAnAttachmentWhoseIssueIsGoneReadsAsOrphaned(t *testing.T) {
	live := entity.Attachment{IssueID: uuid.New(), CommentID: uuid.New()}
	if live.Orphaned() {
		t.Fatal("an attachment on a live issue was reported as reclaimable")
	}

	uncommented := entity.Attachment{IssueID: uuid.New()}
	if uncommented.Orphaned() {
		t.Fatal(
			"an attachment whose comment was deleted was reported as reclaimable. The issue is " +
				"the authority; the comment is only a grouping.",
		)
	}

	orphan := entity.Attachment{CommentID: uuid.New()}
	if !orphan.Orphaned() {
		t.Fatal("an attachment whose issue was purged was not reported as reclaimable, so its bytes leak")
	}
}

func TestBothStorageErrorsCarryTheirNumbersAndTheirSentinel(t *testing.T) {
	oversized := entity.AttachmentTooLargeError{SizeBytes: 100, MaxBytes: 50}
	if !strings.Contains(oversized.Error(), "100") || !strings.Contains(oversized.Error(), "50") {
		t.Fatalf("the refusal reads %q without naming the sizes", oversized.Error())
	}

	exhausted := entity.StorageExhaustedError{SizeBytes: 10, StoredBytes: 99, MaxBytes: 100}
	if !strings.Contains(exhausted.Error(), "99") {
		t.Fatalf("the refusal reads %q without naming what is stored", exhausted.Error())
	}
}
