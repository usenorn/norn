package entity_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

var reference = time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC)

func sessionWith(idleOffset, absoluteOffset time.Duration) entity.Session {
	return entity.Session{
		IssuedAt:          reference,
		LastUsedAt:        reference,
		IdleExpiresAt:     reference.Add(idleOffset),
		AbsoluteExpiresAt: reference.Add(absoluteOffset),
	}
}

func TestSessionAuthMethodValidity(t *testing.T) {
	for _, method := range []entity.SessionAuthMethod{entity.SessionAuthMethodPassword, entity.SessionAuthMethodSSO} {
		if !method.Valid() {
			t.Errorf("auth method %q should be valid", method)
		}
	}

	for _, method := range []entity.SessionAuthMethod{"", "PASSWORD", "saml"} {
		if method.Valid() {
			t.Errorf("auth method %q should not be valid", method)
		}
	}
}

func TestSessionExpiresAtTheEarlierOfIdleAndAbsolute(t *testing.T) {
	cases := []struct {
		name     string
		session  entity.Session
		expected time.Time
	}{
		{"idle first", sessionWith(time.Hour, 24*time.Hour), reference.Add(time.Hour)},
		{"absolute first", sessionWith(24*time.Hour, time.Hour), reference.Add(time.Hour)},
		{"converged", sessionWith(time.Hour, time.Hour), reference.Add(time.Hour)},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.session.ExpiresAt(); !got.Equal(c.expected) {
				t.Errorf("ExpiresAt = %s, want %s", got, c.expected)
			}
		})
	}
}

func TestSessionExpiryIsInclusiveOfTheDeadline(t *testing.T) {
	session := sessionWith(time.Hour, 24*time.Hour)

	cases := []struct {
		name    string
		now     time.Time
		expired bool
	}{
		{"before", reference.Add(time.Hour - time.Second), false},
		{"at the deadline", reference.Add(time.Hour), true},
		{"after", reference.Add(2 * time.Hour), true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := session.ExpiredAt(c.now); got != c.expired {
				t.Errorf("ExpiredAt = %t, want %t", got, c.expired)
			}
		})
	}
}

func TestASessionIssuedAtOrBeforeTheRevocationInstantIsRevoked(t *testing.T) {
	session := sessionWith(time.Hour, 24*time.Hour)

	cases := []struct {
		name      string
		revokedAt time.Time
		revoked   bool
	}{
		{"never revoked", time.Time{}, false},
		{"revoked before the session was issued", reference.Add(-time.Second), false},
		{"revoked at the issuing instant", reference, true},
		{"revoked after the session was issued", reference.Add(time.Second), true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := session.RevokedAt(c.revokedAt); got != c.revoked {
				t.Errorf("RevokedAt = %t, want %t", got, c.revoked)
			}
		})
	}
}

func TestRefreshNeverAdvancesPastTheStoredAbsoluteDeadline(t *testing.T) {
	session := sessionWith(time.Hour, 2*time.Hour)

	refreshed := session.Refreshed(reference.Add(90*time.Minute), 24*time.Hour)

	if !refreshed.IdleExpiresAt.Equal(session.AbsoluteExpiresAt) {
		t.Fatalf("IdleExpiresAt = %s, want it capped at the absolute deadline %s",
			refreshed.IdleExpiresAt, session.AbsoluteExpiresAt)
	}

	if !refreshed.AbsoluteExpiresAt.Equal(session.AbsoluteExpiresAt) {
		t.Fatal("refreshing moved the absolute deadline, which would make it unreachable")
	}
}

func TestRefreshExtendsTheIdleDeadlineAndStampsLastUse(t *testing.T) {
	session := sessionWith(time.Hour, 30*24*time.Hour)

	now := reference.Add(30 * time.Minute)
	refreshed := session.Refreshed(now, time.Hour)

	if !refreshed.LastUsedAt.Equal(now) {
		t.Fatalf("LastUsedAt = %s, want %s", refreshed.LastUsedAt, now)
	}

	if !refreshed.IdleExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("IdleExpiresAt = %s, want %s", refreshed.IdleExpiresAt, now.Add(time.Hour))
	}
}

