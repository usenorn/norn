package entity_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func stageOf(t *testing.T, err error) entity.SSOStage {
	t.Helper()

	failure, ok := entity.AsSSOError(err)
	if !ok {
		t.Fatalf("error %v is not an OIDC failure, so no screen can say where it broke", err)
	}

	return failure.Stage
}

func TestEveryStageIsDistinguishableFromTheOthers(t *testing.T) {
	stages := []entity.SSOStage{
		entity.SSOStageDiscovery,
		entity.SSOStageEndpoints,
		entity.SSOStageJWKS,
		entity.SSOStageAuthorization,
		entity.SSOStageTokenExchange,
		entity.SSOStageIDToken,
		entity.SSOStageClaims,
		entity.SSOStageMatching,
		entity.SSOStageProvisioning,
	}

	seen := make(map[entity.SSOStage]struct{}, len(stages))

	for _, stage := range stages {
		if !stage.Valid() {
			t.Errorf("stage %q is not accepted by Valid, so it cannot cross the API", stage)
		}

		if _, repeated := seen[stage]; repeated {
			t.Errorf("stage %q appears twice, so two failures would be reported as one", stage)
		}

		seen[stage] = struct{}{}

		if got := stageOf(t, entity.NewSSOError(stage, "something went wrong")); got != stage {
			t.Errorf("a %q failure reported itself as %q", stage, got)
		}
	}

	if len(seen) != 9 {
		t.Fatalf("expected nine distinct stages, got %d", len(seen))
	}

	for _, notAStage := range []entity.SSOStage{"", "network", "TOKEN_EXCHANGE"} {
		if notAStage.Valid() {
			t.Errorf("%q was accepted as a stage", notAStage)
		}
	}
}

func TestAFailureKeepsWhatTheProviderActuallySaid(t *testing.T) {
	cause := errors.New("invalid_client: client secret mismatch")
	failure := entity.SSOFailure(entity.SSOStageTokenExchange, "The provider refused the exchange.", cause)

	if failure.Detail != cause.Error() {
		t.Fatalf(
			"detail = %q, want the provider's own words. Without them an admin cannot tell a "+
				"wrong secret from a wrong redirect URI.",
			failure.Detail,
		)
	}

	if !errors.Is(failure, cause) {
		t.Fatal("the underlying error is not reachable through errors.Is")
	}

	if !strings.Contains(failure.Error(), "token_exchange") {
		t.Fatalf("Error() = %q, want the stage named in it", failure.Error())
	}
}

func TestClaimsMustCarryASubjectAndAnEmail(t *testing.T) {
	verified, unverified := true, false

	for name, tc := range map[string]struct {
		claims entity.OIDCClaims
		accept bool
	}{
		"complete and verified": {
			claims: entity.OIDCClaims{Subject: "abc", Email: "ada@example.com", EmailVerified: &verified},
			accept: true,
		},
		"no email_verified claim at all": {
			claims: entity.OIDCClaims{Subject: "abc", Email: "ada@example.com"},
			accept: true,
		},
		"provider says the address is unverified": {
			claims: entity.OIDCClaims{Subject: "abc", Email: "ada@example.com", EmailVerified: &unverified},
			accept: false,
		},
		"no email": {
			claims: entity.OIDCClaims{Subject: "abc", EmailVerified: &verified},
			accept: false,
		},
		"no subject": {
			claims: entity.OIDCClaims{Email: "ada@example.com", EmailVerified: &verified},
			accept: false,
		},
	} {
		err := entity.ValidateClaims(tc.claims)

		if tc.accept {
			if err != nil {
				t.Errorf("%s: rejected with %v", name, err)
			}

			continue
		}

		if err == nil {
			t.Errorf("%s: accepted", name)

			continue
		}

		if stage := stageOf(t, err); stage != entity.SSOStageClaims {
			t.Errorf("%s: failed at stage %q, want %q", name, stage, entity.SSOStageClaims)
		}
	}
}

func TestAnUnverifiedAddressIsNamedInTheRefusal(t *testing.T) {
	unverified := false

	err := entity.ValidateClaims(entity.OIDCClaims{
		Subject:       "abc",
		Email:         "Ada@Example.COM",
		EmailVerified: &unverified,
	})
	if err == nil {
		t.Fatal("an unverified address was accepted")
	}

	if !strings.Contains(err.Error(), "ada@example.com") {
		t.Fatalf(
			"refusal %q does not name the address. The admin has to know which identity the "+
				"provider would not vouch for.",
			err,
		)
	}
}

