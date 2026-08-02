package account

import (
	"embed"
	"fmt"
	htmltemplate "html/template"
	"net/url"
	"strings"
	texttemplate "text/template"

	"github.com/usenorn/norn/internal/entity"
)

const (
	emailChangeSubject = "Confirm your new Norn email address"
	emailChangePath    = "/account/email-change/confirm"

	passwordResetSubject    = "Reset your Norn password"
	passwordResetSSOSubject = "Your Norn workspaces sign in through single sign-on"
	passwordResetPath       = "/reset-password"
)

//go:embed templates/email_change.txt templates/email_change.html templates/password_reset.txt templates/password_reset.html templates/password_reset_sso.txt templates/password_reset_sso.html
var templates embed.FS

var (
	emailChangePlain = texttemplate.Must(texttemplate.ParseFS(templates, "templates/email_change.txt"))
	emailChangeHTML  = htmltemplate.Must(htmltemplate.ParseFS(templates, "templates/email_change.html"))

	passwordResetPlain = texttemplate.Must(texttemplate.ParseFS(templates, "templates/password_reset.txt"))
	passwordResetHTML  = htmltemplate.Must(htmltemplate.ParseFS(templates, "templates/password_reset.html"))

	passwordResetSSOPlain = texttemplate.Must(texttemplate.ParseFS(templates, "templates/password_reset_sso.txt"))
	passwordResetSSOHTML  = htmltemplate.Must(htmltemplate.ParseFS(templates, "templates/password_reset_sso.html"))
)

type emailChangeContent struct {
	DisplayName    string
	NewEmail       string
	ConfirmURL     string
	ExpiresInHours int
}

func buildEmailChangeConfirmation(baseURL, displayName, newEmail, token string) (entity.Mail, error) {
	confirmURL, err := url.JoinPath(baseURL, emailChangePath)
	if err != nil {
		return entity.Mail{}, fmt.Errorf("build email change confirmation url: %w", err)
	}

	content := emailChangeContent{
		DisplayName:    displayName,
		NewEmail:       newEmail,
		ConfirmURL:     confirmURL + "?token=" + url.QueryEscape(token),
		ExpiresInHours: int(entity.EmailChangeTokenTTL.Hours()),
	}

	var plain strings.Builder
	if err := emailChangePlain.ExecuteTemplate(&plain, "email_change.txt", content); err != nil {
		return entity.Mail{}, fmt.Errorf("render email change plain body: %w", err)
	}

	var html strings.Builder
	if err := emailChangeHTML.ExecuteTemplate(&html, "email_change.html", content); err != nil {
		return entity.Mail{}, fmt.Errorf("render email change html body: %w", err)
	}

	return entity.Mail{
		To:        newEmail,
		Subject:   emailChangeSubject,
		PlainBody: plain.String(),
		HTMLBody:  html.String(),
	}, nil
}

type passwordResetContent struct {
	DisplayName string
	ResetURL    string
}

type passwordResetSSOContent struct {
	DisplayName string
}

func buildPasswordReset(baseURL, displayName, email, token string) (entity.Mail, error) {
	resetURL, err := url.JoinPath(baseURL, passwordResetPath)
	if err != nil {
		return entity.Mail{}, fmt.Errorf("build password reset url: %w", err)
	}

	content := passwordResetContent{
		DisplayName: displayName,
		ResetURL:    resetURL + "?token=" + url.QueryEscape(token),
	}

	var plain strings.Builder
	if err := passwordResetPlain.ExecuteTemplate(&plain, "password_reset.txt", content); err != nil {
		return entity.Mail{}, fmt.Errorf("render password reset plain body: %w", err)
	}

	var html strings.Builder
	if err := passwordResetHTML.ExecuteTemplate(&html, "password_reset.html", content); err != nil {
		return entity.Mail{}, fmt.Errorf("render password reset html body: %w", err)
	}

	return entity.Mail{
		To:        email,
		Subject:   passwordResetSubject,
		PlainBody: plain.String(),
		HTMLBody:  html.String(),
	}, nil
}

func buildPasswordResetSSONotice(displayName, email string) (entity.Mail, error) {
	content := passwordResetSSOContent{DisplayName: displayName}

	var plain strings.Builder
	if err := passwordResetSSOPlain.ExecuteTemplate(&plain, "password_reset_sso.txt", content); err != nil {
		return entity.Mail{}, fmt.Errorf("render password reset sso plain body: %w", err)
	}

	var html strings.Builder
	if err := passwordResetSSOHTML.ExecuteTemplate(&html, "password_reset_sso.html", content); err != nil {
		return entity.Mail{}, fmt.Errorf("render password reset sso html body: %w", err)
	}

	return entity.Mail{
		To:        email,
		Subject:   passwordResetSSOSubject,
		PlainBody: plain.String(),
		HTMLBody:  html.String(),
	}, nil
}
