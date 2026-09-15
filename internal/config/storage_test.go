package config_test

import (
	"testing"

	"github.com/usenorn/norn/internal/config"
)

func storageInstance(t *testing.T, selfHosted string) {
	t.Helper()

	t.Setenv("NORN_SECURITY_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("NORN_SMTP_HOST", "smtp.test")
	t.Setenv("NORN_SMTP_FROM_ADDRESS", "no-reply@norn.test")
	t.Setenv("NORN_INSTANCE_SELF_HOSTED", selfHosted)
	t.Setenv("NORN_ATTACHMENTS_MAX_WORKSPACE_BYTES", "")
}

func TestACloudWorkspaceIsLimitedToOneGibibyteByDefault(t *testing.T) {
	storageInstance(t, "false")

	cfg, err := config.New("")
	if err != nil {
		t.Fatalf("config: %v", err)
	}

	if cfg.Attachments.MaxWorkspaceBytes != 1_073_741_824 {
		t.Fatalf(
			"a cloud workspace is limited to %d bytes by default, want 1073741824. Every "+
				"workspace on the free plan gets 1 GB, and nothing else sets it.",
			cfg.Attachments.MaxWorkspaceBytes,
		)
	}
}

func TestASelfHostedWorkspaceHasNoLimitByDefault(t *testing.T) {
	storageInstance(t, "true")

	cfg, err := config.New("")
	if err != nil {
		t.Fatalf("config: %v", err)
	}

	if cfg.Attachments.MaxWorkspaceBytes != 0 {
		t.Fatalf(
			"a self-hosted workspace is limited to %d bytes by default. The disk belongs to the "+
				"operator, so a limit is theirs to switch on, not a cloud plan carried over.",
			cfg.Attachments.MaxWorkspaceBytes,
		)
	}
}

func TestAnOperatorsLimitWinsOnEitherKindOfInstance(t *testing.T) {
	for name, instance := range map[string]struct {
		selfHosted string
		limit      string
		want       int64
	}{
		"a self-hosted instance that switches one on": {"true", "5368709120", 5_368_709_120},
		"a cloud instance that lifts it":              {"false", "0", 0},
	} {
		t.Run(name, func(t *testing.T) {
			storageInstance(t, instance.selfHosted)
			t.Setenv("NORN_ATTACHMENTS_MAX_WORKSPACE_BYTES", instance.limit)

			cfg, err := config.New("")
			if err != nil {
				t.Fatalf("config: %v", err)
			}

			if cfg.Attachments.MaxWorkspaceBytes != instance.want {
				t.Fatalf(
					"%s ended up with %d bytes, want %d. A value the operator set must never be "+
						"replaced by the default for the kind of instance.",
					name, cfg.Attachments.MaxWorkspaceBytes, instance.want,
				)
			}
		})
	}
}
