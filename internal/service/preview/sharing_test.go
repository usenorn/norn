package preview_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func TestAShareLinkLetsSomebodyOutsideTheWorkspaceLookUntilItIsWithdrawn(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.visible(nil)
	h.reported(t, "web", channelv1.PreviewOpen)

	minted := h.shared(t, "web", "")

	access, err := h.service.Redeem(
		context.Background(), hostFor("web"), tokenFrom(t, minted.URL), "",
	)
	if err != nil {
		t.Fatalf("redeem the share link: %v", err)
	}

	if access.Verdict != entity.PreviewAllowed {
		t.Fatalf("a live share link was answered %q", access.Verdict)
	}

	if err := h.service.RevokeShare(
		signedIn(context.Background(), h.caller),
		h.workspaceID, h.execution.ID, "web", minted.Link.ID,
	); err != nil {
		t.Fatalf("revoke the share link: %v", err)
	}

	_, err = h.service.Redeem(
		context.Background(), hostFor("web"), tokenFrom(t, minted.URL), "",
	)

	refusedWith(t, err, entity.ErrPreviewShareRevoked)
}

func TestRevokingAShareLinkAlsoShutsOutEverybodyItAlreadyLetIn(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.visible(nil)
	h.reported(t, "web", channelv1.PreviewOpen)

	minted := h.shared(t, "web", "")

	access, err := h.service.Redeem(
		context.Background(), hostFor("web"), tokenFrom(t, minted.URL), "",
	)
	if err != nil {
		t.Fatalf("redeem the share link: %v", err)
	}

	if err := h.service.RevokeShare(
		signedIn(context.Background(), h.caller),
		h.workspaceID, h.execution.ID, "web", minted.Link.ID,
	); err != nil {
		t.Fatalf("revoke the share link: %v", err)
	}

	held, err := h.service.Introspect(
		context.Background(), hostFor("web"), access.Token, viewerFrom("203.0.113.9"),
	)
	if err != nil {
		t.Fatalf("ask about a session the withdrawn link minted: %v", err)
	}

	if held.Verdict == entity.PreviewAllowed {
		t.Fatal(
			"somebody a withdrawn link let in is still inside. Withdrawing a link that only " +
				"stops new sessions leaves whoever already used it there until their own expires",
		)
	}
}

func TestOpeningThePasscodePageIsNotAGuessAtThePasscode(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.visible(nil)
	h.reported(t, "web", channelv1.PreviewOpen)

	minted := h.shared(t, "web", "open-sesame")
	token := tokenFrom(t, minted.URL)

	for range entity.PreviewShareMaxAttempts * 2 {
		_, err := h.service.Redeem(context.Background(), hostFor("web"), token, "")

		refusedWith(t, err, entity.ErrPreviewSharePasscodeNeeded)
	}

	if _, err := h.service.Redeem(
		context.Background(), hostFor("web"), token, "open-sesame",
	); err != nil {
		t.Fatalf(
			"asking for the passcode form counted as a guess (%v), so anybody holding the link "+
				"could lock everybody else out of it just by loading the page",
			err,
		)
	}
}

func TestAShareLinkWithAPasscodeOpensNothingWithoutIt(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.visible(nil)
	h.reported(t, "web", channelv1.PreviewOpen)

	minted := h.shared(t, "web", "open-sesame")
	token := tokenFrom(t, minted.URL)

	if !minted.Link.NeedsPasscode() {
		t.Fatal("a link minted with a passcode does not say it needs one")
	}

	for name, expected := range map[string]struct {
		given  string
		refuse error
	}{
		"with no passcode at all": {given: "", refuse: entity.ErrPreviewSharePasscodeNeeded},
		"with the wrong one":      {given: "not-it", refuse: entity.ErrPreviewSharePasscode},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := h.service.Redeem(
				context.Background(), hostFor("web"), token, expected.given,
			)

			refusedWith(t, err, expected.refuse)
		})
	}

	access, err := h.service.Redeem(
		context.Background(), hostFor("web"), token, "open-sesame",
	)
	if err != nil {
		t.Fatalf("redeem with the right passcode: %v", err)
	}

	if access.Verdict != entity.PreviewAllowed {
		t.Fatalf("the right passcode was answered %q", access.Verdict)
	}
}

func TestAPasscodeCannotBeGuessedAtOverAndOver(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.visible(nil)
	h.reported(t, "web", channelv1.PreviewOpen)

	minted := h.shared(t, "web", "open-sesame")
	token := tokenFrom(t, minted.URL)

	var last error

	for range entity.PreviewShareMaxAttempts + 1 {
		_, last = h.service.Redeem(context.Background(), hostFor("web"), token, "wrong")
	}

	refusedWith(t, last, entity.ErrPreviewShareGuessed)

	_, err := h.service.Redeem(context.Background(), hostFor("web"), token, "open-sesame")

	refusedWith(t, err, entity.ErrPreviewShareGuessed)
}

func TestAShareLinkIsAnsweredOnceAndKeptOnlyAsItsHash(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.visible(nil)
	h.reported(t, "web", channelv1.PreviewOpen)

	minted := h.shared(t, "web", "")
	token := tokenFrom(t, minted.URL)

	if !strings.HasPrefix(minted.URL, "https://"+hostFor("web")) {
		t.Fatalf("the share url %q does not point at the preview it opens", minted.URL)
	}

	held, err := h.service.ForExecution(
		signedIn(context.Background(), h.caller), h.workspaceID, h.execution.ID,
	)
	if err != nil {
		t.Fatalf("list the previews: %v", err)
	}

	for _, detail := range held {
		for _, link := range detail.Links {
			if strings.Contains(string(link.TokenHash), token) {
				t.Fatal(
					"the listing carries the live token. Reading the run would then be enough " +
						"to hand out every link somebody minted for it",
				)
			}
		}
	}
}

