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

func executionLimits(t *testing.T, artifact string) error {
	t.Helper()

	t.Setenv("NORN_SECURITY_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("NORN_SMTP_HOST", "smtp.test")
	t.Setenv("NORN_SMTP_FROM_ADDRESS", "no-reply@norn.test")
	t.Setenv("NORN_HTTP_MAX_REQUEST_BYTES", "4194304")
	t.Setenv("NORN_EXECUTIONS_MAX_ARTIFACT_BYTES", artifact)

	_, err := config.New("")

	return err
}

func TestAnArtifactLimitAtTheRequestCapIsRefusedBecauseTheEnvelopeNeedsRoom(t *testing.T) {
	err := executionLimits(t, "4194304")
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

func TestLimitsThatLeaveRoomAreAccepted(t *testing.T) {
	if err := executionLimits(t, "3145728"); err != nil {
		t.Fatalf("config.New() = %v, want limits that leave room to be accepted", err)
	}
}

func hostingLimits(t *testing.T, timeout, outputTokens, totalTokens string) error {
	t.Helper()

	t.Setenv("NORN_SECURITY_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("NORN_SMTP_HOST", "smtp.test")
	t.Setenv("NORN_SMTP_FROM_ADDRESS", "no-reply@norn.test")
	t.Setenv("NORN_HTTP_REQUEST_TIMEOUT", "25s")
	t.Setenv("NORN_AGENT_HOSTING_TIMEOUT", timeout)
	t.Setenv("NORN_AGENT_HOSTING_MAX_OUTPUT_TOKENS", outputTokens)
	t.Setenv("NORN_AGENT_HOSTING_MAX_TOTAL_TOKENS", totalTokens)

	_, err := config.New("")

	return err
}

func TestAHostedConversationMustFinishInsideTheRequestThatAskedIt(t *testing.T) {
	err := hostingLimits(t, "25s", "2048", "60000")
	if err == nil || !strings.Contains(err.Error(), "agent_hosting.timeout") {
		t.Fatalf(
			"a hosted timeout as long as the request answered %v; the request would be cut off "+
				"before the conversation could return what it had",
			err,
		)
	}
}

func TestAHostedBudgetSmallerThanOneAnswerIsRefused(t *testing.T) {
	err := hostingLimits(t, "20s", "2048", "1024")
	if err == nil || !strings.Contains(err.Error(), "agent_hosting.max_total_tokens") {
		t.Fatalf("a total budget below one answer's budget answered %v", err)
	}
}

func TestTheDefaultHostingLimitsAreAWorkingConfiguration(t *testing.T) {
	if err := hostingLimits(t, "20s", "16384", "200000"); err != nil {
		t.Fatalf("config.New() = %v, want the default hosting limits accepted", err)
	}
}
