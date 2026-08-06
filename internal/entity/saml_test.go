package entity_test

import (
	"strings"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

func TestAnEmailIsFoundHoweverTheProviderSpellsTheAttribute(t *testing.T) {
	for name, attribute := range map[string]string{
		"the plain name":     "email",
		"the LDAP name":      "mail",
		"the camelCase name": "emailAddress",
		"the LDAP OID":       "urn:oid:0.9.2342.19200300.100.1.3",
		"the AD FS claim":    "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
	} {
		identity, err := entity.ResolveSAMLIdentity(entity.SAMLAssertion{
			ID:         "assertion-1",
			NameID:     "ada",
			Attributes: map[string][]string{attribute: {"Ada@Example.com"}},
		}, entity.SAMLAttributeMapping{})
		if err != nil {
			t.Errorf("%s (%s): %v", name, attribute, err)

			continue
		}

		if identity.Email != "ada@example.com" {
			t.Errorf("%s: got %q, want the normalised address", name, identity.Email)
		}
	}
}

func TestAnAssertionWithNoNameIDIsRefusedRatherThanKeyedOnItsAddress(t *testing.T) {
	_, err := entity.ResolveSAMLIdentity(entity.SAMLAssertion{
		ID:         "assertion-1",
		Attributes: map[string][]string{"email": {"ada@example.com"}},
	}, entity.SAMLAttributeMapping{})

	if err == nil {
		t.Fatal(
			"an assertion with no NameID was accepted, and the address became the subject. The " +
				"subject is what makes the binding durable, so deriving it from the very attribute " +
				"it is meant to outrank leaves nothing pinning the identity down.",
		)
	}

	if stage := stageOf(t, err); stage != entity.SSOStageAttributes {
		t.Fatalf("refused at stage %q, want %q", stage, entity.SSOStageAttributes)
	}
}

func TestAnExplicitMappingBeatsTheGuesses(t *testing.T) {
	identity, err := entity.ResolveSAMLIdentity(entity.SAMLAssertion{
		ID:     "assertion-1",
		NameID: "ada",
		Attributes: map[string][]string{
			"email":       {"wrong@example.com"},
			"work_email":  {"right@example.com"},
			"displayName": {"Ada Lovelace"},
		},
	}, entity.SAMLAttributeMapping{Email: "work_email"})
	if err != nil {
		t.Fatalf("ResolveSAMLIdentity: %v", err)
	}

	if identity.Email != "right@example.com" {
		t.Fatalf(
			"got %q. An administrator who names the attribute explicitly has told Norn which one "+
				"is authoritative; guessing over the top of that would sign people in as the wrong person.",
			identity.Email,
		)
	}
}

func TestAnEmailOnlyInTheNameIDIsStillFound(t *testing.T) {
	identity, err := entity.ResolveSAMLIdentity(entity.SAMLAssertion{
		ID:           "assertion-1",
		NameID:       "Ada@Example.com",
		NameIDFormat: "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress",
	}, entity.SAMLAttributeMapping{})
	if err != nil {
		t.Fatalf("a provider that puts the address only in the NameID was rejected: %v", err)
	}

	if identity.Email != "ada@example.com" {
		t.Fatalf("got %q, want the address from the NameID", identity.Email)
	}
}

func TestAnAssertionWithNoAddressAnywhereIsRefusedAtTheAttributeStage(t *testing.T) {
	_, err := entity.ResolveSAMLIdentity(entity.SAMLAssertion{
		ID:         "assertion-1",
		NameID:     "ada",
		Attributes: map[string][]string{"displayName": {"Ada Lovelace"}},
	}, entity.SAMLAttributeMapping{})
	if err == nil {
		t.Fatal("an assertion with no email address was accepted")
	}

	if stage := stageOf(t, err); stage != entity.SSOStageAttributes {
		t.Fatalf("stage %q, want %q", stage, entity.SSOStageAttributes)
	}
}

func TestAnAssertionWithNoIDIsRefusedBecauseReplayCouldNotBeDetected(t *testing.T) {
	_, err := entity.ResolveSAMLIdentity(entity.SAMLAssertion{
		NameID:     "ada@example.com",
		Attributes: map[string][]string{"email": {"ada@example.com"}},
	}, entity.SAMLAttributeMapping{})
	if err == nil {
		t.Fatal(
			"an assertion with no ID was accepted. Without an ID there is nothing to record, so " +
				"the same assertion could be replayed for ever.",
		)
	}

	if stage := stageOf(t, err); stage != entity.SSOStageResponse {
		t.Fatalf("stage %q, want %q", stage, entity.SSOStageResponse)
	}
}

func TestGroupsAreCollectedFromEveryMatchingAttribute(t *testing.T) {
	identity, err := entity.ResolveSAMLIdentity(entity.SAMLAssertion{
		ID:     "assertion-1",
		NameID: "ada@example.com",
		Attributes: map[string][]string{
			"email":  {"ada@example.com"},
			"groups": {"engineering", "  ", "platform"},
		},
	}, entity.SAMLAttributeMapping{})
	if err != nil {
		t.Fatalf("ResolveSAMLIdentity: %v", err)
	}

	if len(identity.Groups) != 2 || identity.Groups[0] != "engineering" || identity.Groups[1] != "platform" {
		t.Fatalf("groups %v, want the two non-empty values", identity.Groups)
	}
}

func TestACertificateExpiryIsCountedInWholeDays(t *testing.T) {
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)

	for name, tc := range map[string]struct {
		expiry time.Time
		want   int
	}{
		"a month away":     {now.Add(30 * 24 * time.Hour), 30},
		"tomorrow":         {now.Add(24 * time.Hour), 1},
		"later today":      {now.Add(6 * time.Hour), 0},
		"already expired":  {now.Add(-24 * time.Hour), -1},
		"long gone":        {now.Add(-40 * 24 * time.Hour), -40},
		"just under a day": {now.Add(23 * time.Hour), 0},
	} {
		if got := entity.DaysUntil(tc.expiry, now); got != tc.want {
			t.Errorf("%s: got %d days, want %d", name, got, tc.want)
		}
	}

	if !entity.CertificateExpired(now.Add(-time.Second), now) {
		t.Error("a certificate that expired a second ago does not read as expired")
	}

	if entity.CertificateExpired(now.Add(time.Hour), now) {
		t.Error("a certificate valid for another hour reads as expired")
	}
}

