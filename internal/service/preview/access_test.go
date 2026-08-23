package preview_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func TestTheGatewayIsRefusedAboutAHostNoMachineEverRegistered(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.reported(t, "web", channelv1.PreviewOpen)

	access, err := h.service.Introspect(
		context.Background(), "postgres-exec-01ABC."+previewDomain, "", viewerFrom("203.0.113.9"),
	)
	if err != nil {
		t.Fatalf("ask about an unregistered host: %v", err)
	}

	if access.Verdict != entity.PreviewRefused {
		t.Fatalf(
			"a host no machine registered was answered %q, want refused. The whole guarantee is "+
				"that an unregistered pair is unroutable, and it is this answer that makes it so",
			access.Verdict,
		)
	}
}

func TestAViewerWithNoPreviewSessionIsSentIntoSignInAndBackToThePreview(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.reported(t, "web", channelv1.PreviewOpen)

	access, err := h.service.Introspect(
		context.Background(), hostFor("web"), "", viewerFrom("203.0.113.9"),
	)
	if err != nil {
		t.Fatalf("ask about a viewer with no session: %v", err)
	}

	if access.Verdict != entity.PreviewSignIn {
		t.Fatalf("a signed-out viewer was answered %q, want sign_in", access.Verdict)
	}

	if !strings.HasPrefix(access.Redirect, appBaseURL+"/sign-in?") {
		t.Fatalf("the viewer was sent to %q rather than into norn's own sign-in", access.Redirect)
	}

	if !strings.Contains(access.Redirect, "previews") ||
		!strings.Contains(access.Redirect, "host") {
		t.Fatalf(
			"the sign-in link at %q does not carry the way back to the preview, so somebody who "+
				"signs in lands on the dashboard rather than the thing they clicked",
			access.Redirect,
		)
	}
}

func TestASignedOutBrowserAtTheAuthorizeDoorIsSentToSignInRatherThanRefused(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.visible(nil)
	h.reported(t, "web", channelv1.PreviewOpen)

	access, err := h.service.Authorize(context.Background(), hostFor("web"), "/dashboard")
	if err != nil {
		t.Fatalf("authorize a signed-out browser: %v", err)
	}

	if access.Verdict != entity.PreviewSignIn {
		t.Fatalf("a signed-out browser was answered %q, want sign_in", access.Verdict)
	}
}

func TestAMemberOfTheWorkspaceIsLetInAndSomebodyOutsideItIsNot(t *testing.T) {
	member := newHarness(t)
	member.holds()
	member.visible(nil)
	member.reported(t, "web", channelv1.PreviewOpen)

	access, err := member.service.Authorize(
		signedIn(context.Background(), member.caller), hostFor("web"), "/",
	)
	if err != nil {
		t.Fatalf("authorize a member: %v", err)
	}

	if access.Verdict != entity.PreviewAllowed {
		t.Fatalf("a member of the workspace was answered %q, want allowed", access.Verdict)
	}

	stranger := newHarness(t)
	stranger.holds()
	stranger.visible(entity.ErrExecutionNotFound)
	stranger.reported(t, "web", channelv1.PreviewOpen)

	_, err = stranger.service.Authorize(
		signedIn(context.Background(), stranger.caller), hostFor("web"), "/",
	)

	refusedWith(t, err, entity.ErrExecutionNotFound)
}

func TestTheTicketAMemberCarriesBackIsSpentOnceAndBecomesTheirSession(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.visible(nil)
	h.reported(t, "web", channelv1.PreviewOpen)

	access, err := h.service.Authorize(
		signedIn(context.Background(), h.caller), hostFor("web"), "/",
	)
	if err != nil {
		t.Fatalf("authorize a member: %v", err)
	}

	ticket := ticketFrom(t, access.Redirect)

	session, err := h.service.RedeemTicket(context.Background(), ticket)
	if err != nil {
		t.Fatalf("redeem the ticket: %v", err)
	}

	if session.Token == "" {
		t.Fatal("redeeming the ticket produced no preview session")
	}

	if _, err := h.service.RedeemTicket(context.Background(), ticket); err == nil {
		t.Fatal(
			"the same ticket was redeemed twice. It rides in a URL, so anything that logs or " +
				"refers the address would hand out a second session",
		)
	}
}

