package entity_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestARecoveryCodeIsReadableAloudAndMatchesHoweverItIsTypedBack(t *testing.T) {
	code, hash, err := entity.NewBreakGlassCode()
	if err != nil {
		t.Fatalf("NewBreakGlassCode: %v", err)
	}

	if len(hash) != 32 {
		t.Fatalf("hash is %d bytes, want a SHA-256 digest", len(hash))
	}

	if strings.ContainsAny(code, "01IO") {
		t.Fatalf(
			"the code %q contains a character that is read back wrongly over the phone. This is "+
				"used exactly once, under pressure, by somebody locked out.",
			code,
		)
	}

	for name, typed := range map[string]string{
		"as shown":         code,
		"lower case":       strings.ToLower(code),
		"without dashes":   strings.ReplaceAll(code, "-", ""),
		"with whitespace":  "  " + code + "  ",
		"pasted lowercase": strings.ToLower(strings.ReplaceAll(code, "-", "")),
	} {
		if got := entity.HashBreakGlassCode(typed); string(got) != string(hash) {
			t.Errorf("%s: the same code did not match its own hash", name)
		}
	}
}

func TestTwoRecoveryCodesAreNeverTheSame(t *testing.T) {
	seen := make(map[string]struct{}, 64)

	for range 64 {
		code, _, err := entity.NewBreakGlassCode()
		if err != nil {
			t.Fatalf("NewBreakGlassCode: %v", err)
		}

		if _, repeated := seen[code]; repeated {
			t.Fatalf("code %q was issued twice", code)
		}

		seen[code] = struct{}{}
	}
}

func TestAWrongCodeDoesNotMatch(t *testing.T) {
	_, hash, err := entity.NewBreakGlassCode()
	if err != nil {
		t.Fatalf("NewBreakGlassCode: %v", err)
	}

	if string(entity.HashBreakGlassCode("AAAA-BBBB-CCCC")) == string(hash) {
		t.Fatal("an unrelated code matched")
	}
}

func TestEachRefusalToRequireSingleSignOnSaysWhichThingIsMissing(t *testing.T) {
	seen := make(map[string]struct{}, 3)

	for _, blocker := range []entity.EnforcementBlocker{
		entity.EnforcementBlockerNoConnection,
		entity.EnforcementBlockerNotVerified,
		entity.EnforcementBlockerNoLinkedAdmin,
	} {
		err := entity.EnforcementRefusedError{Blocker: blocker}

		if !errors.Is(err, entity.ErrEnforcementRefused) {
			t.Errorf("%q does not read as a refusal to enforce", blocker)
		}

		message := err.Error()
		if _, repeated := seen[message]; repeated {
			t.Errorf("%q reuses another blocker's wording, so an admin cannot tell them apart", blocker)
		}

		seen[message] = struct{}{}
	}
}

func TestALinkedIdentityOnlyMatchesItsOwnSubject(t *testing.T) {
	const issuer = "https://login.example.com"

	linked := &entity.SSOIdentity{Issuer: issuer, Subject: "provider-subject-1"}

	if err := entity.MatchLink(linked, issuer, "provider-subject-1", "ada@example.com"); err != nil {
		t.Fatalf("the linked subject was refused: %v", err)
	}

	if err := entity.MatchLink(nil, issuer, "anything", "ada@example.com"); err != nil {
		t.Fatalf("an account with no link yet was refused: %v", err)
	}

	if err := entity.MatchLink(
		linked,
		"https://login.elsewhere.example.com",
		"provider-subject-2",
		"ada@example.com",
	); err != nil {
		t.Fatalf(
			"a link left behind by a provider this workspace no longer uses blocked the new one, "+
				"so repointing the connection would lock everybody out: %v",
			err,
		)
	}

	err := entity.MatchLink(linked, issuer, "provider-subject-2", "Ada@Example.com")
	if err == nil {
		t.Fatal(
			"a different provider identity was accepted for an account that is already linked. " +
				"Anyone who can get that email address issued at the provider would inherit the " +
				"Norn account.",
		)
	}

	if stage := stageOf(t, err); stage != entity.SSOStageMatching {
		t.Fatalf("stage %q, want %q", stage, entity.SSOStageMatching)
	}

	if !strings.Contains(err.Error(), "ada@example.com") {
		t.Errorf("the refusal does not name the address: %v", err)
	}
}