func TestAnAdministratorIsWarnedOnceAtEachThresholdAndNotAgain(t *testing.T) {
	for name, tc := range map[string]struct {
		daysLeft  int
		notified  *int
		wantSend  bool
		wantAtDay int
	}{
		"far away, nothing to say":            {45, nil, false, 0},
		"first crossing of thirty":            {30, nil, true, 30},
		"between thirty and fourteen":         {20, nil, true, 30},
		"already told at thirty, still there": {20, intPtr(30), false, 0},
		"now inside fourteen":                 {12, intPtr(30), true, 14},
		"already told at fourteen":            {12, intPtr(14), false, 0},
		"inside seven":                        {5, intPtr(14), true, 7},
		"the last day":                        {1, intPtr(7), true, 1},
		"expired, already told at one":        {-3, intPtr(1), false, 0},
		"expired, never told":                 {-3, nil, true, 1},
	} {
		day, send := entity.ExpiryNoticeDue(entity.SAMLExpiryNoticeDays, tc.daysLeft, tc.notified)

		if send != tc.wantSend {
			t.Errorf("%s: send = %v, want %v", name, send, tc.wantSend)

			continue
		}

		if send && day != tc.wantAtDay {
			t.Errorf("%s: threshold %d, want %d", name, day, tc.wantAtDay)
		}
	}
}

func TestTheExpiryWarningNeverGoesBackwards(t *testing.T) {
	notified := 7

	if _, send := entity.ExpiryNoticeDue(entity.SAMLExpiryNoticeDays, 20, &notified); send {
		t.Fatal(
			"an administrator already warned at seven days was warned again at thirty. A " +
				"certificate that was replaced resets the record; a clock that jumped must not " +
				"produce a second, less urgent warning.",
		)
	}
}

func TestAConditionsFailureNamesTheClocksAndBothEndsOfTheWindow(t *testing.T) {
	notBefore := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	notOnOrAfter := notBefore.Add(5 * time.Minute)
	now := notBefore.Add(30 * time.Minute)

	err := entity.SAMLConditionsFailure(notBefore, notOnOrAfter, now)

	if stage := stageOf(t, err); stage != entity.SSOStageConditions {
		t.Fatalf("stage %q, want %q", stage, entity.SSOStageConditions)
	}

	message := err.Error()

	if !strings.Contains(message, "clocks disagree") && !strings.Contains(message, "clocks") {
		t.Fatalf(
			"the refusal reads %q, which never names clock skew. That is the cause in almost "+
				"every real case and the one thing an administrator can act on.",
			message,
		)
	}

	for _, moment := range []time.Time{notBefore, notOnOrAfter, now} {
		if !strings.Contains(message, moment.Format(time.RFC3339)) {
			t.Errorf("the refusal does not carry %s, so the size of the skew is invisible", moment)
		}
	}
}

func TestAReplayFailureSaysSoRatherThanLookingLikeSomethingElse(t *testing.T) {
	err := entity.SAMLReplayFailure()

	if stage := stageOf(t, err); stage != entity.SSOStageReplay {
		t.Fatalf("stage %q, want %q", stage, entity.SSOStageReplay)
	}
}

func TestProviderMetadataMustCarryEnoughToTrustIt(t *testing.T) {
	complete := entity.SAMLDescriptor{
		EntityID:     "https://login.example.com/realms/norn",
		SSOURL:       "https://login.example.com/protocol/saml",
		Certificates: []string{"MIIC..."},
	}

	if err := complete.Validate(); err != nil {
		t.Fatalf("complete metadata was rejected: %v", err)
	}

	for name, tc := range map[string]struct {
		mutate func(*entity.SAMLDescriptor)
		stage  entity.SSOStage
	}{
		"no entity id":       {func(d *entity.SAMLDescriptor) { d.EntityID = "" }, entity.SSOStageMetadata},
		"no sign-in url":     {func(d *entity.SAMLDescriptor) { d.SSOURL = "" }, entity.SSOStageMetadata},
		"plain http sign-in": {func(d *entity.SAMLDescriptor) { d.SSOURL = "http://login.example.com/x" }, entity.SSOStageMetadata},
		"no certificate": {
			func(d *entity.SAMLDescriptor) { d.Certificates = nil },
			entity.SSOStageCertificate,
		},
	} {
		broken := complete
		tc.mutate(&broken)

		err := broken.Validate()
		if err == nil {
			t.Errorf("%s: accepted", name)

			continue
		}

		if stage := stageOf(t, err); stage != tc.stage {
			t.Errorf("%s: stage %q, want %q", name, stage, tc.stage)
		}
	}
}

func intPtr(value int) *int { return &value }
