package session_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func held(accountID uuid.UUID, slot string, issuedAt time.Time) entity.Session {
	session := liveSession(accountID, issuedAt)
	session.Slot = slot
	session.TokenHash = entity.HashSessionToken("token-" + slot)

	return session
}

func presenting(sessions ...entity.Session) []service.PresentedSession {
	presented := make([]service.PresentedSession, 0, len(sessions))

	for _, session := range sessions {
		presented = append(presented, service.PresentedSession{
			Slot:  session.Slot,
			Token: "token-" + session.Slot,
		})
	}

	return presented
}

func (h *harness) presents(sessions ...entity.Session) {
	for _, session := range sessions {
		h.sessions.EXPECT().
			Get(gomock.Any(), entity.HashSessionToken("token-"+session.Slot)).
			Return(session, nil).
			AnyTimes()

		h.sessions.EXPECT().
			RevokedAt(gomock.Any(), session.AccountID).
			Return(time.Time{}, nil).
			AnyTimes()
	}
}

func TestASingleSessionActsWithoutBeingNamed(t *testing.T) {
	h := newHarness(t)
	only := held(uuid.New(), "one", time.Now().UTC())
	h.presents(only)

	resolved, err := h.service.Resolve(context.Background(), service.ResolveSessionsInput{
		Presented: presenting(only),
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if !resolved.Found || resolved.Acting.Slot != "one" {
		t.Fatal("the only live session did not act; a browser with one account must never have to name it")
	}
}

func TestASelectorNamingASessionTheRequestDidNotPresentActsAsNobody(t *testing.T) {
	h := newHarness(t)
	first := held(uuid.New(), "one", time.Now().UTC())
	second := held(uuid.New(), "two", time.Now().UTC())
	h.presents(first, second)

	resolved, err := h.service.Resolve(context.Background(), service.ResolveSessionsInput{
		Presented: presenting(first, second),
		Selector:  "three",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if resolved.Found {
		t.Fatalf(
			"a selector naming an absent session acted as %s. Falling back would let a stale "+
				"link act as the wrong person.",
			resolved.Acting.AccountID,
		)
	}

	if len(resolved.Held) != 2 {
		t.Fatal("the signed-in set was dropped; the browser still holds those sessions")
	}
}

func TestSeveralSessionsAndNoSelectorActAsNobody(t *testing.T) {
	h := newHarness(t)
	first := held(uuid.New(), "one", time.Now().UTC())
	second := held(uuid.New(), "two", time.Now().UTC())
	h.presents(first, second)

	resolved, err := h.service.Resolve(context.Background(), service.ResolveSessionsInput{
		Presented: presenting(first, second),
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if resolved.Found {
		t.Fatal("an account was guessed; which identity acts is never inferred from recency")
	}
}

func TestAWorkspaceInThePathPicksTheSessionThatBelongsToIt(t *testing.T) {
	h := newHarness(t)
	now := time.Now().UTC()
	stranger := held(uuid.New(), "one", now.Add(-time.Hour))
	member := held(uuid.New(), "two", now)
	workspaceID := uuid.New()

	h.presents(stranger, member)
	h.memberships.EXPECT().
		Get(gomock.Any(), workspaceID, stranger.AccountID).
		Return(entity.Membership{}, entity.ErrMembershipNotFound)
	h.memberships.EXPECT().
		Get(gomock.Any(), workspaceID, member.AccountID).
		Return(entity.Membership{WorkspaceID: workspaceID, AccountID: member.AccountID}, nil)

	resolved, err := h.service.Resolve(context.Background(), service.ResolveSessionsInput{
		Presented:   presenting(stranger, member),
		WorkspaceID: workspaceID,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if !resolved.Found || resolved.Acting.AccountID != member.AccountID {
		t.Fatal("an address that names a workspace and cannot carry a selector did not act as its member")
	}
}

func TestACookieClaimingASlotItsSessionDoesNotCarryIsDiscarded(t *testing.T) {
	h := newHarness(t)
	genuine := held(uuid.New(), "one", time.Now().UTC())
	h.presents(genuine)

	resolved, err := h.service.Resolve(context.Background(), service.ResolveSessionsInput{
		Presented: []service.PresentedSession{{Slot: "tossed", Token: "token-one"}},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if resolved.Found || len(resolved.Held) != 0 {
		t.Fatal("a cookie named for one slot carrying another slot's session was honoured")
	}

	if len(resolved.Dead) != 1 || resolved.Dead[0] != "tossed" {
		t.Fatal("the mismatched cookie was not marked for expiry")
	}
}

func TestOnlyTheActingSessionIsStamped(t *testing.T) {
	h := newHarness(t)
	now := time.Now().UTC()
	acting := held(uuid.New(), "one", now.Add(-2*refreshInterval))
	idle := held(uuid.New(), "two", now.Add(-2*refreshInterval))

	h.presents(acting, idle)
	h.sessions.EXPECT().
		Touch(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, session entity.Session) error {
			if session.Slot != "one" {
				t.Errorf(
					"session %q was stamped though it was not acting; a session only used because "+
						"the browser presented it must not look active",
					session.Slot,
				)
			}

			return nil
		})

	if _, err := h.service.Resolve(context.Background(), service.ResolveSessionsInput{
		Presented: presenting(acting, idle),
		Selector:  "one",
	}); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
}
