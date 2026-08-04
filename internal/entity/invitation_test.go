package entity_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

func TestInvitationTokenHashesStablyAndNeverEmbedsThePlaintext(t *testing.T) {
	token, hash, err := entity.NewInvitationToken()
	if err != nil {
		t.Fatalf("NewInvitationToken: %v", err)
	}

	if token == "" {
		t.Fatal("NewInvitationToken returned an empty token")
	}

	if !bytes.Equal(hash, entity.HashInvitationToken(token)) {
		t.Fatal("hashing the issued token does not reproduce the stored hash")
	}

	if strings.Contains(string(hash), token) {
		t.Fatal("the stored hash contains the plaintext token")
	}
}

func TestInvitationTokensAreDistinctPerIssue(t *testing.T) {
	first, firstHash, err := entity.NewInvitationToken()
	if err != nil {
		t.Fatalf("NewInvitationToken: %v", err)
	}

	second, secondHash, err := entity.NewInvitationToken()
	if err != nil {
		t.Fatalf("NewInvitationToken: %v", err)
	}

	if first == second {
		t.Fatal("two issued invitation tokens are identical")
	}

	if bytes.Equal(firstHash, secondHash) {
		t.Fatal("two issued invitation tokens hash to the same value")
	}
}

func TestInvitationIsUnusableOnceRevokedAcceptedOrExpired(t *testing.T) {
	now := time.Date(2026, time.August, 2, 12, 0, 0, 0, time.UTC)
	accepted := now.Add(-time.Hour)

	cases := []struct {
		name       string
		invitation entity.Invitation
		want       error
	}{
		{
			name:       "pending and in date",
			invitation: entity.Invitation{Status: entity.InvitationStatusPending, ExpiresAt: now.Add(time.Hour)},
			want:       nil,
		},
		{
			name:       "revoked before expiry",
			invitation: entity.Invitation{Status: entity.InvitationStatusRevoked, ExpiresAt: now.Add(time.Hour)},
			want:       entity.ErrInvitationRevoked,
		},
		{
			name: "already accepted",
			invitation: entity.Invitation{
				Status:     entity.InvitationStatusAccepted,
				ExpiresAt:  now.Add(time.Hour),
				AcceptedAt: &accepted,
			},
			want: entity.ErrInvitationAccepted,
		},
		{
			name:       "expired exactly now",
			invitation: entity.Invitation{Status: entity.InvitationStatusPending, ExpiresAt: now},
			want:       entity.ErrInvitationExpired,
		},
		{
			name:       "expired earlier",
			invitation: entity.Invitation{Status: entity.InvitationStatusPending, ExpiresAt: now.Add(-time.Second)},
			want:       entity.ErrInvitationExpired,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := testCase.invitation.UsableAt(now); got != testCase.want {
				t.Fatalf("UsableAt = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestInvitationRevokedOutranksExpiry(t *testing.T) {
	now := time.Date(2026, time.August, 2, 12, 0, 0, 0, time.UTC)

	invitation := entity.Invitation{
		Status:    entity.InvitationStatusRevoked,
		ExpiresAt: now.Add(-time.Hour),
	}

	if got := invitation.UsableAt(now); got != entity.ErrInvitationRevoked {
		t.Fatalf("UsableAt = %v, want ErrInvitationRevoked", got)
	}
}

func TestInvitationEnumsAcceptOnlyKnownValues(t *testing.T) {
	statuses := map[entity.InvitationStatus]bool{
		entity.InvitationStatusPending:  true,
		entity.InvitationStatusAccepted: true,
		entity.InvitationStatusRevoked:  true,
		"":                              false,
		"cancelled":                     false,
	}

	for status, want := range statuses {
		if got := status.Valid(); got != want {
			t.Errorf("InvitationStatus(%q).Valid() = %t, want %t", status, got, want)
		}
	}

	deliveries := map[entity.InvitationDelivery]bool{
		entity.InvitationDeliveryPending:  true,
		entity.InvitationDeliverySent:     true,
		entity.InvitationDeliveryFailed:   true,
		entity.InvitationDeliveryLinkOnly: true,
		"":                                false,
		"bounced":                         false,
	}

	for delivery, want := range deliveries {
		if got := delivery.Valid(); got != want {
			t.Errorf("InvitationDelivery(%q).Valid() = %t, want %t", delivery, got, want)
		}
	}

	outcomes := map[entity.InvitationOutcome]bool{
		entity.InvitationOutcomeCreated:       true,
		entity.InvitationOutcomeInvalidEmail:  true,
		entity.InvitationOutcomeAlreadyMember: true,
		"":                                    false,
		"seat_limit_reached":                  false,
	}

	for outcome, want := range outcomes {
		if got := outcome.Valid(); got != want {
			t.Errorf("InvitationOutcome(%q).Valid() = %t, want %t", outcome, got, want)
		}
	}
}

func TestInvitationLinksLastSevenDays(t *testing.T) {
	if entity.InvitationTokenTTL != 7*24*time.Hour {
		t.Fatalf("InvitationTokenTTL = %v, want 7 days", entity.InvitationTokenTTL)
	}
}