func TestWhoGetsInAndWhoDoesNot(t *testing.T) {
	for name, tc := range map[string]struct {
		accountExists bool
		isMember      bool
		provisioning  bool
		want          entity.MatchOutcome
	}{
		"a member signs in":                          {true, true, false, entity.MatchOutcomeSignIn},
		"a member signs in with provisioning on":     {true, true, true, entity.MatchOutcomeSignIn},
		"an account that is not a member is refused": {true, false, false, entity.MatchOutcomeNotMember},
		"provisioning does not admit a non-member":   {true, false, true, entity.MatchOutcomeNotMember},
		"an unknown address is provisioned":          {false, false, true, entity.MatchOutcomeProvision},
		"an unknown address without JIT is refused":  {false, false, false, entity.MatchOutcomeNoAccount},
	} {
		got := entity.ResolveMatch(tc.accountExists, tc.isMember, tc.provisioning)
		if got != tc.want {
			t.Errorf("%s: got %q, want %q", name, got, tc.want)
		}
	}
}

func TestProvisioningNeverPromotesSomeoneWhoWasAlreadyTurnedAway(t *testing.T) {
	off := entity.ResolveMatch(true, false, false)
	on := entity.ResolveMatch(true, false, true)

	if off != on {
		t.Fatalf(
			"turning provisioning on changed an existing non-member from %q to %q. "+
				"Just-in-time provisioning creates accounts; it must never widen who may enter "+
				"a workspace they were not invited to.",
			off, on,
		)
	}
}

func TestARefusalNamesTheStageThatMatchesTheReason(t *testing.T) {
	notMember := entity.MatchOutcomeNotMember.Refusal("Ada@Example.com")
	if stage := stageOf(t, notMember); stage != entity.SSOStageMatching {
		t.Errorf("a non-member refusal reported stage %q, want %q", stage, entity.SSOStageMatching)
	}

	if !strings.Contains(notMember.Error(), "ada@example.com") {
		t.Errorf("the non-member refusal does not name the address: %v", notMember)
	}

	noAccount := entity.MatchOutcomeNoAccount.Refusal("ada@example.com")
	if stage := stageOf(t, noAccount); stage != entity.SSOStageProvisioning {
		t.Errorf("a missing-account refusal reported stage %q, want %q", stage, entity.SSOStageProvisioning)
	}

	for _, admitted := range []entity.MatchOutcome{entity.MatchOutcomeSignIn, entity.MatchOutcomeProvision} {
		if !admitted.Admits() {
			t.Errorf("%q does not admit", admitted)
		}

		if err := admitted.Refusal("ada@example.com"); err != nil {
			t.Errorf("%q produced a refusal %v", admitted, err)
		}
	}

	for _, refused := range []entity.MatchOutcome{entity.MatchOutcomeNotMember, entity.MatchOutcomeNoAccount} {
		if refused.Admits() {
			t.Errorf("%q admits", refused)
		}
	}
}

func TestOpenidIsAlwaysRequested(t *testing.T) {
	for name, tc := range map[string]struct {
		given []string
		want  []string
	}{
		"nothing given falls back to the defaults": {nil, entity.DefaultOIDCScopes},
		"empty entries are dropped":                {[]string{"", "  ", "email"}, []string{"openid", "email"}},
		"openid is added when left out":            {[]string{"email", "groups"}, []string{"openid", "email", "groups"}},
		"openid is not duplicated":                 {[]string{"openid", "email"}, []string{"openid", "email"}},
		"repeats are collapsed":                    {[]string{"openid", "email", "email"}, []string{"openid", "email"}},
	} {
		got := entity.NormalizeOIDCScopes(tc.given)

		if len(got) != len(tc.want) {
			t.Errorf("%s: got %v, want %v", name, got, tc.want)

			continue
		}

		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%s: got %v, want %v", name, got, tc.want)

				break
			}
		}
	}
}

func TestNormalizingScopesDoesNotAliasTheSharedDefault(t *testing.T) {
	first := entity.NormalizeOIDCScopes(nil)
	first[0] = "tampered"

	if entity.NormalizeOIDCScopes(nil)[0] != "openid" {
		t.Fatal(
			"the defaults are handed out by reference, so one workspace editing its scopes " +
				"would silently change every other workspace's.",
		)
	}
}

