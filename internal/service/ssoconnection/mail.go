package ssoconnection

import (
	"embed"
	"fmt"
	"strings"
	texttemplate "text/template"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/mailtemplate"
)

const (
	certificateExpirySubject  = "Your single sign-on certificate is expiring"
	certificateExpiredSubject = "Single sign-on has stopped working"
	settingsPath              = "/settings/authentication"
)

//go:embed templates/certificate_expiry.txt templates/certificate_expiry.html
var templates embed.FS

var (
	certificateExpiryPlain = texttemplate.Must(
		texttemplate.ParseFS(templates, "templates/certificate_expiry.txt"),
	)
	certificateExpiryHTML = mailtemplate.MustHTML(templates, "templates/certificate_expiry.html")
)

type certificateExpiryContent struct {
	Workspace   string
	Provider    string
	ExpiresAt   string
	DaysLeft    int
	Expired     bool
	SettingsURL string
}

func settingsURL(baseURL, workspaceSlug string) string {
	return strings.TrimRight(baseURL, "/") + "/" + workspaceSlug + settingsPath
}

func buildCertificateExpiry(to string, content certificateExpiryContent) (entity.Mail, error) {
	var plain strings.Builder

	if err := certificateExpiryPlain.Execute(&plain, content); err != nil {
		return entity.Mail{}, fmt.Errorf("render certificate expiry text: %w", err)
	}

	subject := certificateExpirySubject
	if content.Expired {
		subject = certificateExpiredSubject
	}

	html, err := mailtemplate.Render(certificateExpiryHTML, mailtemplate.Shell{
		Subject:   subject,
		Preheader: "Upload the replacement certificate before it lapses.",
		Eyebrow:   "Single sign-on",
		LogoURL:   mailtemplate.LogoURLFrom(content.SettingsURL),
		Content:   content,
	})
	if err != nil {
		return entity.Mail{}, err
	}

	return entity.Mail{
		To:        to,
		Subject:   subject,
		PlainBody: plain.String(),
		HTMLBody:  html,
	}, nil
}
