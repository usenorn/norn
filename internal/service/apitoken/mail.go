package apitoken

import (
	"embed"
	"fmt"
	htmltemplate "html/template"
	"strings"
	texttemplate "text/template"

	"github.com/usenorn/norn/internal/entity"
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
	tokenExpiryHTML = htmltemplate.Must(
		htmltemplate.ParseFS(templates, "templates/token_expiry.html"),
	)
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
	var plain, html strings.Builder

	if err := tokenExpiryPlain.Execute(&plain, content); err != nil {
		return entity.Mail{}, fmt.Errorf("render token expiry text: %w", err)
	}

	if err := tokenExpiryHTML.Execute(&html, content); err != nil {
		return entity.Mail{}, fmt.Errorf("render token expiry html: %w", err)
	}

	subject := tokenExpirySubject
	if content.Expired {
		subject = tokenExpiredSubject
	}

	return entity.Mail{
		To:        to,
		Subject:   subject,
		PlainBody: plain.String(),
		HTMLBody:  html.String(),
	}, nil
}
