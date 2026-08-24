package preview_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func TestAPreviewExistsOnlyBecauseTheMachineHoldingTheRunSaidSo(t *testing.T) {
	h := newHarness(t)
	h.holds()

	stranger := entity.Runner{ID: uuid.New(), AgentID: uuid.New()}

	err := h.service.Reported(
		context.Background(), stranger, previewMessage("web", "web", h.portFor("web"), channelv1.PreviewOpen),
	)

	refusedWith(t, err, entity.ErrExecutionNotFound)

	if len(h.stored) != 0 {
		t.Fatal(
			"a machine that does not hold this run registered a preview against it. Every " +
				"routable address would then be one any enrolled machine could claim",
		)
	}
}

func TestAReportedPreviewTakesTheAddressTheGatewayWillRouteBy(t *testing.T) {
	h := newHarness(t)
	h.holds()

	preview := h.reported(t, "web", channelv1.PreviewOpen)

	if preview.Host != h.hostFor("web") {
		t.Fatalf(
			"the preview took the host %q, want %q. The runner only knows a loopback address, "+
				"so the shared one has to be composed here or it is composed nowhere",
			preview.Host, h.hostFor("web"),
		)
	}

	if preview.URL("https") != "https://"+h.hostFor("web") {
		t.Fatalf("the preview answered with %q", preview.URL("https"))
	}
}

func TestAServerServingNoPreviewDomainRecordsThePreviewWithoutInventingAnAddress(t *testing.T) {
	h := newHarnessServing(t, "")
	h.holds()

	preview := h.reported(t, "web", channelv1.PreviewOpen)

	if preview.Host != "" || preview.URL("https") != "" {
		t.Fatalf(
			"a server with no preview domain composed the address %q. Nothing resolves it, and "+
				"handing somebody a dead link is worse than saying there is none",
			preview.URL("https"),
		)
	}

	if len(h.previewEvents()) != 1 {
		t.Fatal("the preview did not reach the run's timeline")
	}
}

func TestOpeningAPreviewPutsItOnTheRunsTimelineWithTheAddressPeopleCanOpen(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.reported(t, "web", channelv1.PreviewOpen)

	seen := h.previewEvents()
	if len(seen) != 1 {
		t.Fatalf("the timeline carries %d preview lines, want 1", len(seen))
	}

	if seen[0].Reason == "" {
		t.Fatal("the preview line says nothing, so the timeline shows a blank row")
	}

	if seen[0].Actor.RunnerID != h.runner.ID {
		t.Fatal("the preview line is not attributed to the machine that opened it")
	}
}

func TestAPreviewReportReplayedAfterAReconnectAddsNothingASecondTime(t *testing.T) {
	h := newHarness(t)
	h.holds()

	message := previewMessage("web", "web", h.portFor("web"), channelv1.PreviewOpen)

	for range 2 {
		if err := h.service.Reported(context.Background(), h.runner, message); err != nil {
			t.Fatalf("report the preview: %v", err)
		}
	}

	if seen := h.previewEvents(); len(seen) != 1 {
		t.Fatalf(
			"a replayed report put %d lines on the timeline, want 1. Delivery is at-least-once, "+
				"so a reconnect would double every line a run ever reported",
			len(seen),
		)
	}
}

func TestOnePortOfARunKeepsOneAddressHoweverThePreviewIsRenamed(t *testing.T) {
	h := newHarness(t)
	h.holds()

	opened := h.reported(t, "web", channelv1.PreviewOpen)

	if err := h.service.Reported(
		context.Background(), h.runner,
		previewMessage("issue-description", "web", h.portFor("web"), channelv1.PreviewOpen),
	); err != nil {
		t.Fatalf("report the same port under another name: %v", err)
	}

	renamed := h.stored[h.portFor("web")]

	if renamed.Host != opened.Host {
		t.Fatalf(
			"the address moved from %q to %q because the machine called the preview something "+
				"else. A link already shared outside the workspace then points at a host that "+
				"no longer exists, and nothing says why",
			opened.Host, renamed.Host,
		)
	}

	if renamed.Name != "issue-description" {
		t.Fatalf(
			"the preview is still called %q on the run screen. The name is the machine's to "+
				"choose and is what a person reads",
			renamed.Name,
		)
	}
}

func TestTwoServicesOfOneRunAreTwoAddressesRatherThanOne(t *testing.T) {
	h := newHarness(t)
	h.holds()

	web := h.reported(t, "web", channelv1.PreviewOpen)
	docs := h.reported(t, "docs", channelv1.PreviewOpen)

	if web.Host == docs.Host {
		t.Fatalf(
			"both services answer on %q. A person following a link reaches whichever of them "+
				"the gateway picked",
			web.Host,
		)
	}

	if len(h.stored) != 2 {
		t.Fatalf("the run holds %d previews on record, want 2", len(h.stored))
	}
}

func TestAPreviewCarriesTheNameTheMachineGaveItOntoTheTimeline(t *testing.T) {
	h := newHarness(t)
	h.holds()

	h.reported(t, "issue-description", channelv1.PreviewOpen)

	seen := h.previewEvents()
	if len(seen) != 1 {
		t.Fatalf("the run's timeline holds %d preview lines, want 1", len(seen))
	}

	if !strings.Contains(seen[0].Reason, "issue-description") {
		t.Fatalf(
			"the timeline says %q and never uses the name the machine gave the preview. The "+
				"address is norn's, but the name is what a person reads",
			seen[0].Reason,
		)
	}
}

func TestAPreviewNamedSomethingThatCouldNotBeAHostIsRefused(t *testing.T) {
	h := newHarness(t)
	h.holds()

	err := h.service.Reported(
		context.Background(), h.runner, previewMessage("Web App", "web", h.portFor("web"), channelv1.PreviewOpen),
	)
	if err == nil {
		t.Fatal(
			"a preview name that is not a hostname label was accepted. It becomes a subdomain, " +
				"so the address it produces resolves for nobody",
		)
	}
}

func TestClosingAPreviewLeavesItOnRecordAndStopsItResolving(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.reported(t, "web", channelv1.PreviewOpen)

	closed := h.reported(t, "web", channelv1.PreviewClosed)

	if closed.Open() {
		t.Fatal("a preview the machine closed is still open")
	}

	if closed.ClosedAt.IsZero() {
		t.Fatal("a closed preview carries no time it closed, so the timeline cannot say when")
	}
}

func TestARunNeverHoldsMorePreviewsThanTheMachineWouldGiveIt(t *testing.T) {
	h := newHarness(t)
	h.holds()

	for index := range entity.PreviewsMax {
		h.reported(t, "app"+string(rune('a'+index)), channelv1.PreviewOpen)
	}

	err := h.service.Reported(
		context.Background(), h.runner, previewMessage("extra", "extra", h.portFor("extra"), channelv1.PreviewOpen),
	)

	refusedWith(t, err, entity.ErrPreviewCrowded)
}