func TestAShareLinkOnlyEverOpensThePreviewItWasMintedFor(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.visible(nil)
	h.reported(t, "web", channelv1.PreviewOpen)
	h.reported(t, "docs", channelv1.PreviewOpen)

	minted := h.shared(t, "web", "")

	_, err := h.service.Redeem(
		context.Background(), hostFor("docs"), tokenFrom(t, minted.URL), "",
	)

	refusedWith(t, err, entity.ErrPreviewShareNotFound)
}

func TestAShareLinkIsRefusedForAPreviewTheMachineHasClosed(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.visible(nil)
	h.reported(t, "web", channelv1.PreviewOpen)

	minted := h.shared(t, "web", "")
	h.reported(t, "web", channelv1.PreviewClosed)

	_, err := h.service.Redeem(
		context.Background(), hostFor("web"), tokenFrom(t, minted.URL), "",
	)

	refusedWith(t, err, entity.ErrPreviewClosed)
}

func TestNobodyOutsideTheWorkspaceCanMintAShareLinkOfTheirOwn(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.visible(entity.ErrExecutionNotFound)
	h.stored[previewPort("web")] = entity.PreviewSession{
		ID:          uuid.New(),
		ExecutionID: h.execution.ID,
		WorkspaceID: h.workspaceID,
		Name:        "web",
		Service:     "web",
		Port:        previewPort("web"),
		Mode:        entity.PreviewBySubdomain,
		Host:        hostFor("web"),
		State:       entity.PreviewOpen,
	}

	_, err := h.service.Share(
		signedIn(context.Background(), uuid.New()),
		h.workspaceID,
		h.execution.ID,
		service.PreviewShareRequest{Name: "web"},
	)

	refusedWith(t, err, entity.ErrExecutionNotFound)
}

func TestAShareLinkIsRefusedALifetimeLongerThanTheInstanceAllows(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.visible(nil)
	h.reported(t, "web", channelv1.PreviewOpen)

	_, err := h.service.Share(
		signedIn(context.Background(), h.caller),
		h.workspaceID,
		h.execution.ID,
		service.PreviewShareRequest{Name: "web", Lifetime: 30 * 24 * time.Hour},
	)
	if err == nil {
		t.Fatal(
			"a link was minted to live a month. The longest lifetime is what an admin sets, " +
				"and a link nobody remembers is one nobody withdraws",
		)
	}
}

func TestMintingAndWithdrawingAShareLinkBothReachTheAuditLog(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.visible(nil)
	h.reported(t, "web", channelv1.PreviewOpen)

	minted := h.shared(t, "web", "")

	if err := h.service.RevokeShare(
		signedIn(context.Background(), h.caller),
		h.workspaceID, h.execution.ID, "web", minted.Link.ID,
	); err != nil {
		t.Fatalf("revoke the share link: %v", err)
	}

	if h.auditedFor(entity.AuditPreviewShared) != 1 {
		t.Error("minting a share link left nothing on the audit log")
	}

	if h.auditedFor(entity.AuditPreviewShareRevoked) != 1 {
		t.Error("withdrawing a share link left nothing on the audit log")
	}
}

func TestAViewerWhoCameThroughAShareLinkIsRecordedAsHavingDoneSo(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.visible(nil)
	h.reported(t, "web", channelv1.PreviewOpen)

	minted := h.shared(t, "web", "")

	access, err := h.service.Redeem(
		context.Background(), hostFor("web"), tokenFrom(t, minted.URL), "",
	)
	if err != nil {
		t.Fatalf("redeem the share link: %v", err)
	}

	if _, err := h.service.Introspect(
		context.Background(), hostFor("web"), access.Token, viewerFrom("203.0.113.9"),
	); err != nil {
		t.Fatalf("ask about the share-link viewer: %v", err)
	}

	seen := h.previewEvents()
	last := seen[len(seen)-1]

	if !strings.Contains(string(last.Detail), minted.Link.ID.String()) {
		t.Fatalf(
			"the timeline line %q does not name the link the viewer came through. Somebody "+
				"reading it cannot tell which link to withdraw",
			last.Detail,
		)
	}
}

func TestAShareLinkOutlivesTheNameThePreviewWasOpenedUnder(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.visible(nil)

	const port = 43000

	h.reportedOn(t, "issue-description", port, channelv1.PreviewOpen)

	minted := h.shared(t, "issue-description", "")

	h.reportedOn(t, "issue-description", port, channelv1.PreviewClosed)
	h.reportedOn(t, "web", port, channelv1.PreviewOpen)

	access, err := h.service.Redeem(
		context.Background(), hostOnPort(port), tokenFrom(t, minted.URL), "",
	)
	if err != nil {
		t.Fatalf(
			"a link shared outside the workspace stopped working because the run re-opened "+
				"the same port under another name: %v",
			err,
		)
	}

	if access.Verdict != entity.PreviewAllowed {
		t.Fatalf("the shared link was answered %q", access.Verdict)
	}
}
