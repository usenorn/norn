package oidcconnection_test

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

func TestSavingAConnectionAlwaysClearsAnEarlierVerification(t *testing.T) {
	body := source(t)

	upsert := between(t, body, "const upsertConnectionQuery = `", "`\n")

	inserted, update, found := strings.Cut(upsert, "DO UPDATE SET")
	if !found {
		t.Fatal("the upsert has no update branch, so re-saving a connection would do nothing")
	}

	if !strings.Contains(update, "verified_at            = NULL") {
		t.Fatal(
			"re-saving a connection does not reset verified_at. An admin could point a verified " +
				"connection at a different issuer or client and it would still read as tested, " +
				"which is exactly the state enforcement is meant to gate on.",
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
			t.Errorf(
				"verified_at is set to %q somewhere other than MarkVerified's parameter. Every "+
					"other write must clear it.",
				value,
			)
		}
	}

	if timed != 1 {
		t.Fatalf(
			"verified_at is set to a real time in %d places, want exactly one (MarkVerified)",
			timed,
		)
	}
}

func TestTheStoredSecretIsOnlyEverWrittenSealedAndReadUnsealed(t *testing.T) {
	body := source(t)

	if strings.Contains(body, "connection.ClientSecret,") {
		t.Fatal(
			"the plaintext client secret is passed to a query. It must go through crypter.Seal " +
				"so a database dump does not hand over working provider credentials.",
		)
	}

	if !strings.Contains(body, "r.crypter.Seal([]byte(connection.ClientSecret))") {
		t.Error("Save does not seal the client secret before writing it")
	}

	if !strings.Contains(body, "r.crypter.Open(sealed)") {
		t.Error("reads do not unseal the stored secret")
	}
}

func TestEveryColumnTheUpsertWritesIsAlsoReadBack(t *testing.T) {
	body := source(t)

	columns := splitColumns(between(t, body, "const connectionColumns = `", "`"))

	insert := splitColumns(between(
		t,
		body,
		"INSERT INTO workspace_oidc_connections (",
		")\nVALUES",
	))

	if len(columns) != len(insert) {
		t.Fatalf(
			"the insert writes %d columns but reads back %d. A column written and never read is "+
				"data silently discarded on the next load.",
			len(insert), len(columns),
		)
	}

	for i := range columns {
		if columns[i] != insert[i] {
			t.Errorf("column %d: insert writes %q, the read list has %q", i, insert[i], columns[i])
		}
	}

	values := splitColumns(between(t, body, "VALUES (", ")\nON CONFLICT"))
	if len(values) != len(insert) {
		t.Fatalf(
			"the insert names %d columns but supplies %d values, so the arguments are shifted",
			len(insert), len(values),
		)
	}

	distinct := make(map[string]struct{}, len(values))

	for _, value := range values {
		if strings.HasPrefix(value, "$") {
			distinct[value] = struct{}{}
		}
	}

	if len(distinct) != 13 {
		t.Fatalf(
			"the VALUES list binds %d distinct placeholders; verified_at is a literal NULL and "+
				"created_at and updated_at share one timestamp, so 13 is the only correct count",
			len(distinct),
		)
	}
}

func splitColumns(list string) []string {
	parts := strings.Split(list, ",")
	columns := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			columns = append(columns, trimmed)
		}
	}

	return columns
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

func TestAnInstanceWithoutAnEncryptionKeyGetsATellingErrorNotACrash(t *testing.T) {
	body := source(t)

	if !strings.Contains(body, "crypter.ErrKeyMissing") {
		t.Fatal(
			"a missing encryption key is not translated into a domain error, so an install " +
				"that never set one gets a bare 500 with nothing naming the setting to change.",
		)
	}

	if strings.Count(body, "entity.ErrOIDCEncryptionKeyMissing") < 2 {
		t.Error("only one of sealing and opening translates the missing key")
	}
}
