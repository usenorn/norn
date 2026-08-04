package ssoconnection_test

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func source(t *testing.T) string {
	t.Helper()

	body, err := os.ReadFile("postgres.go")
	if err != nil {
		t.Fatalf("read postgres.go: %v", err)
	}

	return string(body)
}

func TestSavingAnyProviderClearsAnEarlierVerification(t *testing.T) {
	body := source(t)

	upsert := between(t, body, "const upsertParentQuery = `", "`\n")

	inserted, update, found := strings.Cut(upsert, "DO UPDATE SET")
	if !found {
		t.Fatal("the parent upsert has no update branch, so re-saving would do nothing")
	}

	if !strings.Contains(update, "verified_at  = NULL") {
		t.Fatal(
			"re-saving a provider does not reset verified_at. An administrator could repoint a " +
				"verified connection at a different issuer, certificate or client and it would " +
				"still read as tested — which is exactly the state enforcement gates on.",
		)
	}

	if !strings.Contains(inserted, "NULL") {
		t.Fatal("a newly inserted connection is not created unverified")
	}
}

func TestOnlyMarkVerifiedEverSetsVerifiedAtToATime(t *testing.T) {
	body := source(t)

	assignments := regexp.MustCompile(`verified_at\s*=\s*([^,\n]+)`).FindAllStringSubmatch(body, -1)
	if len(assignments) == 0 {
		t.Fatal("nothing in this repository writes verified_at")
	}

	timed := 0

	for _, assignment := range assignments {
		value := strings.TrimSpace(assignment[1])
		if value == "NULL" {
			continue
		}

		timed++

		if value != "$2" {
			t.Errorf("verified_at is set to %q outside MarkVerified", value)
		}
	}

	if timed != 1 {
		t.Fatalf("verified_at is given a real time in %d places, want exactly one", timed)
	}
}

func TestEverySecretIsSealedOnTheWayInAndOpenedOnTheWayOut(t *testing.T) {
	body := source(t)

	for _, plaintext := range []string{
		"connection.ClientSecret,",
		"connection.SPPrivateKey,",
	} {
		if strings.Contains(body, plaintext) {
			t.Errorf(
				"%s is passed straight to a query. Both the OIDC client secret and the SAML "+
					"private key must go through crypter.Seal, or a database dump hands over "+
					"working credentials.",
				strings.TrimSuffix(plaintext, ","),
			)
		}
	}

	if !strings.Contains(body, "r.seal([]byte(connection.ClientSecret))") {
		t.Error("the OIDC client secret is not sealed")
	}

	if !strings.Contains(body, "r.seal(connection.SPPrivateKey)") {
		t.Error("the SAML private key is not sealed")
	}

	if strings.Count(body, "r.open(sealed)") != 2 {
		t.Error("both protocols must unseal what they stored")
	}
}

func TestChangingProtocolRemovesTheOtherProvidersRow(t *testing.T) {
	body := source(t)

	claim := between(t, body, "func (r *connectionRepository) claim(", "\n}\n")

	if !strings.Contains(claim, "held != protocol") {
		t.Fatal("saving does not notice that the workspace currently holds the other protocol")
	}

	if !strings.Contains(claim, "deleteParentQuery") {
		t.Fatal(
			"switching protocol does not delete the parent row. The detail tables hang off it by " +
				"a composite foreign key, so without that delete the old provider's row survives " +
				"and a workspace ends up with two.",
		)
	}
}

func TestAMissingEncryptionKeyBecomesADomainErrorRatherThanACrash(t *testing.T) {
	body := source(t)

	if strings.Count(body, "crypter.ErrKeyMissing") != 2 {
		t.Fatal(
			"sealing and opening must both translate a missing encryption key, or an install " +
				"that never set one gets a bare 500 naming nothing.",
		)
	}

	if !strings.Contains(body, "entity.ErrSSOEncryptionKeyMissing") {
		t.Error("the domain error that names the setting is not returned")
	}
}

func between(t *testing.T, body, opening, closing string) string {
	t.Helper()

	start := strings.Index(body, opening)
	if start < 0 {
		t.Fatalf("could not find %q in postgres.go", opening)
	}

	start += len(opening)

	end := strings.Index(body[start:], closing)
	if end < 0 {
		t.Fatalf("could not find %q after %q in postgres.go", closing, opening)
	}

	return body[start : start+end]
}
