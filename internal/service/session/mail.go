package session

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/mailtemplate"
)

const signInCodeSubject = "Your Norn sign-in code"

//go:embed templates/sign_in_code.txt templates/sign_in_code.html
var templates embed.FS

var (
	signInCodePlain = texttemplate.Must(texttemplate.ParseFS(templates, "templates/sign_in_code.txt"))
	signInCodeHTML  = mailtemplate.MustHTML(templates, "templates/sign_in_code.html")
)

type signInCodeContent struct {
	DisplayName string
	Code        string
	Minutes     int
	Expiry      string
}

func buildSignInCode(baseURL, displayName, email, code string) (entity.Mail, error) {
	minutes := int(entity.SignInChallengeTTL / time.Minute)
	content := signInCodeContent{
		DisplayName: displayName,
		Code:        code,
		Minutes:     minutes,
		Expiry:      fmt.Sprintf("%d minutes", minutes),
	}

	var plain strings.Builder

	if err := signInCodePlain.ExecuteTemplate(&plain, "sign_in_code.txt", content); err != nil {
		return entity.Mail{}, fmt.Errorf("render sign-in code text: %w", err)
	}

	html, err := mailtemplate.Render(signInCodeHTML, mailtemplate.Shell{
		Subject:   signInCodeSubject,
		Preheader: "One code, good for " + content.Expiry + ".",
		Eyebrow:   "Sign in",
		LogoURL:   mailtemplate.LogoURL(baseURL),
		Content:   content,
	})
	if err != nil {
		return entity.Mail{}, fmt.Errorf("render sign-in code html: %w", err)
	}

	return entity.Mail{
		To:        email,
		Subject:   signInCodeSubject,
		PlainBody: plain.String(),
		HTMLBody:  html,
	}, nil
}

func (s *sessionsService) SendSignInCode(ctx context.Context, challengeID, code string) error {
	challenge, err := s.challenges.Get(ctx, challengeID)
	if err != nil {
		if errors.Is(err, entity.ErrSignInChallengeNotFound) {
			return nil
		}

		return err
	}

	if challenge.ExpiredAt(time.Now().UTC()) {
		return nil
	}

	message, err := buildSignInCode(s.app.BaseURL, challenge.DisplayName, challenge.Email, code)
	if err != nil {
		return err
	}

	return s.mailer.Send(ctx, message)
}
