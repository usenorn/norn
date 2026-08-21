package account

import (
	"embed"
	"fmt"
	"net/url"
	"strings"
	texttemplate "text/template"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/mailtemplate"
)

const (
	signUpVerificationSubject = "Confirm your Norn account"
	signUpVerificationPath    = "/sign-up/confirm"

	emailChangeSubject = "Confirm your new Norn email address"
	emailChangePath    = "/account/email-change/confirm"

	passwordResetSubject    = "Reset your Norn password"
	passwordResetSSOSubject = "Your Norn workspaces sign in through single sign-on"
	passwordResetPath       = "/reset-password"
)

//go:embed templates/sign_up_verification.txt templates/sign_up_verification.html templates/email_change.txt templates/email_change.html templates/password_reset.txt templates/password_reset.html templates/password_reset_sso.txt templates/password_reset_sso.html
var templates embed.FS

var (
	signUpVerificationPlain = texttemplate.Must(texttemplate.ParseFS(templates, "templates/sign_up_verification.txt"))
	signUpVerificationHTML  = mailtemplate.MustHTML(templates, "templates/sign_up_verification.html")

	emailChangePlain = texttemplate.Must(texttemplate.ParseFS(templates, "templates/email_change.txt"))
	emailChangeHTML  = mailtemplate.MustHTML(templates, "templates/email_change.html")

	passwordResetPlain = texttemplate.Must(texttemplate.ParseFS(templates, "templates/password_reset.txt"))
	passwordResetHTML  = mailtemplate.MustHTML(templates, "templates/password_reset.html")

	passwordResetSSOPlain = texttemplate.Must(texttemplate.ParseFS(templates, "templates/password_reset_sso.txt"))
	passwordResetSSOHTML  = mailtemplate.MustHTML(templates, "templates/password_reset_sso.html")
)

func (s *accountsService) signUpURL(token string) string {
	confirmURL, err := url.JoinPath(s.app.BaseURL, signUpVerificationPath)
	if err != nil {
		return ""
	}

	return confirmURL + "?token=" + url.QueryEscape(token)
}

type signUpVerificationContent struct {
	DisplayName string
	ConfirmURL  string
}

func buildSignUpVerification(baseURL, confirmURL, displayName, email string) (entity.Mail, error) {
	content := signUpVerificationContent{
		DisplayName: displayName,
		ConfirmURL:  confirmURL,
	}

	var plain strings.Builder
	if err := signUpVerificationPlain.ExecuteTemplate(&plain, "sign_up_verification.txt", content); err != nil {
		return entity.Mail{}, fmt.Errorf("render sign-up verification plain body: %w", err)
	}

	html, err := mailtemplate.Render(signUpVerificationHTML, mailtemplate.Shell{
		Subject:   signUpVerificationSubject,
		Preheader: "One link, good for an hour.",
		Eyebrow:   "Account",
		LogoURL:   mailtemplate.LogoURL(baseURL),
		Content:   content,
	})
	if err != nil {
		return entity.Mail{}, err
	}

	return entity.Mail{
		To:        email,
		Subject:   signUpVerificationSubject,
		PlainBody: plain.String(),
		HTMLBody:  html,
	}, nil
}

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

	html, err := mailtemplate.Render(emailChangeHTML, mailtemplate.Shell{
		Subject:   emailChangeSubject,
		Preheader: "Confirm the new address to finish the change.",
		Eyebrow:   "Account",
		LogoURL:   mailtemplate.LogoURL(baseURL),
		Content:   content,
	})
	if err != nil {
		return entity.Mail{}, err
	}

	return entity.Mail{
		To:        newEmail,
		Subject:   emailChangeSubject,
		PlainBody: plain.String(),
		HTMLBody:  html,
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

	html, err := mailtemplate.Render(passwordResetHTML, mailtemplate.Shell{
		Subject:   passwordResetSubject,
		Preheader: "Set a new password with a one-time link.",
		Eyebrow:   "Security",
		LogoURL:   mailtemplate.LogoURL(baseURL),
		Content:   content,
	})
	if err != nil {
		return entity.Mail{}, err
	}

	return entity.Mail{
		To:        email,
		Subject:   passwordResetSubject,
		PlainBody: plain.String(),
		HTMLBody:  html,
	}, nil
}

func buildPasswordResetSSONotice(displayName, email string) (entity.Mail, error) {
	content := passwordResetSSOContent{DisplayName: displayName}

	var plain strings.Builder
	if err := passwordResetSSOPlain.ExecuteTemplate(&plain, "password_reset_sso.txt", content); err != nil {
		return entity.Mail{}, fmt.Errorf("render password reset sso plain body: %w", err)
	}

	html, err := mailtemplate.Render(passwordResetSSOHTML, mailtemplate.Shell{
		Subject:   passwordResetSSOSubject,
		Preheader: "Your workspaces sign in through your identity provider.",
		Eyebrow:   "Security",
		Content:   content,
	})
	if err != nil {
		return entity.Mail{}, err
	}

	return entity.Mail{
		To:        email,
		Subject:   passwordResetSSOSubject,
		PlainBody: plain.String(),
		HTMLBody:  html,
	}, nil
}