func TestNeedsRefreshThrottlesWritesAndStopsOnceDeadlinesConverge(t *testing.T) {
	cases := []struct {
		name     string
		session  entity.Session
		now      time.Time
		interval time.Duration
		needs    bool
	}{
		{"within the interval", sessionWith(time.Hour, 24*time.Hour), reference.Add(30 * time.Second), time.Minute, false},
		{"at the interval", sessionWith(time.Hour, 24*time.Hour), reference.Add(time.Minute), time.Minute, true},
		{"past the interval", sessionWith(time.Hour, 24*time.Hour), reference.Add(10 * time.Minute), time.Minute, true},
		{"deadlines converged", sessionWith(time.Hour, time.Hour), reference.Add(10 * time.Minute), time.Minute, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.session.NeedsRefresh(c.now, c.interval); got != c.needs {
				t.Errorf("NeedsRefresh = %t, want %t", got, c.needs)
			}
		})
	}
}

func TestSessionTokensAreUniqueAndReturnTheirOwnHash(t *testing.T) {
	first, firstHash, err := entity.NewSessionToken()
	if err != nil {
		t.Fatalf("NewSessionToken: %v", err)
	}

	second, _, err := entity.NewSessionToken()
	if err != nil {
		t.Fatalf("NewSessionToken: %v", err)
	}

	if first == second {
		t.Fatal("two generated session tokens are identical")
	}

	if firstHash != entity.HashSessionToken(first) {
		t.Fatal("the returned hash does not match the hash of the returned token")
	}

	if strings.Contains(firstHash, first) {
		t.Fatal("the hash contains the raw token")
	}
}

func TestTruncateUserAgentBoundsAttackerControlledInput(t *testing.T) {
	if got := entity.TruncateUserAgent("Mozilla/5.0"); got != "Mozilla/5.0" {
		t.Fatalf("TruncateUserAgent altered a short value: %q", got)
	}

	long := strings.Repeat("a", entity.UserAgentMaxLen+100)
	if got := entity.TruncateUserAgent(long); len(got) != entity.UserAgentMaxLen {
		t.Fatalf("TruncateUserAgent length = %d, want %d", len(got), entity.UserAgentMaxLen)
	}
}

func TestAuthEnforcementPermits(t *testing.T) {
	cases := []struct {
		enforcement entity.AuthEnforcement
		method      entity.SessionAuthMethod
		permitted   bool
	}{
		{entity.AuthEnforcementAny, entity.SessionAuthMethodPassword, true},
		{entity.AuthEnforcementAny, entity.SessionAuthMethodSSO, true},
		{entity.AuthEnforcementAny, entity.SessionAuthMethod("none"), false},
		{entity.AuthEnforcementSSO, entity.SessionAuthMethodPassword, false},
		{entity.AuthEnforcementSSO, entity.SessionAuthMethodSSO, true},
		{entity.AuthEnforcement("unknown"), entity.SessionAuthMethodSSO, false},
	}

	for _, c := range cases {
		t.Run(string(c.enforcement)+"_"+string(c.method), func(t *testing.T) {
			if got := c.enforcement.Permits(c.method); got != c.permitted {
				t.Errorf("Permits = %t, want %t", got, c.permitted)
			}
		})
	}
}

func TestAWorkspaceWithoutAPolicyAcceptsEveryAuthenticationMethod(t *testing.T) {
	policy := entity.DefaultWorkspaceAuthPolicy(uuid.New())

	if policy.Enforcement != entity.AuthEnforcementAny {
		t.Fatalf("default enforcement = %q, want any", policy.Enforcement)
	}

	if !policy.Enforcement.Permits(entity.SessionAuthMethodPassword) {
		t.Fatal("the default policy must accept a password session")
	}
}