func TestASessionMintedForOneRunNeverOpensAnotherRunsPreview(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.visible(nil)
	h.reported(t, "web", channelv1.PreviewOpen)
	h.reported(t, "docs", channelv1.PreviewOpen)

	access, err := h.service.Authorize(
		signedIn(context.Background(), h.caller), hostFor("web"), "/",
	)
	if err != nil {
		t.Fatalf("authorize a member: %v", err)
	}

	session, err := h.service.RedeemTicket(context.Background(), ticketFrom(t, access.Redirect))
	if err != nil {
		t.Fatalf("redeem the ticket: %v", err)
	}

	held, err := h.service.Introspect(
		context.Background(), hostFor("docs"), session.Token, viewerFrom("203.0.113.9"),
	)
	if err != nil {
		t.Fatalf("ask about the other preview: %v", err)
	}

	if held.Verdict == entity.PreviewAllowed {
		t.Fatal(
			"a session for one preview opened another. Each address is a different service on " +
				"the machine, so this would be the whole point of the registration constraint lost",
		)
	}
}

func TestAViewerHoldingASessionIsLetInUntilThePreviewCloses(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.visible(nil)
	h.reported(t, "web", channelv1.PreviewOpen)

	session := sessionFor(t, h, "web")

	access, err := h.service.Introspect(
		context.Background(), hostFor("web"), session, viewerFrom("203.0.113.9"),
	)
	if err != nil {
		t.Fatalf("ask about a viewer holding a session: %v", err)
	}

	if access.Verdict != entity.PreviewAllowed {
		t.Fatalf("a viewer holding a live session was answered %q", access.Verdict)
	}

	h.reported(t, "web", channelv1.PreviewClosed)

	closed, err := h.service.Introspect(
		context.Background(), hostFor("web"), session, viewerFrom("203.0.113.9"),
	)
	if err != nil {
		t.Fatalf("ask about a closed preview: %v", err)
	}

	if closed.Verdict != entity.PreviewRefused {
		t.Fatalf(
			"a preview the machine closed was answered %q. The service behind it is gone, so "+
				"the gateway would proxy to nothing",
			closed.Verdict,
		)
	}
}

func TestOneViewerAddsOneTimelineLineHoweverManyRequestsThePageMakes(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.visible(nil)
	h.reported(t, "web", channelv1.PreviewOpen)

	session := sessionFor(t, h, "web")
	opening := len(h.previewEvents())

	for range 5 {
		if _, err := h.service.Introspect(
			context.Background(), hostFor("web"), session, viewerFrom("203.0.113.9"),
		); err != nil {
			t.Fatalf("ask about the viewer: %v", err)
		}
	}

	added := len(h.previewEvents()) - opening
	if added != 1 {
		t.Fatalf(
			"one viewer loading a page put %d lines on the timeline, want 1. The gateway asks "+
				"once per request, so every image on the page would bury the run's own history",
			added,
		)
	}
}

func TestADifferentAddressLookingAtThePreviewIsItsOwnLineOnTheTimeline(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.visible(nil)
	h.reported(t, "web", channelv1.PreviewOpen)

	session := sessionFor(t, h, "web")
	opening := len(h.previewEvents())

	for _, address := range []string{"203.0.113.9", "198.51.100.4"} {
		if _, err := h.service.Introspect(
			context.Background(), hostFor("web"), session, viewerFrom(address),
		); err != nil {
			t.Fatalf("ask about a viewer at %s: %v", address, err)
		}
	}

	added := len(h.previewEvents()) - opening
	if added != 2 {
		t.Fatalf(
			"two addresses produced %d lines, want 2. A session that moved somewhere else is "+
				"exactly what somebody reading the audit is looking for",
			added,
		)
	}
}

func TestEveryLookAtAPreviewIsAlsoOnTheAuditLog(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.visible(nil)
	h.reported(t, "web", channelv1.PreviewOpen)

	session := sessionFor(t, h, "web")

	if _, err := h.service.Introspect(
		context.Background(), hostFor("web"), session, viewerFrom("203.0.113.9"),
	); err != nil {
		t.Fatalf("ask about the viewer: %v", err)
	}

	if h.auditedFor(entity.AuditPreviewOpened) != 1 {
		t.Fatal(
			"looking at a preview left nothing on the audit log. The timeline is the run's, and " +
				"an admin reading the workspace's own record would see nothing at all",
		)
	}
}

func TestAViewerCarryingSomethingThatIsNotAGrantIsSentToSignInRatherThanIn(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.reported(t, "web", channelv1.PreviewOpen)

	access, err := h.service.Introspect(
		context.Background(), hostFor("web"), "not-a-grant", viewerFrom("203.0.113.9"),
	)
	if err != nil {
		t.Fatalf("ask about an unknown grant: %v", err)
	}

	if access.Verdict != entity.PreviewSignIn {
		t.Fatalf("an unknown grant was answered %q, want sign_in", access.Verdict)
	}
}

