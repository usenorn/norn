package webhook

import (
	"context"
	"embed"
	"fmt"
	htmltemplate "html/template"
	"strings"
	texttemplate "text/template"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
)

const (
	webhookDisabledSubject = "Norn stopped sending to your webhook"
	webhooksPath           = "/settings/webhooks"
)

//go:embed templates/webhook_disabled.txt templates/webhook_disabled.html
var templates embed.FS

var (
	webhookDisabledPlain = texttemplate.Must(
		texttemplate.ParseFS(templates, "templates/webhook_disabled.txt"),
	)
	webhookDisabledHTML = htmltemplate.Must(
		htmltemplate.ParseFS(templates, "templates/webhook_disabled.html"),
	)
)

type webhookDisabledContent struct {
	Name          string
	URL           string
	Workspace     string
	FailureStreak int
	LastError     string
	SettingsURL   string
}

func webhooksURL(baseURL, slug string) string {
	return strings.TrimRight(baseURL, "/") + "/" + slug + webhooksPath
}

func buildWebhookDisabled(to string, content webhookDisabledContent) (entity.Mail, error) {
	var plain, html strings.Builder

	if err := webhookDisabledPlain.Execute(&plain, content); err != nil {
		return entity.Mail{}, fmt.Errorf("render webhook disabled text: %w", err)
	}

	if err := webhookDisabledHTML.Execute(&html, content); err != nil {
		return entity.Mail{}, fmt.Errorf("render webhook disabled html: %w", err)
	}

	return entity.Mail{
		To:        to,
		Subject:   webhookDisabledSubject,
		PlainBody: plain.String(),
		HTMLBody:  html.String(),
	}, nil
}

func (s *deliverer) warn(ctx context.Context, hook entity.Webhook, delivery entity.WebhookDelivery, streak int) {
	owner, err := s.accounts.GetByID(ctx, hook.OwnerAccountID)
	if err != nil || owner.Status != entity.AccountStatusActive {
		return
	}

	workspace, err := s.workspaces.GetByID(ctx, hook.WorkspaceID)
	if err != nil {
		return
	}

	content := webhookDisabledContent{
		Name:          hook.Name,
		URL:           hook.URL,
		Workspace:     workspace.Name,
		FailureStreak: streak,
		SettingsURL:   webhooksURL(s.app.BaseURL, workspace.Slug),
	}

	if attempts, err := s.deliveries.Attempts(ctx, delivery.ID); err == nil && len(attempts) > 0 {
		content.LastError = attempts[len(attempts)-1].Error
	}

	mail, err := buildWebhookDisabled(owner.Email, content)
	if err != nil {
		logging.From(ctx).ErrorContext(ctx, "building the webhook disabled notice failed", "error", err.Error())

		return
	}

	if err := s.mailer.Send(ctx, mail); err != nil {
		logging.From(ctx).ErrorContext(
			ctx,
			"telling the owner their webhook was disabled failed",
			"webhook_id", hook.ID.String(),
			"error", err.Error(),
		)
	}
}
