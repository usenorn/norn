package linear_test

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestAStoredFileIsChargedToTheWorkspaceItWasPulledFor(t *testing.T) {
	var reads atomic.Int64

	held := standing(t).
		answering(oneIssueHolding("", theFileRow, "")).
		holding(servingTheScreenshot(&reads))

	page := fetched(t, staging(), held.source(), asking(entity.ImportAttachment))
	file := attachmentOf(t, page, screenshotRow)

	if charged := held.storage.held()[file.ObjectKey]; charged != int64(len(screenshot)) {
		t.Fatalf(
			"a stored file of %d bytes was charged %d. An import's files sit in the workspace's "+
				"storage from the moment they are pulled, so they count against its limit from then.",
			len(screenshot), charged,
		)
	}
}

func TestAFileIntoAFullWorkspaceIsLeftNamedRatherThanStored(t *testing.T) {
	var reads atomic.Int64

	held := standing(t).
		answering(oneIssueHolding("", theFileRow, "")).
		holding(servingTheScreenshot(&reads))

	held.storage.full = true

	page, err := held.source().Fetch(staging(), asking(entity.ImportAttachment))
	if err != nil {
		t.Fatalf(
			"a full workspace ended the whole attachment phase: %v. The issues and comments behind "+
				"the file still fit, and the report is where a file left behind is named.",
			err,
		)
	}

	file := attachmentOf(t, page, screenshotRow)

	if file.ObjectKey != "" || file.SourceURL == "" {
		t.Fatalf(
			"the file a full workspace refused names object %q and source %q. It must name no "+
				"object, since nothing was written, and keep where it came from.",
			file.ObjectKey, file.SourceURL,
		)
	}

	if stored := held.blobs.objects(); len(stored) != 0 {
		t.Fatalf("%d objects were written into a workspace with no room for them", len(stored))
	}
}

func refusing(store *blobStore) {
	store.mu.Lock()
	defer store.mu.Unlock()

	store.refuse = errors.New("storage is unreachable")
}

func TestAFirstFileTheStoreRefusesGivesBackWhatItWasCharged(t *testing.T) {
	var reads atomic.Int64

	held := standing(t).
		answering(oneIssueHolding("", theFileRow, "")).
		holding(servingTheScreenshot(&reads))

	refusing(held.blobs)

	page, err := held.source().Fetch(staging(), asking(entity.ImportAttachment))
	if err != nil {
		t.Fatalf("a store that refused one file ended the attachment phase: %v", err)
	}

	if file := attachmentOf(t, page, screenshotRow); file.ObjectKey != "" {
		t.Fatalf("the file the store refused still names object %q", file.ObjectKey)
	}

	if len(held.storage.putBack()) != 1 || len(held.storage.held()) != 0 {
		t.Fatalf(
			"the refused file left %v charged after %d restores. Bytes that never reached storage "+
				"must not stay on the workspace's bill.",
			held.storage.held(), len(held.storage.putBack()),
		)
	}
}

func TestAFileStagedAgainThatTheStoreRefusesKeepsItsEarlierCharge(t *testing.T) {
	var reads atomic.Int64

	held := standing(t).
		answering(oneIssueHolding("", theFileRow, "")).
		holding(servingTheScreenshot(&reads))

	first := attachmentOf(t, fetched(t, staging(), held.source(), asking(entity.ImportAttachment)), screenshotRow)

	refusing(held.blobs)

	page, err := held.source().Fetch(staging(), asking(entity.ImportAttachment))
	if err != nil {
		t.Fatalf("a store that refused the second pull ended the attachment phase: %v", err)
	}

	if again := attachmentOf(t, page, screenshotRow); again.ObjectKey != "" {
		t.Fatalf("the pull the store refused still names object %q", again.ObjectKey)
	}

	if charged := held.storage.held()[first.ObjectKey]; charged != int64(len(screenshot)) {
		t.Fatalf(
			"after a second pull of the same file failed, %q is charged %d bytes, want the %d the "+
				"first pull stored. The earlier object is still in storage under that key, so its "+
				"charge has to survive the failed replacement.",
			first.ObjectKey, charged, len(screenshot),
		)
	}
}
