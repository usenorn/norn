package scm_test

import (
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

func TestALateDeliveryDoesNotUndoALaterOne(t *testing.T) {
	early := time.Date(2026, 8, 7, 10, 0, 0, 0, time.UTC)
	late := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)

	merged := entity.CodeLink{State: entity.CodeChangeMerged, SourceUpdatedAt: &late}

	if !merged.Supersedes(&early) {
		t.Fatal(
			"a stored merge did not supersede an older event. Deliveries arrive out of order, " +
				"and without this a reopen sent before the merge would settle the change as open",
		)
	}

	if merged.Supersedes(&late) != true {
		t.Error("an event of the same age is applied, so a redelivery refreshes rather than skips")
	}

	newer := time.Date(2026, 8, 7, 13, 0, 0, 0, time.UTC)

	if merged.Supersedes(&newer) {
		t.Error("a genuinely newer event must be applied over what is stored")
	}
}

func TestAChangeWithNoRecordedTimeAcceptsWhateverArrives(t *testing.T) {
	stored := entity.CodeLink{State: entity.CodeChangeOpen}
	at := time.Date(2026, 8, 7, 10, 0, 0, 0, time.UTC)

	if stored.Supersedes(&at) {
		t.Fatal(
			"a link with no recorded time claimed to be newer than an event. A row written " +
				"before timestamps were recorded would then never update again",
		)
	}

	if !stored.Supersedes(nil) {
		t.Error("an event carrying no time cannot be ordered and must not overwrite what is stored")
	}
}
