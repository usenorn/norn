package entity_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestAnUnclaimedDelegationIsClaimable(t *testing.T) {
	now := time.Date(2026, 8, 13, 9, 0, 0, 0, time.UTC)

	delegation := entity.IssueDelegation{}

	if !delegation.Claimable(now) {
		t.Fatal(
			"an open delegation nobody holds refused a claim; the queue would stall on the first " +
				"issue no runner had picked up yet",
		)
	}
}

func TestALiveClaimKeepsEveryOtherRunnerOut(t *testing.T) {
	now := time.Date(2026, 8, 13, 9, 0, 0, 0, time.UTC)

	delegation := entity.IssueDelegation{
		Claim: entity.DelegationClaim{
			Runner:    "laptop-a",
			Token:     uuid.New(),
			ClaimedAt: now,
			ExpiresAt: now.Add(entity.DelegationClaimTTLDefault),
		},
	}

	if delegation.Claimable(now) {
		t.Fatal(
			"an issue with a live claim read as claimable; two runners would drive the same " +
				"working tree at once",
		)
	}
}

func TestAClaimStopsHoldingTheIssueOnceItLapses(t *testing.T) {
	claimed := time.Date(2026, 8, 13, 9, 0, 0, 0, time.UTC)
	expires := claimed.Add(entity.DelegationClaimTTLDefault)

	delegation := entity.IssueDelegation{
		Claim: entity.DelegationClaim{
			Runner:    "laptop-a",
			Token:     uuid.New(),
			ClaimedAt: claimed,
			ExpiresAt: expires,
		},
	}

	cases := map[string]struct {
		at   time.Time
		want bool
	}{
		"a moment before it lapses": {expires.Add(-time.Second), false},
		"exactly when it lapses":    {expires, true},
		"long after it lapses":      {expires.Add(time.Hour), true},
	}

	for name, tc := range cases {
		if got := delegation.Claimable(tc.at); got != tc.want {
			t.Errorf(
				"%s: claimable = %v, want %v; a runner that died holding a claim must release the "+
					"issue on its own, or the queue is stuck until somebody notices",
				name, got, tc.want,
			)
		}
	}
}

func TestARecalledDelegationIsNeverClaimable(t *testing.T) {
	now := time.Date(2026, 8, 13, 9, 0, 0, 0, time.UTC)
	recalled := now.Add(-time.Hour)

	delegation := entity.IssueDelegation{RecalledAt: &recalled}

	if delegation.Claimable(now) {
		t.Fatal(
			"a recalled delegation offered itself for claiming; taking work back from an agent " +
				"has to mean the work stops",
		)
	}
}

func TestAClaimTTLOutsideTheAllowedRangeIsRefused(t *testing.T) {
	cases := map[string]struct {
		ttl  time.Duration
		want string
	}{
		"below the floor": {entity.DelegationClaimTTLMin - time.Second, entity.ValidationCodeOutOfRange},
		"at the floor":    {entity.DelegationClaimTTLMin, ""},
		"the default":     {entity.DelegationClaimTTLDefault, ""},
		"at the ceiling":  {entity.DelegationClaimTTLMax, ""},
		"above the ceiling": {
			entity.DelegationClaimTTLMax + time.Second, entity.ValidationCodeOutOfRange,
		},
	}

	for name, tc := range cases {
		if got := entity.ValidateDelegationClaimTTL("ttlSeconds", tc.ttl).Code; got != tc.want {
			t.Errorf("%s: code = %q, want %q", name, got, tc.want)
		}
	}
}

func TestAnUnsetClaimTTLFallsBackToTheDefault(t *testing.T) {
	if got := entity.DelegationClaimTTL(0); got != entity.DelegationClaimTTLDefault {
		t.Errorf("ttl = %v, want the default %v", got, entity.DelegationClaimTTLDefault)
	}

	if got := entity.DelegationClaimTTL(time.Minute); got != time.Minute {
		t.Errorf("ttl = %v, want the requested minute", got)
	}
}

func TestARunnerMustNameItself(t *testing.T) {
	cases := map[string]struct {
		runner string
		want   string
	}{
		"empty":          {"", entity.ValidationCodeRequired},
		"only spaces":    {"   ", entity.ValidationCodeRequired},
		"a name":         {"laptop-a", ""},
		"far too long":   {string(make([]rune, entity.RunnerNameMaxLen+1)), entity.ValidationCodeTooLong},
		"exactly at max": {string(make([]rune, entity.RunnerNameMaxLen)), ""},
	}

	for name, tc := range cases {
		if got := entity.ValidateRunnerName("runner", tc.runner).Code; got != tc.want {
			t.Errorf("%s: code = %q, want %q", name, got, tc.want)
		}
	}
}
