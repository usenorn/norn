package webhook_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
)

func packageSource(t *testing.T) string {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading this package: %v", err)
	}

	var combined strings.Builder

	for _, entry := range entries {
		name := entry.Name()

		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		body, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}

		combined.Write(body)
	}

	if combined.Len() == 0 {
		t.Fatal("this guard found no source, so it is protecting nothing")
	}

	return combined.String()
}

func (h *harness) drains(deliveries, outbox []int) (*[]time.Time, *[]time.Time) {
	deliveryCutoffs, outboxCutoffs := new([]time.Time), new([]time.Time)

	drain := func(batches []int, seen *[]time.Time) func(context.Context, time.Time, int) (int, error) {
		return func(_ context.Context, cutoff time.Time, _ int) (int, error) {
			*seen = append(*seen, cutoff)

			if len(*seen) > len(batches) {
				return 0, nil
			}

			return batches[len(*seen)-1], nil
		}
	}

	h.retention.EXPECT().
		DropDeliveriesBefore(gomock.Any(), gomock.Any(), sweepBatch).
		DoAndReturn(drain(deliveries, deliveryCutoffs)).
		AnyTimes()

	h.retention.EXPECT().
		DropOutboxBefore(gomock.Any(), gomock.Any(), sweepBatch).
		DoAndReturn(drain(outbox, outboxCutoffs)).
		AnyTimes()

	return deliveryCutoffs, outboxCutoffs
}

func TestTheSweepKeepsDrainingUntilABatchComesUpShort(t *testing.T) {
	h := newHarness(t)

	deliveryCutoffs, outboxCutoffs := h.drains(
		[]int{sweepBatch, sweepBatch, 11},
		[]int{sweepBatch, 4},
	)

	dropped, err := h.sweeper.Sweep(context.Background())
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	if len(*deliveryCutoffs) != 3 {
		t.Fatalf(
			"the sweep made %d passes over the deliveries, want 3. It stops at the first batch "+
				"smaller than the limit; stopping after one full batch would leave history that "+
				"accumulated faster than a single run can clear it, for ever.",
			len(*deliveryCutoffs),
		)
	}

	if len(*outboxCutoffs) != 2 {
		t.Fatalf(
			"the sweep made %d passes over the outbox, want 2. Every event that ever fanned out "+
				"leaves a row there, so the outbox is the table that grows fastest.",
			len(*outboxCutoffs),
		)
	}

	if want := 2*sweepBatch + 11 + sweepBatch + 4; dropped != want {
		t.Errorf(
			"the sweep reported %d rows removed but dropped %d, so an operator watching the number "+
				"cannot tell whether retention is keeping up",
			dropped, want,
		)
	}
}

func TestTheSweepRemovesBothTheDeliveriesAndTheEventsThatCausedThem(t *testing.T) {
	h := newHarness(t)

	deliveryCutoffs, outboxCutoffs := h.drains([]int{2}, []int{1})

	if _, err := h.sweeper.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	if len(*deliveryCutoffs) == 0 || len(*outboxCutoffs) == 0 {
		t.Fatalf(
			"the sweep touched deliveries %d times and the outbox %d times. Both hold the bodies "+
				"of events, so retention that clears only one of them keeps the payloads it "+
				"promised to age out.",
			len(*deliveryCutoffs), len(*outboxCutoffs),
		)
	}

	age := time.Since((*deliveryCutoffs)[0])
	window := webhookConfig().Retention

	if age < window-time.Minute || age > window+time.Minute {
		t.Errorf(
			"the sweep cut off at %s ago but retention is configured at %s. A cutoff that drifts "+
				"from the configured window removes history an operator was told would be kept.",
			age, window,
		)
	}

	if !(*outboxCutoffs)[0].Equal((*deliveryCutoffs)[0]) {
		t.Error(
			"the outbox and the deliveries were aged out at different cutoffs, so a delivery can " +
				"outlive the event it came from and the log stops joining up",
		)
	}
}

func TestWebhookRetentionIsNotGatedOnALicence(t *testing.T) {
	source := packageSource(t)

	for _, gate := range []string{"Licensing", "Licence", "Permits(", "entity.Feature"} {
		if strings.Contains(source, gate) {
			t.Errorf(
				"this package mentions %q. Webhooks are how a self-hosted instance gets its data "+
					"out, and gating them behind a licence would make an expiry silently stop "+
					"every integration a workspace depends on.",
				gate,
			)
		}
	}
}
