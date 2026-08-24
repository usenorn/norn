package channelv1_test

import (
	"strconv"
	"strings"
	"testing"

	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

const (
	testExecutionID = "exec-01M0SMJXBJ451KZ0MCQ6TY2GH1"
	testDomain      = "norn.ink"
	testTitle       = "A preview address must be one per task, execution and port"
)

func TestAPreviewHostIsTheIssueTheRunAndThePortAndNothingElse(t *testing.T) {
	host := channelv1.PreviewHost(
		"NORN-75", testTitle, testExecutionID, 43000, channelv1.PreviewBySubdomain, testDomain,
	)

	want := "norn-75-a-preview-address-exec-01m0smjxbj451kz0mcq6ty2gh1-43000.norn.ink"
	if host != want {
		t.Fatalf(
			"the host came out as %q, want %q. The address is norn's to derive from the issue, "+
				"the run and the port; anything else in it is somebody else's choice reaching DNS",
			host, want,
		)
	}
}

func TestTwoPortsOfOneRunAreTwoAddressesAndOnePortIsAlwaysTheSame(t *testing.T) {
	first := channelv1.PreviewHost(
		"NORN-75", testTitle, testExecutionID, 43000, channelv1.PreviewBySubdomain, testDomain,
	)
	second := channelv1.PreviewHost(
		"NORN-75", testTitle, testExecutionID, 43001, channelv1.PreviewBySubdomain, testDomain,
	)

	if first == second {
		t.Fatalf(
			"two services of one run answer on %q. A person following a link would reach "+
				"whichever of them the gateway picked",
			first,
		)
	}

	again := channelv1.PreviewHost(
		"NORN-75", testTitle, testExecutionID, 43000, channelv1.PreviewBySubdomain, testDomain,
	)
	if again != first {
		t.Fatalf(
			"the same port composed %q and then %q. A shared link has to keep pointing at the "+
				"run it was taken from, however often the preview is closed and opened again",
			first, again,
		)
	}
}

func TestNoIssueAndPortComposeALabelDNSWouldRefuse(t *testing.T) {
	references := []string{"", "n-1", "norn-75", "abcde-123456789"}
	ports := []int{1, 8080, 43000, 65535}
	titles := []string{
		"",
		"...",
		testTitle,
		strings.Repeat("a very long title indeed ", 20),
	}

	for _, reference := range references {
		for _, port := range ports {
			for _, title := range titles {
				host := channelv1.PreviewHost(
					reference, title, testExecutionID, port,
					channelv1.PreviewBySubdomain, testDomain,
				)

				label, _, _ := strings.Cut(host, ".")
				if len(label) > channelv1.PreviewLabelMax {
					t.Fatalf(
						"%q is %d characters, past the %d a DNS label may be. The title is the "+
							"only elastic part and has to be cut to what the rest leaves",
						label, len(label), channelv1.PreviewLabelMax,
					)
				}

				if strings.Contains(label, "--") || strings.HasPrefix(label, "-") {
					t.Fatalf(
						"%q holds an empty segment. A title that slugifies to nothing has to "+
							"collapse rather than leave a hyphen with no name beside it",
						label,
					)
				}

				if !strings.HasSuffix(label, "-"+strconv.Itoa(port)) {
					t.Fatalf("%q does not end in the port %d it belongs to", label, port)
				}
			}
		}
	}
}

func TestALongIssueReferenceTakesTheRoomFromTheTitleRatherThanTheLabel(t *testing.T) {
	host := channelv1.PreviewHost(
		"ABCDE-123456789", testTitle, testExecutionID, 43000,
		channelv1.PreviewBySubdomain, testDomain,
	)

	want := "abcde-123456789-a-preview-exec-01m0smjxbj451kz0mcq6ty2gh1-43000.norn.ink"
	if host != want {
		t.Fatalf(
			"the host came out as %q, want %q. What the reference, the run and the port spend "+
				"is fixed, so the cut on the title is whatever they leave and never a constant",
			host, want,
		)
	}
}

func TestATitleThatSlugifiesToNothingLeavesNoEmptySegment(t *testing.T) {
	host := channelv1.PreviewHost(
		"NORN-75", "!!!", testExecutionID, 43000, channelv1.PreviewBySubdomain, testDomain,
	)

	want := "norn-75-exec-01m0smjxbj451kz0mcq6ty2gh1-43000.norn.ink"
	if host != want {
		t.Fatalf("the host came out as %q, want %q", host, want)
	}
}

func TestTheExecutionIdIsCarriedWholeAndNeverPrefixedTwice(t *testing.T) {
	host := channelv1.PreviewHost(
		"NORN-75", testTitle, testExecutionID, 43000, channelv1.PreviewBySubdomain, testDomain,
	)

	if strings.Contains(host, "exec-exec-") {
		t.Fatalf(
			"the host came out as %q. An execution id already carries its own prefix, and a "+
				"second one is an address that resolves nowhere",
			host,
		)
	}

	if !strings.Contains(host, strings.ToLower(testExecutionID)) {
		t.Fatalf("the host came out as %q and does not carry the run it belongs to", host)
	}
}

func TestAPathModeHostHoldsTheRunAndLeavesThePortToThePath(t *testing.T) {
	host := channelv1.PreviewHost(
		"NORN-75", testTitle, testExecutionID, 43000, channelv1.PreviewByPath, testDomain,
	)

	want := "norn-75-a-preview-address-must-exec-01m0smjxbj451kz0mcq6ty2gh1.norn.ink"
	if host != want {
		t.Fatalf(
			"the path-mode host came out as %q, want %q. One host serves the whole run there, "+
				"so the port is what tells two previews of it apart",
			host, want,
		)
	}
}

func TestAServerServingNoPreviewDomainComposesNoAddressAtAll(t *testing.T) {
	host := channelv1.PreviewHost(
		"NORN-75", testTitle, testExecutionID, 43000, channelv1.PreviewBySubdomain, "",
	)

	if host != "" {
		t.Fatalf(
			"a server with no preview domain composed %q. Handing somebody an address that "+
				"resolves nowhere is worse than admitting there is none",
			host,
		)
	}
}
