package apitoken

import (
	"embed"
	"fmt"
	"strings"
	texttemplate "text/template"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/mailtemplate"
)

const (
	tokenExpirySubject  = "Your API token is expiring"
	tokenExpiredSubject = "Your API token has expired"
	tokensPath          = "/settings/tokens"
)

//go:embed templates/token_expiry.txt templates/token_expiry.html
var templates embed.FS

var (
	tokenExpiryPlain = texttemplate.Must(
		texttemplate.ParseFS(templates, "templates/token_expiry.txt"),
	)
	tokenExpiryHTML = mailtemplate.MustHTML(templates, "templates/token_expiry.html")
)

type tokenExpiryContent struct {
	Name        string
	ExpiresAt   string
	DaysLeft    int
	Expired     bool
	Workspaces  string
	SettingsURL string
}

func tokensURL(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + tokensPath
}

func buildTokenExpiry(to string, content tokenExpiryContent) (entity.Mail, error) {
	var plain strings.Builder

	if err := tokenExpiryPlain.Execute(&plain, content); err != nil {
		return entity.Mail{}, fmt.Errorf("render token expiry text: %w", err)
	}

	subject := tokenExpirySubject
	if content.Expired {
		subject = tokenExpiredSubject
	}

	html, err := mailtemplate.Render(tokenExpiryHTML, mailtemplate.Shell{
		Subject:   subject,
		Preheader: "Mint a replacement and move anything using this token over.",
		Eyebrow:   "API token",
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
