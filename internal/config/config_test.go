package config_test

import (
	"testing"

	"github.com/usenorn/norn/internal/config"
)

func TestTrustedProxiesAcceptBareAddressesAndCIDRBlocks(t *testing.T) {
	cfg := config.HTTP{TrustedProxies: []string{"127.0.0.1", " 10.0.0.0/8 ", "::1", ""}}

	prefixes, err := cfg.TrustedPrefixes()
	if err != nil {
		t.Fatalf("TrustedPrefixes() = %v, want no error", err)
	}

	want := []string{"127.0.0.1/32", "10.0.0.0/8", "::1/128"}

	if len(prefixes) != len(want) {
		t.Fatalf("TrustedPrefixes() = %v, want %v", prefixes, want)
	}

	for i, prefix := range prefixes {
		if prefix.String() != want[i] {
			t.Fatalf("TrustedPrefixes()[%d] = %s, want %s", i, prefix, want[i])
		}
	}
}

func TestATrustedProxyThatIsNotAnAddressIsRejected(t *testing.T) {
	cfg := config.HTTP{TrustedProxies: []string{"127.0.0.1", "not-an-address"}}

	if _, err := cfg.TrustedPrefixes(); err == nil {
		t.Fatal("TrustedPrefixes() = nil error, want a rejection of the malformed entry")
	}
}