func TestAServerServingNoPreviewDomainRefusesToHandOutASession(t *testing.T) {
	h := newHarnessServing(t, "")
	h.holds()
	h.visible(nil)
	h.reported(t, "web", channelv1.PreviewOpen)

	_, err := h.service.Authorize(
		signedIn(context.Background(), h.caller), hostFor("web"), "/",
	)

	refusedWith(t, err, entity.ErrPreviewNotRoutable)
}

func sessionFor(t *testing.T, h *harness, name string) string {
	t.Helper()

	access, err := h.service.Authorize(
		signedIn(context.Background(), h.caller), hostFor(name), "/",
	)
	if err != nil {
		t.Fatalf("authorize a member for %s: %v", name, err)
	}

	session, err := h.service.RedeemTicket(context.Background(), ticketFrom(t, access.Redirect))
	if err != nil {
		t.Fatalf("redeem the ticket for %s: %v", name, err)
	}

	return session.Token
}

func holdingSessions(ctx context.Context, accountIDs ...uuid.UUID) context.Context {
	held := make([]entity.Session, 0, len(accountIDs))

	for index, accountID := range accountIDs {
		held = append(held, entity.Session{
			ID:        uuid.New(),
			Slot:      strconv.Itoa(index),
			AccountID: accountID,
			IssuedAt:  time.Now().UTC().Add(time.Duration(index) * time.Minute),
		})
	}

	return identity.WithSignedIn(ctx, held)
}

func (h *harness) visibleOnlyTo(accountID uuid.UUID) {
	h.runs.EXPECT().
		Visible(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			ctx context.Context, _ uuid.UUID, _ string,
		) (entity.Execution, error) {
			actor, signedIn := identity.Actor(ctx)
			if !signedIn || actor.AccountID != accountID {
				return entity.Execution{}, entity.AccessDeniedError{
					Reason: entity.DenyReasonNotAMember,
				}
			}

			return h.execution, nil
		}).
		AnyTimes()
}

func TestAViewerHoldingTwoAccountsIsLetInAsTheOneThatCanSeeTheRun(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.reported(t, "web", channelv1.PreviewOpen)

	outsider := uuid.New()

	h.visibleOnlyTo(h.caller)

	access, err := h.service.Authorize(
		holdingSessions(context.Background(), outsider, h.caller), hostFor("web"), "/",
	)
	if err != nil {
		t.Fatalf("authorize a browser holding two accounts: %v", err)
	}

	if access.Verdict != entity.PreviewAllowed {
		t.Fatalf(
			"a viewer holding an account that can see the run was answered %q; a second account "+
				"signed in elsewhere in the browser must not cost them the preview",
			access.Verdict,
		)
	}

	grant, found := h.tickets[ticketFrom(t, access.Redirect)]
	if !found {
		t.Fatalf("no ticket was issued for %q", access.Redirect)
	}

	if grant.AccountID != h.caller {
		t.Fatalf(
			"the grant was issued to %s, want %s; it has to name the account that can actually "+
				"see the run, not whichever session happened to be first",
			grant.AccountID, h.caller,
		)
	}
}

func TestAViewerWhoseAccountsCannotSeeTheRunIsStillSentToSignIn(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.reported(t, "web", channelv1.PreviewOpen)
	h.visibleOnlyTo(uuid.New())

	access, err := h.service.Authorize(
		holdingSessions(context.Background(), uuid.New(), uuid.New()), hostFor("web"), "/",
	)
	if err != nil {
		t.Fatalf("authorize a browser holding only outsiders: %v", err)
	}

	if access.Verdict != entity.PreviewSignIn {
		t.Fatalf(
			"a viewer whose accounts cannot see the run was answered %q, want sign_in; electing "+
				"an account must never widen who gets in",
			access.Verdict,
		)
	}
}

func TestATroubleReadingTheRunIsNotMistakenForTheWrongAccount(t *testing.T) {
	h := newHarness(t)
	h.holds()
	h.reported(t, "web", channelv1.PreviewOpen)

	broken := errors.New("the database is unreachable")

	h.runs.EXPECT().
		Visible(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.Execution{}, broken).
		AnyTimes()

	_, err := h.service.Authorize(
		holdingSessions(context.Background(), h.caller), hostFor("web"), "/",
	)

	if !errors.Is(err, broken) {
		t.Fatalf(
			"a failure reading the run surfaced as %v; reading it as \"not this account\" would "+
				"send somebody round the sign-in loop instead of saying what broke",
			err,
		)
	}
}
