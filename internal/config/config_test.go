package config_test

import (
	"strings"
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
	t.Setenv("NORN_SECURITY_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("NORN_SMTP_HOST", "smtp.test")
	t.Setenv("NORN_SMTP_FROM_ADDRESS", "no-reply@norn.test")

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

func executionLimits(t *testing.T, chunk, artifact string) error {
	t.Helper()

	t.Setenv("NORN_SECURITY_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("NORN_SMTP_HOST", "smtp.test")
	t.Setenv("NORN_SMTP_FROM_ADDRESS", "no-reply@norn.test")
	t.Setenv("NORN_HTTP_MAX_REQUEST_BYTES", "4194304")
	t.Setenv("NORN_EXECUTIONS_MAX_CHUNK_BYTES", chunk)
	t.Setenv("NORN_EXECUTIONS_MAX_ARTIFACT_BYTES", artifact)

	_, err := config.New("")

	return err
}

func TestAnArtifactLimitAtTheRequestCapIsRefusedBecauseTheEnvelopeNeedsRoom(t *testing.T) {
	err := executionLimits(t, "1048576", "4194304")
	if err == nil {
		t.Fatal(
			"an artifact limit equal to the request cap started. The multipart envelope sits " +
				"above the file, so the transport cuts the upload off first and answers without " +
				"naming the file that was too big.",
		)
	}

	if !strings.Contains(err.Error(), "executions.max_artifact_bytes") {
		t.Errorf("error = %v, want it to name the setting that is wrong", err)
	}
}

func TestABatchLimitAboveTheRequestCapIsRefused(t *testing.T) {
	err := executionLimits(t, "8388608", "1048576")
	if err == nil || !strings.Contains(err.Error(), "executions.max_chunk_bytes") {
		t.Fatalf("a batch limit above the request cap answered %v", err)
	}
}

func TestLimitsThatLeaveRoomAreAccepted(t *testing.T) {
	if err := executionLimits(t, "1048576", "3145728"); err != nil {
		t.Fatalf("config.New() = %v, want limits that leave room to be accepted", err)
	}
}
