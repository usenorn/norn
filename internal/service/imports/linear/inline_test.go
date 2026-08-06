package linear_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

const (
	chartAddress = "https://uploads.linear.app/a1/b2/chart.png"
	chartSigned  = chartAddress + "?signature=zyxwvu"

	hubDescription = `The parser drops a frame.\n\n![the failure](` + shotSigned + `)\n`
	commentBody    = `Here is the shape of it: ![the chart](` + chartSigned + `)`

	theCommentRow = `{"id":"` + firstComment + `","body":"` + commentBody + `",
		"createdAt":"2026-01-10T09:00:00.000Z","updatedAt":"2026-01-10T09:00:00.000Z"}`
)

func TestAnImageInsideABodyBecomesAMarkerAndAFileRecordThatAgreeOnTheSameImage(t *testing.T) {
	held := standing(t).answering(wholeWorkspace())
	source := held.source()

	issues := fetched(t, staging(), source, asking(entity.ImportIssue))
	described := payloadOf[service.ImportIssuePayload](t, recordNamed(t, issues, hubIssue)).Description

	if strings.Contains(described, "signature=") {
		t.Fatalf(
			"the description still points at the signed URL:\n%s\nIt renders today and shows a hole "+
				"the moment the signature expires, which is long after anybody is still looking at "+
				"this import.",
			described,
		)
	}

	if !entity.ImportEmbedded(described) {
		t.Fatalf(
			"the description holds no embed marker:\n%s\nThe embed phase rewrites markers into "+
				"attachment references and nothing else, so an image left addressed any other way "+
				"is never re-pointed at the file this run stored.",
			described,
		)
	}

	files := fetched(t, staging(), source, asking(entity.ImportAttachment))
	inline := recordNamed(t, files, shotAddress+"#"+hubIssue)
	payload := payloadOf[service.ImportAttachmentPayload](t, inline)

	if !payload.Inline {
		t.Fatalf(
			"the image the description points at is staged as an ordinary attachment. Inline is " +
				"what the staging pass derives an embed record from, so without it the picture " +
				"arrives hanging off the issue and the body still names nothing.",
		)
	}

	if payload.Issue != hubIssue || payload.Comment != "" {
		t.Errorf(
			"the image is filed against issue %q and comment %q, want the issue whose description "+
				"holds it and no comment.",
			payload.Issue, payload.Comment,
		)
	}

	marker := entity.ImportEmbedMarker(inline.ExternalID)

	if !strings.Contains(described, marker) {
		t.Fatalf(
			"the body names %s and the file record is addressed by %q. The two are derived from one "+
				"read for exactly this reason: a marker naming anything the record is not leaves the "+
				"embed phase with a reference it cannot resolve and a picture that never comes back.",
			described, inline.ExternalID,
		)
	}

	if want := "The parser drops a frame.\n\n![the failure](" + marker + ")\n"; described != want {
		t.Fatalf(
			"the description arrived as\n%q\nwant\n%q\nOnly the address inside the image changes; "+
				"the alt text and everything around it is somebody's writing.",
			described, want,
		)
	}
}

func TestAnImageInsideACommentIsFiledAgainstTheCommentThatHoldsIt(t *testing.T) {
	held := standing(t).answering(oneIssueHolding("", "", theCommentRow))

	page := fetched(t, staging(), held.source(), asking(entity.ImportAttachment))
	payload := attachmentOf(t, page, chartAddress+"#"+firstComment)

	if payload.Issue != hubIssue || payload.Comment != firstComment {
		t.Fatalf(
			"the comment's image is filed against issue %q and comment %q, want %q and %q. The "+
				"embed phase rewrites the body that holds the marker, and it finds that body through "+
				"the comment named here.",
			payload.Issue, payload.Comment, hubIssue, firstComment,
		)
	}

	if !payload.Inline {
		t.Errorf("the comment's image is staged as an ordinary attachment rather than an inline one")
	}
}

