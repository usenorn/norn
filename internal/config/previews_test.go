package config_test

import (
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/config"
)

func previewDomain(t *testing.T, app, previews string) error {
	t.Helper()

	t.Setenv("NORN_SECURITY_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("NORN_SMTP_HOST", "smtp.test")
	t.Setenv("NORN_SMTP_FROM_ADDRESS", "no-reply@norn.test")
	t.Setenv("NORN_SESSION_SECURE", "true")
	t.Setenv("NORN_APP_BASE_URL", app)
	t.Setenv("NORN_PREVIEWS_BASE_DOMAIN", previews)

	_, err := config.New("")

	return err
}

func TestAPreviewDomainUnderTheAppsOwnDomainIsRefused(t *testing.T) {
	cases := map[string]struct {
		app      string
		previews string
	}{
		"the same host exactly":   {app: "https://norn.site", previews: "norn.site"},
		"a preview under the app": {app: "https://norn.site", previews: "preview.norn.site"},
		"the app under a preview": {app: "https://app.norn.site", previews: "norn.site"},
		"the app carrying a port": {app: "https://norn.site:8443", previews: "norn.site"},
	}

	for name, given := range cases {
		t.Run(name, func(t *testing.T) {
			err := previewDomain(t, given.app, given.previews)
			if err == nil {
				t.Fatal(
					"an instance started serving previews from a domain that carries its own " +
						"session cookie. A preview runs code norn did not write, and a browser " +
						"sends that cookie to every host underneath the domain that set it",
				)
			}

			if !strings.Contains(err.Error(), "previews.base_domain") {
				t.Errorf("error = %v, want it to name the setting that is wrong", err)
			}
		})
	}
}

func TestAPreviewDomainOfItsOwnIsAccepted(t *testing.T) {
	if err := previewDomain(t, "https://norn.site", "preview.norn-previews.site"); err != nil {
		t.Fatalf("config.New() = %v, want a separate preview domain to be accepted", err)
	}
}

func TestAnInstanceServingNoPreviewDomainStillStarts(t *testing.T) {
	if err := previewDomain(t, "https://norn.site", ""); err != nil {
		t.Fatalf(
			"config.New() = %v, want an instance with no gateway in front of it to start. "+
				"Previews are recorded either way; only the address is missing",
			err,
		)
	}
}

func TestAShareLinkDefaultLongerThanTheLongestOneAllowedIsRefused(t *testing.T) {
	t.Setenv("NORN_SECURITY_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("NORN_SMTP_HOST", "smtp.test")
	t.Setenv("NORN_SMTP_FROM_ADDRESS", "no-reply@norn.test")
	t.Setenv("NORN_PREVIEWS_SHARE_DEFAULT_TTL", "30d")
	t.Setenv("NORN_PREVIEWS_SHARE_MAX_TTL", "168h")

	if _, err := config.New(""); err == nil {
		t.Fatal(
			"an instance started where every link minted without a stated lifetime is already " +
				"past the longest lifetime anybody may ask for",
		)
	}
}
