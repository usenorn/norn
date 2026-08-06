package linear_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

const (
	screenshot = "\x89PNG\r\n\x1a\n a screenshot of the failure"

	theLinkRow = `{"id":"` + pullRequestRow + `","title":"Fix the decoder",
		"subtitle":"usenorn/norn#812","url":"` + pullRequest + `",
		"createdAt":"2026-01-11T09:00:00.000Z","updatedAt":"2026-01-11T09:00:00.000Z"}`

	theFileRow = `{"id":"` + screenshotRow + `","title":"the failure.png",
		"subtitle":"","url":"` + shotSigned + `",
		"createdAt":"2026-01-11T09:02:00.000Z","updatedAt":"2026-01-11T09:02:00.000Z"}`
)

func oneIssueHolding(description, attachments, comments string) map[string]string {
	return map[string]string{
		"ImportAttachments": `{"issues":{"nodes":[
			{"id":"` + hubIssue + `","description":"` + description + `",
			 "createdAt":"2026-01-06T09:00:00.000Z","updatedAt":"2026-01-18T16:30:00.000Z",
			 "attachments":{"nodes":[` + attachments + `]},
			 "comments":{"nodes":[` + comments + `]}}
		],"pageInfo":{"hasNextPage":false,"endCursor":"cursor-attachments"}}}`,
	}
}

func servingTheScreenshot(reads *atomic.Int64) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		reads.Add(1)

		writer.Header().Set("Content-Type", "image/png")
		_, _ = writer.Write([]byte(screenshot))
	}
}

func attachmentOf(
	t *testing.T,
	page service.ImportFetchPage,
	externalID string,
) service.ImportAttachmentPayload {
	t.Helper()

	return payloadOf[service.ImportAttachmentPayload](t, recordNamed(t, page, externalID))
}

func storedUnderTheRun(t *testing.T, key string) {
	t.Helper()

	prefix := entity.ImportBlobPrefix(stagingWorkspace) + "/" + stagingRun.String() + "/"

	if !strings.HasPrefix(key, prefix) {
		t.Fatalf(
			"the file was written to %q, which is outside %q. Objects are swept by workspace prefix "+
				"when a workspace is purged and by run when an import is reverted: a key written "+
				"anywhere else survives both and belongs to nobody.",
			key, prefix,
		)
	}
}

func TestAnAttachmentThatIsOnlyALinkIsReportedAndNeverFetched(t *testing.T) {
	var reads atomic.Int64

	held := standing(t).
		answering(oneIssueHolding("", theLinkRow, "")).
		holding(servingTheScreenshot(&reads))

	page := fetched(t, staging(), held.source(), asking(entity.ImportAttachment))
	link := attachmentOf(t, page, pullRequestRow)

	if link.SourceURL != pullRequest {
		t.Fatalf(
			"the link arrived pointing at %q, want the pull request it came from. The report is "+
				"where somebody goes to re-link these by hand, and a row that lost its address "+
				"names nothing they can act on.",
			link.SourceURL,
		)
	}

	if link.ObjectKey != "" {
		t.Errorf(
			"the link claims stored object %q. Nothing was stored there, so the apply pass adopts a "+
				"key pointing at nothing and the attachment it creates is broken from the start.",
			link.ObjectKey,
		)
	}

	if stored := held.blobs.objects(); len(stored) != 0 {
		t.Fatalf(
			"the object store was written to %d times for a row that is a link to somewhere else. "+
				"Most of what Linear calls an attachment is a pull request, an alert or a "+
				"conversation: pulling those would put an HTML page in blob storage for every one "+
				"of them and charge the workspace for the space.",
			len(stored),
		)
	}

	if reads.Load() != 0 {
		t.Errorf(
			"the adapter made %d requests for a link it was never going to store. The URL belongs "+
				"to somebody else's service, and a staging pass that walks them is a crawler nobody "+
				"asked for.",
			reads.Load(),
		)
	}
}

func TestAFileIsPulledOnceAndStoredUnderTheRunThatAskedForIt(t *testing.T) {
	var reads atomic.Int64

	held := standing(t).
		answering(oneIssueHolding("", theFileRow, "")).
		holding(servingTheScreenshot(&reads))

	page := fetched(t, staging(), held.source(), asking(entity.ImportAttachment))
	file := attachmentOf(t, page, screenshotRow)

	stored := held.blobs.objects()

	if len(stored) != 1 {
		t.Fatalf(
			"the file was stored %d times. Execute runs after somebody has read a preview and "+
				"decided, which is hours or days after the source's signed URL stopped answering: a "+
				"file not pulled while staging reads it is a file this workspace never gets.",
			len(stored),
		)
	}

	storedUnderTheRun(t, stored[0].Key)

	if file.ObjectKey != stored[0].Key {
		t.Fatalf(
			"the row names object %q and the bytes went to %q. The apply pass adopts the object the "+
				"payload names and fetches nothing itself, so the two disagreeing leaves an "+
				"attachment pointing at nothing and an orphan nobody sweeps.",
			file.ObjectKey, stored[0].Key,
		)
	}

	if string(stored[0].Bytes) != screenshot {
		t.Errorf("the stored object holds %q, want the bytes the source served", stored[0].Bytes)
	}

	if stored[0].Size != int64(len(screenshot)) || file.SizeBytes != int64(len(screenshot)) {
		t.Errorf(
			"the object was written declaring %d bytes and the row claims %d, want %d. The store is "+
				"told the length up front and the workspace is charged on what the row claims, so a "+
				"wrong one either fails the write or bills for a file nobody uploaded.",
			stored[0].Size, file.SizeBytes, len(screenshot),
		)
	}

	if stored[0].ContentType != "image/png" || file.ContentType != "image/png" {
		t.Errorf(
			"the file was stored as %q and staged as %q, want the type the source served it with. "+
				"An attachment served back under the wrong type downloads instead of rendering, and "+
				"an image nobody can see in the issue is the whole reason for carrying it.",
			stored[0].ContentType, file.ContentType,
		)
	}

	if file.FileName != "the failure.png" {
		t.Errorf(
			"the file arrived named %q, want the name the source gave it. That name is what the "+
				"attachment is called from here on, so a raw path or a percent-escaped one is what "+
				"somebody reads in the issue forever.",
			file.FileName,
		)
	}

	if reads.Load() != 1 {
		t.Errorf(
			"the adapter pulled the file %d times. Every read of a signed URL spends a request "+
				"against a rate limit the whole run shares.",
			reads.Load(),
		)
	}
}