func TestABodyHoldingNoUploadIsCarriedAcrossExactlyAsItWasWritten(t *testing.T) {
	held := standing(t).answering(wholeWorkspace())

	page := fetched(t, staging(), held.source(), asking(entity.ImportIssue))
	described := payloadOf[service.ImportIssuePayload](t, recordNamed(t, page, looseIssue)).Description

	want := "See the [runbook](https://example.com/runbook) and ![a diagram](https://example.com/a.png)."

	if described != want {
		t.Fatalf(
			"a description holding nothing this import stores came across as\n%q\nwant\n%q\nAn "+
				"image hosted somewhere else keeps working on its own and this import has no copy "+
				"of it to point at; rewriting it would replace a picture that renders with a "+
				"reference to nothing.",
			described, want,
		)
	}

	files := fetched(t, staging(), held.source(), asking(entity.ImportAttachment))

	for _, record := range files.Records {
		if strings.Contains(record.ExternalID, "example.com") {
			t.Errorf(
				"an image hosted outside the source was staged as a file to store, as %q. Pulling "+
					"every URL a body mentions makes a staging pass into a crawler over whatever "+
					"somebody linked.",
				record.ExternalID,
			)
		}
	}
}

const (
	pastedOne = "https://uploads.linear.app/aaaa-1111/image.png"
	pastedTwo = "https://uploads.linear.app/bbbb-2222/image.png"

	twoScreenshots = `First: ![one](` + pastedOne + `) and second: ![two](` + pastedTwo + `)`
)

func TestTwoScreenshotsPastedUnderTheSameNameAreStoredApart(t *testing.T) {
	held := standing(t).
		answering(oneIssueHolding(twoScreenshots, "", "")).
		holding(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte("bytes of " + request.URL.Path))
		})

	page := fetched(t, staging(), held.source(), asking(entity.ImportAttachment))

	keys := make(map[string]string)

	for _, record := range page.Records {
		payload := payloadOf[service.ImportAttachmentPayload](t, record)

		if payload.ObjectKey == "" {
			continue
		}

		if earlier, clash := keys[payload.ObjectKey]; clash {
			t.Fatalf(
				"%s and %s are both stored at %q. Only the last segment of an upload address is "+
					"the file name, and \"image.png\" is what Linear calls every pasted screenshot: "+
					"naming the object after that alone puts every screenshot in the workspace at "+
					"one key, so the second import overwrites the first with different bytes and "+
					"one issue quietly ends up illustrated with another issue's picture.",
				earlier, payload.SourceURL, payload.ObjectKey,
			)
		}

		keys[payload.ObjectKey] = payload.SourceURL
	}

	if len(keys) != 2 {
		t.Fatalf("stored %d objects for two distinct uploads, want 2", len(keys))
	}
}

func TestOneImagePastedIntoAnIssueAndItsCommentIsStagedOncePerPlace(t *testing.T) {
	shared := `Look: ![it](` + chartSigned + `)`
	sharedComment := `{"id":"` + firstComment + `","body":"` + shared + `",
		"createdAt":"2026-01-10T09:00:00.000Z","updatedAt":"2026-01-10T09:00:00.000Z"}`

	held := standing(t).
		answering(oneIssueHolding(shared, "", sharedComment)).
		holding(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte("bytes of " + request.URL.Path))
		})

	page := fetched(t, staging(), held.source(), asking(entity.ImportAttachment))

	seen := make(map[string]bool)
	owners := make(map[string]string)

	for _, record := range page.Records {
		payload := payloadOf[service.ImportAttachmentPayload](t, record)

		if !payload.Inline {
			continue
		}

		if seen[record.ExternalID] {
			t.Fatalf(
				"two staged rows share the address %q while describing different places the same "+
					"image sits. Records are keyed by run, resource and external id, so the second "+
					"upsert replaces the first: one of the two markers then resolves to a record "+
					"naming the other body, and the picture is filed against the wrong row.",
				record.ExternalID,
			)
		}

		seen[record.ExternalID] = true
		owners[record.ExternalID] = payload.Comment
	}

	if len(seen) != 2 {
		t.Fatalf(
			"the same image sits in a description and in a comment on it, and %d rows were staged. "+
				"Each place it appears is its own attachment to make.",
			len(seen),
		)
	}

	described, commented := 0, 0

	for _, comment := range owners {
		if comment == "" {
			described++
		} else {
			commented++
		}
	}

	if described != 1 || commented != 1 {
		t.Errorf(
			"the two rows are filed as %d on the issue and %d on a comment, want one of each",
			described, commented,
		)
	}
}
