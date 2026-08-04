package invitation

import (
	"context"
	"embed"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"net/url"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

const (
	invitationSubject = "You have been invited to a Norn workspace"
	invitationPath    = "/accept-invitation"
)

//go:embed templates/invitation.txt templates/invitation.html
var templates embed.FS

var (
	invitationPlain = texttemplate.Must(texttemplate.ParseFS(templates, "templates/invitation.txt"))
	invitationHTML  = htmltemplate.Must(htmltemplate.ParseFS(templates, "templates/invitation.html"))
)

type invitationContent struct {
	WorkspaceName string
	Role          string
	AcceptURL     string
	ExpiresInDays int
}

func (s *invitationsService) invitationURL(token string) string {
	acceptURL, err := url.JoinPath(s.app.BaseURL, invitationPath)
	if err != nil {
		return ""
	}

	return acceptURL + "?token=" + url.QueryEscape(token)
}

func buildInvitation(acceptURL, workspaceName, email string, role entity.MembershipRole) (entity.Mail, error) {
	content := invitationContent{
		WorkspaceName: workspaceName,
		Role:          string(role),
		AcceptURL:     acceptURL,
		ExpiresInDays: int(entity.InvitationTokenTTL / (24 * time.Hour)),
	}

	var plain strings.Builder
	if err := invitationPlain.ExecuteTemplate(&plain, "invitation.txt", content); err != nil {
		return entity.Mail{}, fmt.Errorf("render invitation plain body: %w", err)
	}

	var html strings.Builder
	if err := invitationHTML.ExecuteTemplate(&html, "invitation.html", content); err != nil {
		return entity.Mail{}, fmt.Errorf("render invitation html body: %w", err)
	}

	return entity.Mail{
		To:        email,
		Subject:   invitationSubject,
		PlainBody: plain.String(),
		HTMLBody:  html.String(),
	}, nil
}

func (s *invitationsService) SendInvitation(ctx context.Context, invitationID uuid.UUID, token string) error {
	invitation, err := s.invitations.GetByID(ctx, invitationID)
	if err != nil {
		if errors.Is(err, entity.ErrInvitationNotFound) {
			return nil
		}

		return err
	}

	if invitation.UsableAt(time.Now().UTC()) != nil {
		return nil
	}

	workspace, err := s.workspaces.GetByID(ctx, invitation.WorkspaceID)
	if err != nil {
		if errors.Is(err, entity.ErrWorkspaceNotFound) {
			return nil
		}

		return err
	}

	message, err := buildInvitation(s.invitationURL(token), workspace.Name, invitation.Email, invitation.Role)
	if err != nil {
		return err
	}

	if err := s.mailer.Send(ctx, message); err != nil {
		if deliveryErr := s.invitations.SetDelivery(ctx, invitation.ID, entity.InvitationDeliveryFailed); deliveryErr != nil {
			return errors.Join(err, deliveryErr)
		}

		return err
	}

	return s.invitations.SetDelivery(ctx, invitation.ID, entity.InvitationDeliverySent)
}
