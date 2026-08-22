package config_test

import (
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/config"
)

func TestAnInstanceWithNowhereToSendMailRefusesToStart(t *testing.T) {
	for name, mail := range map[string]struct{ host, from string }{
		"nothing at all":       {"", ""},
		"a host and no sender": {"smtp.test", ""},
		"a sender and no host": {"", "no-reply@norn.test"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("NORN_SECURITY_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
			t.Setenv("NORN_SMTP_HOST", mail.host)
			t.Setenv("NORN_SMTP_FROM_ADDRESS", mail.from)

			_, err := config.New("")
			if err == nil {
				t.Fatal(
					"the instance started with nowhere to send mail. Signing in needs a code " +
						"delivered by email, so it would accept a password and then have no way " +
						"to finish, locking everybody out of a running instance.",
				)
			}

			if !strings.Contains(err.Error(), "smtp.host") {
				t.Errorf("error = %v, want it to name the setting that is missing", err)
			}
		})
	}
}

func TestAMailServerIsEnoughToStart(t *testing.T) {
	t.Setenv("NORN_SECURITY_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("NORN_SMTP_HOST", "smtp.test")
	t.Setenv("NORN_SMTP_FROM_ADDRESS", "no-reply@norn.test")

	cfg, err := config.New("")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if cfg.SMTP.Host != "smtp.test" || cfg.SMTP.FromAddress != "no-reply@norn.test" {
		t.Errorf("smtp = %+v, want the values the environment set", cfg.SMTP)
	}
}
