package entity_test

import (
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestTheBreachDigestSplitsTheHashIntoAPrefixAndItsRemainder(t *testing.T) {
	prefix, suffix := entity.PasswordBreachDigest("password")

	if prefix != "5BAA6" {
		t.Fatalf("prefix = %q, want 5BAA6", prefix)
	}

	if suffix != "1E4C9B93F3F0682250B6CF8331B7EE68FD8" {
		t.Fatalf("suffix = %q, want the remaining 35 characters of the sha-1", suffix)
	}
}

func TestTheBreachDigestIsAlwaysAFiveCharacterPrefix(t *testing.T) {
	for _, password := range []string{"", "a", "correct horse battery staple"} {
		prefix, suffix := entity.PasswordBreachDigest(password)

		if len(prefix) != entity.PasswordBreachPrefixLen {
			t.Fatalf("prefix for %q has length %d, want %d", password, len(prefix), entity.PasswordBreachPrefixLen)
		}

		if len(prefix)+len(suffix) != 40 {
			t.Fatalf("prefix and suffix for %q total %d characters, want 40", password, len(prefix)+len(suffix))
		}
	}
}
