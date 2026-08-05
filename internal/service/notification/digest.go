package notification

import (
	"context"
	"embed"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"net/url"
	"strconv"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

const digestSubject = "What happened while you were away"

//go:embed templates/digest.txt templates/digest.html
var templates embed.FS

var (
	digestPlain = texttemplate.Must(texttemplate.ParseFS(templates, "templates/digest.txt"))
	digestHTML  = htmltemplate.Must(htmltemplate.ParseFS(templates, "templates/digest.html"))
)

type digestEntry struct {
	Reference string
	Title     string
	Summary   string
	URL       string
}

type digestContent struct {
	WorkspaceName string
	InboxURL      string
	Entries       []digestEntry
}

func (s *notificationsService) Digest(ctx context.Context, now time.Time) error {
	if !s.smtp.Configured() {
		return nil
	}

	window := entity.NotificationDigestWindowAt(now)

	recipients, err := s.notifications.DigestRecipients(ctx, window)
	if err != nil {
		return err
	}

	failures := make([]error, 0)

	for _, recipient := range recipients {
		claimed, err := s.notifications.ClaimDigest(ctx, recipient)
		if err != nil {
			return err
		}

		if !claimed {
			continue
		}

		if err := s.send(ctx, recipient); err != nil {
			if outcomeErr := s.notifications.RecordDigestOutcome(ctx, recipient, err); outcomeErr != nil {
				return errors.Join(err, outcomeErr)
			}

			failures = append(failures, err)

			continue
		}

		if err := s.notifications.RecordDigestOutcome(ctx, recipient, nil); err != nil {
			return err
		}
	}

	return errors.Join(failures...)
}

func (s *notificationsService) send(ctx context.Context, recipient entity.NotificationDigestClaim) error {
	entries, err := s.notifications.DigestEntries(
		ctx, recipient.WorkspaceID, recipient.AccountID, recipient.Window,
	)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		return nil
	}

	workspace, err := s.workspaces.GetByID(ctx, recipient.WorkspaceID)
	if err != nil {
		return err
	}

	message, err := buildDigest(s.app.BaseURL, workspace.Slug, workspace.Name, recipient.Email, entries)
	if err != nil {
		return err
	}

	return s.mailer.Send(ctx, message)
}

func buildDigest(
	baseURL, workspaceSlug, workspaceName, email string,
	notifications []entity.Notification,
) (entity.Mail, error) {
	content := digestContent{
		WorkspaceName: workspaceName,
		InboxURL:      workspacePath(baseURL, workspaceSlug, "inbox"),
		Entries:       make([]digestEntry, 0, len(notifications)),
	}

	for _, notification := range notifications {
		content.Entries = append(content.Entries, digestEntry{
			Reference: notification.Reference,
			Title:     notification.Title,
			Summary:   summarise(notification),
			URL:       subjectURL(baseURL, workspaceSlug, notification),
		})
	}

	var plain strings.Builder
	if err := digestPlain.ExecuteTemplate(&plain, "digest.txt", content); err != nil {
		return entity.Mail{}, fmt.Errorf("render digest plain body: %w", err)
	}

	var html strings.Builder
	if err := digestHTML.ExecuteTemplate(&html, "digest.html", content); err != nil {
		return entity.Mail{}, fmt.Errorf("render digest html body: %w", err)
	}

	return entity.Mail{
		To:        email,
		Subject:   digestSubject,
		PlainBody: plain.String(),
		HTMLBody:  html.String(),
	}, nil
}

func summarise(notification entity.Notification) string {
	actor := notification.ActorName
	if actor == "" {
		actor = "Someone"
	}

	if notification.ActorKind == entity.ActorKindAgent {
		actor += " (agent)"
	}

	change := "made a change"

	switch notification.Kind {
	case entity.NotificationKindAssigned:
		change = "assigned this to you"
	case entity.NotificationKindCommented:
		change = "commented"
	case entity.NotificationKindStateChanged:
		change = "changed the state"
	case entity.NotificationKindMembership:
		change = "added you"
	}

	if notification.Reason == entity.NotificationReasonMentioned {
		change = "mentioned you"
	}

	if notification.UnreadCount > 1 {
		change += " (" + strconv.Itoa(notification.UnreadCount) + " updates)"
	}

	return actor + " " + change
}

func subjectURL(baseURL, workspaceSlug string, notification entity.Notification) string {
	switch notification.Subject.Kind {
	case entity.NotificationSubjectProject:
		return workspacePath(baseURL, workspaceSlug, "projects", notification.Subject.ID.String())
	case entity.NotificationSubjectTeam:
		return workspacePath(baseURL, workspaceSlug, "teams", notification.TeamKey)
	default:
		return workspacePath(baseURL, workspaceSlug, "issues", notification.Reference)
	}
}

func workspacePath(baseURL, workspaceSlug string, segments ...string) string {
	joined, err := url.JoinPath(baseURL, append([]string{workspaceSlug}, segments...)...)
	if err != nil {
		return baseURL
	}

	return joined
}
