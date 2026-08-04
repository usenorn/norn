package ssoconnection

import (
	"embed"
	"fmt"
	htmltemplate "html/template"
	"strings"
	texttemplate "text/template"

	"github.com/usenorn/norn/internal/entity"
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
	certificateExpiryHTML = htmltemplate.Must(
		htmltemplate.ParseFS(templates, "templates/certificate_expiry.html"),
	)
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
	var plain, html strings.Builder

	if err := certificateExpiryPlain.Execute(&plain, content); err != nil {
		return entity.Mail{}, fmt.Errorf("render certificate expiry text: %w", err)
	}

	if err := certificateExpiryHTML.Execute(&html, content); err != nil {
		return entity.Mail{}, fmt.Errorf("render certificate expiry html: %w", err)
	}

	subject := certificateExpirySubject
	if content.Expired {
		subject = certificateExpiredSubject
	}

	return entity.Mail{
		To:        to,
		Subject:   subject,
		PlainBody: plain.String(),
		HTMLBody:  html.String(),
	}, nil
}
