package config_test

import (
	"testing"

	"github.com/usenorn/norn/internal/config"
)

func TestTheDocumentedGitHubAppEnvironmentVariablesAreActuallyRead(t *testing.T) {
	t.Setenv("NORN_SOURCE_CONTROL_GITHUB_APP_ID", "4562764")
	t.Setenv("NORN_SOURCE_CONTROL_GITHUB_APP_SLUG", "nornbot")
	t.Setenv("NORN_SOURCE_CONTROL_GITHUB_APP_PRIVATE_KEY", "-----BEGIN RSA PRIVATE KEY-----\nx\n-----END RSA PRIVATE KEY-----\n")
	t.Setenv("NORN_SOURCE_CONTROL_GITHUB_APP_WEBHOOK_SECRET", "shhh")
	t.Setenv("NORN_SECURITY_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")

	cfg, err := config.New("")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if cfg.SourceControl.GitHubAppSlug != "nornbot" {
		t.Errorf("slug = %q, want the value the environment set", cfg.SourceControl.GitHubAppSlug)
	}

	if !cfg.SourceControl.GitHubAppConfigured() {
		t.Fatal(
			"the six documented NORN_SOURCE_CONTROL_GITHUB_APP_* variables were set and the " +
				"instance still reports no application. An operator following the docs gets " +
				"silence: no error, no warning, and a screen saying no app is held.",
		)
	}
}
