package preview

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

func (s *previewsService) Share(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
	request service.PreviewShareRequest,
) (service.PreviewShareMinted, error) {
	preview, err := s.named(ctx, workspaceID, executionID, request.Name, entity.ActionManage)
	if err != nil {
		return service.PreviewShareMinted{}, err
	}

	if !s.settings.Routable() {
		return service.PreviewShareMinted{}, entity.ErrPreviewNotRoutable
	}

	if !preview.Open() {
		return service.PreviewShareMinted{}, entity.ErrPreviewClosed
	}

	lifetime := request.Lifetime
	if lifetime <= 0 {
		lifetime = s.settings.ShareDefaultTTL
	}

	if err := entity.NewValidationError(
		entity.ValidatePreviewShareLifetime("lifetime", lifetime, s.settings.ShareMaxTTL),
		entity.ValidatePreviewSharePasscode("passcode", request.Passcode),
	); err != nil {
		return service.PreviewShareMinted{}, err
	}

	if err := s.spare(ctx, preview); err != nil {
		return service.PreviewShareMinted{}, err
	}

	passcode, err := passcodeOf(request.Passcode)
	if err != nil {
		return service.PreviewShareMinted{}, err
	}

	token, hash, err := entity.NewPreviewShareToken()
	if err != nil {
		return service.PreviewShareMinted{}, err
	}

	actor, _ := identity.Actor(ctx)
	now := time.Now().UTC()

	created, err := s.shares.Create(ctx, entity.PreviewShareLink{
		PreviewID:    preview.ID,
		ExecutionID:  preview.ExecutionID,
		WorkspaceID:  preview.WorkspaceID,
		TokenHash:    hash,
		PasscodeHash: passcode,
		CreatedBy:    actor.Authority(),
		ExpiresAt:    now.Add(lifetime),
	})
	if err != nil {
		return service.PreviewShareMinted{}, err
	}

	s.noted(ctx, preview, entity.AuditPreviewShared)

	return service.PreviewShareMinted{
		Link: created,
		URL:  shareURL(preview, s.settings.Scheme, token),
	}, nil
}

func (s *previewsService) RevokeShare(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID, name string,
	linkID uuid.UUID,
) error {
	preview, err := s.named(ctx, workspaceID, executionID, name, entity.ActionManage)
	if err != nil {
		return err
	}

	if _, err := s.shares.Revoke(ctx, preview.ID, linkID, time.Now().UTC()); err != nil {
		return err
	}

	if err := s.grants.RevokeLink(ctx, linkID); err != nil {
		return err
	}

	s.noted(ctx, preview, entity.AuditPreviewShareRevoked)

	return nil
}

func (s *previewsService) Redeem(
	ctx context.Context,
	host, token, passcode string,
) (entity.PreviewAccess, error) {
	preview, err := s.serving(ctx, host)
	if err != nil {
		return entity.PreviewAccess{}, err
	}

	link, err := s.shares.ByToken(ctx, entity.HashPreviewShareToken(token))
	if err != nil {
		if errors.Is(err, entity.ErrPreviewShareNotFound) {
			entity.BurnPasswordGuess(passcode)
		}

		return entity.PreviewAccess{}, err
	}

	if link.PreviewID != preview.ID {
		return entity.PreviewAccess{}, entity.ErrPreviewShareNotFound
	}

	now := time.Now().UTC()

	if err := link.Usable(now); err != nil {
		return entity.PreviewAccess{}, err
	}

	if err := s.guessed(ctx, link); err != nil {
		return entity.PreviewAccess{}, err
	}

	if err := admits(link, passcode); err != nil {
		return entity.PreviewAccess{}, err
	}

	expires := now.Add(s.settings.SessionTTL)
	if link.ExpiresAt.Before(expires) {
		expires = link.ExpiresAt
	}

	granted, err := s.grants.Issue(
		ctx,
		entity.NewPreviewGrant(preview, uuid.Nil, link.ID, now, expires),
		expires.Sub(now),
	)
	if err != nil {
		return entity.PreviewAccess{}, err
	}

	if err := s.shares.Used(ctx, link.ID, now); err != nil {
		return entity.PreviewAccess{}, err
	}

	return entity.PreviewAccess{
		Verdict:   entity.PreviewAllowed,
		Preview:   preview,
		Token:     granted,
		ExpiresAt: expires,
	}, nil
}

func (s *previewsService) guessed(ctx context.Context, link entity.PreviewShareLink) error {
	if !link.NeedsPasscode() {
		return nil
	}

	made, err := s.grants.Attempt(
		ctx, link.ID.String(), entity.PreviewShareAttemptWindow,
	)
	if err != nil {
		return err
	}

	if made > entity.PreviewShareMaxAttempts {
		return entity.ErrPreviewShareGuessed
	}

	return nil
}

func (s *previewsService) spare(ctx context.Context, preview entity.PreviewSession) error {
	held, err := s.shares.ByPreview(ctx, preview.ID)
	if err != nil {
		return err
	}

	if len(held) >= entity.PreviewShareLinksMax {
		return entity.ErrPreviewShareCrowded
	}

	return nil
}

func (s *previewsService) noted(
	ctx context.Context,
	preview entity.PreviewSession,
	action entity.AuditAction,
) {
	execution, err := s.executions.GetByID(ctx, preview.ExecutionID)
	if err != nil {
		return
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  preview.WorkspaceID,
		Action:       action,
		ResourceKind: string(entity.ResourceIssue),
		ResourceID:   execution.IssueID,
		ResourceName: execution.Reference(),
		Detail:       map[string]string{"preview": preview.Name},
	})
}

func admits(link entity.PreviewShareLink, passcode string) error {
	if !link.NeedsPasscode() {
		return nil
	}

	matched, err := entity.VerifyPassword(link.PasscodeHash, passcode)
	if err != nil {
		return err
	}

	if !matched {
		return entity.ErrPreviewSharePasscode
	}

	return nil
}

func passcodeOf(passcode string) (string, error) {
	if passcode == "" {
		return "", nil
	}

	return entity.HashPassword(passcode)
}

func shareURL(preview entity.PreviewSession, scheme, token string) string {
	return scheme + "://" + preview.Host + entity.PreviewSharePath + token
}

func handover(preview entity.PreviewSession, scheme, ticket, returnTo string) string {
	handed := url.Values{}
	handed.Set("ticket", ticket)

	if returnTo != "" {
		handed.Set("return", returnTo)
	}

	return scheme + "://" + preview.Host + entity.PreviewSessionPath + "?" + handed.Encode()
}