func TestAFileTheSourceWillNotHandOverLeavesTheRowAloneRatherThanFailingThePhase(t *testing.T) {
	var reads atomic.Int64

	held := standing(t).answering(oneIssueHolding("", theFileRow, "")).holding(
		func(writer http.ResponseWriter, _ *http.Request) {
			reads.Add(1)

			writer.WriteHeader(http.StatusForbidden)
		},
	)

	page, err := held.source().Fetch(staging(), asking(entity.ImportAttachment))
	if err != nil {
		t.Fatalf(
			"one file the source would not hand over ended the whole attachment phase: %v. Rows are "+
				"unbounded and a signed URL expires on its own schedule; one screenshot says nothing "+
				"about the thousands of rows behind it.",
			err,
		)
	}

	file := attachmentOf(t, page, screenshotRow)

	if file.ObjectKey != "" {
		t.Fatalf(
			"the row that could not be pulled still names object %q. Nothing was written there, so "+
				"the apply pass adopts a key that resolves to nothing.",
			file.ObjectKey,
		)
	}

	if file.SourceURL == "" {
		t.Errorf(
			"the row that could not be pulled lost its source address as well. It is the only thing " +
				"left saying where the file was, and somebody has to go and get it by hand.",
		)
	}

	if stored := held.blobs.objects(); len(stored) != 0 {
		t.Errorf("%d objects were written for a file the source refused to serve", len(stored))
	}

	if reads.Load() == 0 {
		t.Fatalf("the adapter never asked the source for the file, so nothing here was exercised")
	}
}

func TestAFileLargerThanThisInstanceStoresIsRefusedWithoutBeingReadWhole(t *testing.T) {
	const (
		stores = 4 << 10
		offer  = 64 << 20
		chunk  = 64 << 10
	)

	var written atomic.Int64

	held := standing(t).answering(oneIssueHolding("", theFileRow, "")).capping(stores).holding(
		func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "image/png")

			block := make([]byte, chunk)

			for sent := 0; sent < offer; sent += chunk {
				written.Add(int64(chunk))

				if _, err := writer.Write(block); err != nil {
					return
				}
			}
		},
	)

	page, err := held.source().Fetch(staging(), asking(entity.ImportAttachment))
	if err != nil {
		t.Fatalf(
			"one oversized file ended the whole attachment phase: %v. A file this instance cannot "+
				"take is worth exactly itself, and the rest of the page is still worth staging.",
			err,
		)
	}

	if file := attachmentOf(t, page, screenshotRow); file.ObjectKey != "" {
		t.Fatalf(
			"a file past the %d byte cap was stored anyway, as %q. The cap is what keeps one import "+
				"from filling the disk every other workspace on this instance shares.",
			stores, file.ObjectKey,
		)
	}

	if stored := held.blobs.objects(); len(stored) != 0 {
		t.Fatalf("%d oversized objects reached the store", len(stored))
	}

	if written.Load() == 0 {
		t.Fatalf("the adapter never read a byte of the file, so nothing here reached the cap")
	}

	if written.Load() >= offer {
		t.Fatalf(
			"the adapter took every one of the %d bytes the source offered against a %d byte cap. A "+
				"staging pass is already holding a page of rows in memory; a file measured only after "+
				"it has been read whole takes the worker down with it, and the run's attempts with it.",
			written.Load(), stores,
		)
	}
}

func TestAStagingPassThatDoesNotSayWhichRunItBelongsToStopsBeforeItWritesAnything(t *testing.T) {
	held := standing(t).answering(oneIssueHolding("", theFileRow, ""))

	_, err := held.source().Fetch(context.Background(), asking(entity.ImportAttachment))

	var unavailable entity.ImportSourceUnavailableError

	if !errors.As(err, &unavailable) {
		t.Fatalf(
			"the attachment phase returned %v with no run in the context, want it to stop. An "+
				"object is addressed by workspace and run: without those two a file has nowhere to "+
				"go that a revert or a workspace purge would ever find again.",
			err,
		)
	}

	if !strings.Contains(unavailable.Reason, "run") {
		t.Errorf(
			"the phase stopped saying %q, which never names the run as the thing missing. Whoever "+
				"reads this is looking at a job that queued itself, so the reason has to point at "+
				"the payload rather than at the source.",
			unavailable.Reason,
		)
	}

	if len(held.seen()) != 0 {
		t.Errorf("the source was read for files that could not have been stored anywhere")
	}

	if stored := held.blobs.objects(); len(stored) != 0 {
		t.Errorf(
			"%d objects were written by a pass that did not know which run it belonged to",
			len(stored),
		)
	}
}
