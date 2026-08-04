package ssoconnection

import (
	"context"
	"time"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
)

func (s *connectionsService) SweepCertificates(ctx context.Context) error {
	connections, err := s.connections.ListSAMLCertificates(ctx)
	if err != nil {
		return err
	}

	now := time.Now().UTC()

	for _, connection := range connections {
		daysLeft := entity.DaysUntil(connection.Descriptor.ExpiresAt, now)

		threshold, due := entity.ExpiryNoticeDue(daysLeft, connection.ExpiryNoticeDays)
		if !due {
			continue
		}

		if err := s.warn(ctx, connection, daysLeft); err != nil {
			logging.From(ctx).ErrorContext(
				ctx,
				"warning administrators about an expiring certificate failed",
				"workspace_id", connection.WorkspaceID.String(),
				"error", err.Error(),
			)

			continue
		}

		if err := s.connections.RecordExpiryNotice(ctx, connection.WorkspaceID, threshold); err != nil {
			return err
		}
	}

	return nil
}

func (s *connectionsService) warn(
	ctx context.Context,
	connection entity.SAMLConnection,
	daysLeft int,
) error {
	workspace, err := s.workspaces.GetByID(ctx, connection.WorkspaceID)
	if err != nil {
		return err
	}

	admins, err := s.memberships.ListPageByWorkspaceID(
		ctx,
		connection.WorkspaceID,
		entity.MembershipPage{Limit: entity.MembershipPageMaxSize},
	)
	if err != nil {
		return err
	}

	content := certificateExpiryContent{
		Workspace:   workspace.Name,
		Provider:    connection.Descriptor.EntityID,
		ExpiresAt:   connection.Descriptor.ExpiresAt.UTC().Format("2 January 2006"),
		DaysLeft:    daysLeft,
		Expired:     entity.CertificateExpired(connection.Descriptor.ExpiresAt, time.Now().UTC()),
		SettingsURL: settingsURL(s.app.BaseURL, workspace.Slug),
	}

	for _, member := range admins {
		if member.Membership.Role != entity.MembershipRoleAdmin {
			continue
		}

		mail, err := buildCertificateExpiry(member.Email, content)
		if err != nil {
			return err
		}

		if err := s.mailer.Send(ctx, mail); err != nil {
			return err
		}
	}

	return nil
}
