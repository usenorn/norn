package preview_test

import (
	"context"
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
		context.Background(), stranger, previewMessage("web", "web", channelv1.PreviewOpen),
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

	if preview.Host != hostFor("web") {
		t.Fatalf(
			"the preview took the host %q, want %q. The runner only knows a loopback address, "+
				"so the shared one has to be composed here or it is composed nowhere",
			preview.Host, hostFor("web"),
		)
	}

	if preview.URL("https") != "https://"+hostFor("web") {
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

	message := previewMessage("web", "web", channelv1.PreviewOpen)

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

func TestAPreviewNamedSomethingThatCouldNotBeAHostIsRefused(t *testing.T) {
	h := newHarness(t)
	h.holds()

	err := h.service.Reported(
		context.Background(), h.runner, previewMessage("Web App", "web", channelv1.PreviewOpen),
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
		context.Background(), h.runner, previewMessage("extra", "extra", channelv1.PreviewOpen),
	)

	refusedWith(t, err, entity.ErrPreviewCrowded)
}

func TestThePortDecidesTheAddressSoRenamingAPreviewMovesNobody(t *testing.T) {
	h := newHarness(t)
	h.holds()

	const port = 43000

	opened := h.reportedOn(t, "issue-description", port, channelv1.PreviewOpen)
	h.reportedOn(t, "issue-description", port, channelv1.PreviewClosed)
	reopened := h.reportedOn(t, "web", port, channelv1.PreviewOpen)

	if reopened.Host != opened.Host {
		t.Fatalf(
			"re-opening port %d under another name moved it from %q to %q. The address is the "+
				"run and the port, so a link somebody already sent on has to keep working",
			port, opened.Host, reopened.Host,
		)
	}

	if reopened.ID != opened.ID {
		t.Fatal(
			"re-opening the same port made a second preview. Every share link minted against " +
				"the first one is bound to a preview the address no longer answers as",
		)
	}

	if reopened.Name != "web" {
		t.Fatalf(
			"the preview is still called %q. The name the agent chose decides nothing about "+
				"the address, but it is what a person reads on the run",
			reopened.Name,
		)
	}
}

func TestTwoPortsOnOneRunAreTwoPreviewsAtTwoAddresses(t *testing.T) {
	h := newHarness(t)
	h.holds()

	web := h.reportedOn(t, "web", 43000, channelv1.PreviewOpen)
	api := h.reportedOn(t, "api", 5173, channelv1.PreviewOpen)

	if web.Host == api.Host {
		t.Fatalf(
			"both ports answer at %q, so whichever the gateway picks the other is unreachable",
			web.Host,
		)
	}

	if web.ID == api.ID {
		t.Fatal("two ports were recorded as one preview")
	}
}

func TestAPreviewWithoutAPortIsRefusedBecauseNoAddressFollowsFromIt(t *testing.T) {
	h := newHarness(t)
	h.holds()

	err := h.service.Reported(
		context.Background(), h.runner, previewOnPort("web", "web", 0, channelv1.PreviewOpen),
	)
	if err == nil {
		t.Fatal(
			"a preview carrying no port was accepted. The port is half of what the address is " +
				"derived from, so the run would take an address that names nothing",
		)
	}
}

func TestRenamingTheIssueMidRunDoesNotMoveAnAddressAlreadyHandedOut(t *testing.T) {
	h := newHarness(t)
	h.holds()

	const port = 43000

	opened := h.reportedOn(t, "web", port, channelv1.PreviewOpen)

	h.execution.IssueTitle = "Something else entirely"

	again := h.reportedOn(t, "web", port, channelv1.PreviewOpen)

	if again.Host != opened.Host {
		t.Fatalf(
			"editing the issue title moved the preview from %q to %q. The machine still names "+
				"the address it was given at the start, so the two would stop agreeing and "+
				"every link already shared would resolve nowhere",
			opened.Host, again.Host,
		)
	}
}