func TestAProviderAddressMustBeReachableOverTLS(t *testing.T) {
	for name, tc := range map[string]struct {
		issuer string
		accept bool
	}{
		"an https issuer":                {"https://login.example.com/realms/norn", true},
		"loopback over http for dev":     {"http://localhost:8081/realms/norn", true},
		"docker host over http for dev":  {"http://host.docker.internal:8081/realms/norn", true},
		"plain http to a public host":    {"http://login.example.com", false},
		"a bare hostname with no scheme": {"login.example.com", false},
		"nothing at all":                 {"", false},
		"whitespace":                     {"   ", false},
	} {
		err := entity.ValidateOIDCIssuer(tc.issuer)

		if tc.accept && err != nil {
			t.Errorf("%s: rejected %q with %v", name, tc.issuer, err)
		}

		if !tc.accept {
			if err == nil {
				t.Errorf("%s: accepted %q", name, tc.issuer)

				continue
			}

			if stage := stageOf(t, err); stage != entity.SSOStageDiscovery {
				t.Errorf("%s: reported stage %q, want %q", name, stage, entity.SSOStageDiscovery)
			}
		}
	}
}

func TestManuallyEnteredEndpointsAreCheckedTheSameWayDiscoveredOnesAre(t *testing.T) {
	complete := entity.OIDCEndpoints{
		Issuer:                "https://login.example.com",
		AuthorizationEndpoint: "https://login.example.com/auth",
		TokenEndpoint:         "https://login.example.com/token",
		JWKSURI:               "https://login.example.com/certs",
	}

	if err := complete.Validate(); err != nil {
		t.Fatalf("a complete set of endpoints was rejected: %v", err)
	}

	for name, mutate := range map[string]func(*entity.OIDCEndpoints){
		"no authorization endpoint": func(e *entity.OIDCEndpoints) { e.AuthorizationEndpoint = "" },
		"no token endpoint":         func(e *entity.OIDCEndpoints) { e.TokenEndpoint = "" },
		"no jwks uri":               func(e *entity.OIDCEndpoints) { e.JWKSURI = "" },
		"a token endpoint over plain http": func(e *entity.OIDCEndpoints) {
			e.TokenEndpoint = "http://login.example.com/token"
		},
	} {
		broken := complete
		mutate(&broken)

		err := broken.Validate()
		if err == nil {
			t.Errorf("%s: accepted", name)

			continue
		}

		if stage := stageOf(t, err); stage != entity.SSOStageEndpoints {
			t.Errorf("%s: reported stage %q, want %q", name, stage, entity.SSOStageEndpoints)
		}
	}
}

func TestAConnectionIsOnlyUsableOnceItHasBeenTested(t *testing.T) {
	connection := entity.OIDCConnection{
		Endpoints: entity.OIDCEndpoints{
			Issuer:                "https://login.example.com",
			AuthorizationEndpoint: "https://login.example.com/auth",
			TokenEndpoint:         "https://login.example.com/token",
			JWKSURI:               "https://login.example.com/certs",
		},
		ClientID:     "norn",
		ClientSecret: "s3cr3t",
	}

	if err := connection.Validate(); err != nil {
		t.Fatalf("a complete connection was rejected: %v", err)
	}

	if connection.Verified() {
		t.Fatal("a connection reports itself verified before any test has run")
	}

	withoutID := connection
	withoutID.ClientID = ""

	if err := withoutID.Validate(); err == nil {
		t.Error("a connection with no client id was accepted")
	}

	withoutSecret := connection
	withoutSecret.ClientSecret = ""

	if err := withoutSecret.Validate(); err == nil {
		t.Error("a connection with no client secret was accepted")
	}
}

func TestBothPurposesAreRecognisedAndNothingElseIs(t *testing.T) {
	for _, purpose := range []entity.SSOPurpose{entity.SSOPurposeLogin, entity.SSOPurposeTest} {
		if !purpose.Valid() {
			t.Errorf("%q is not accepted as a purpose", purpose)
		}
	}

	for _, purpose := range []entity.SSOPurpose{"", "signup", "LOGIN"} {
		if purpose.Valid() {
			t.Errorf("%q was accepted as a purpose", purpose)
		}
	}
}
