package config_test

import (
	"testing"

	"github.com/usenorn/norn/internal/config"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
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

func TestAMinimumRunnerVersionTheServerCannotCompareIsRefused(t *testing.T) {
	t.Setenv("NORN_RUNNER_MINIMUM_VERSION", "latest")

	if _, err := config.New(""); err == nil {
		t.Fatal("config.New() accepted a minimum runner version it can never compare against")
	}
}

func TestAServerDefaultsToAMinimumRunnerVersionItCanCompare(t *testing.T) {
	cfg, err := config.New("")
	if err != nil {
		t.Fatalf("config.New() = %v, want the defaults to be a working configuration", err)
	}

	if !channelv1.Released(cfg.Runner.MinimumVersion) {
		t.Fatalf(
			"the default runner floor is %q, which no runner version can be compared against",
			cfg.Runner.MinimumVersion,
		)
	}
}
