package account

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

func (s *accountsService) RequestPasswordReset(ctx context.Context, input service.RequestPasswordResetInput) (time.Time, error) {
	expiresAt := time.Now().UTC().Add(entity.PasswordResetTokenTTL)

	attempts, err := s.throttle.RecordAddressAttempt(ctx, input.Client.IP)
	if err != nil {
		return time.Time{}, err
	}

	if attempts > entity.SignInAddressMaxAttempts {
		return time.Time{}, entity.ErrSignInRateLimited
	}

	normalized := entity.NormalizeEmail(input.Email)

	if err := entity.NewValidationError(entity.ValidateEmail("email", normalized)); err != nil {
		return time.Time{}, err
	}

	account, err := s.accounts.GetByEmail(ctx, normalized)
	if err != nil {
		if errors.Is(err, entity.ErrAccountNotFound) {
			return expiresAt, nil
		}

		return time.Time{}, err
	}

	if account.Agent() || account.Status != entity.AccountStatusActive {
		return expiresAt, nil
	}

	ssoOnly, err := s.ssoEnforcedEverywhere(ctx, account.ID)
	if err != nil {
		return time.Time{}, err
	}

	if ssoOnly {
		if err := s.producer.EnqueuePasswordResetSSONotice(ctx, entity.PasswordResetSSONoticePayload{
			AccountID: account.ID,
		}); err != nil {
			return time.Time{}, err
		}

		return expiresAt, nil
	}

	token, tokenHash, err := entity.NewPasswordResetToken()
	if err != nil {
		return time.Time{}, err
	}

	now := time.Now().UTC()

	var reset entity.PasswordReset

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.passwordResets.DeletePendingByAccountID(ctx, account.ID); err != nil {
			return err
		}

		created, err := s.passwordResets.Create(ctx, entity.PasswordReset{
			AccountID:   account.ID,
			TokenHash:   tokenHash,
			RequestedAt: now,
			ExpiresAt:   expiresAt,
		})
		if err != nil {
			return err
		}

		reset = created

		return nil
	})
	if err != nil {
		return time.Time{}, err
	}

	if err := s.producer.EnqueuePasswordReset(ctx, entity.PasswordResetPayload{
		PasswordResetID: reset.ID,
		Token:           token,
	}); err != nil {
		return time.Time{}, err
	}

	return expiresAt, nil
}

func (s *accountsService) ConfirmPasswordReset(
	ctx context.Context,
	token, password string,
) (uuid.UUID, error) {
	if token == "" {
		return uuid.Nil, entity.ErrPasswordResetTokenInvalid
	}

	reset, err := s.passwordResets.GetByTokenHash(ctx, entity.HashPasswordResetToken(token))
	if err != nil {
		return uuid.Nil, err
	}

	if reset.Used() {
		return uuid.Nil, entity.ErrPasswordResetAlreadyUsed
	}

	now := time.Now().UTC()

	if reset.ExpiredAt(now) {
		return uuid.Nil, entity.ErrPasswordResetExpired
	}

	account, err := s.activeAccount(ctx, reset.AccountID)
	if err != nil {
		return uuid.Nil, err
	}

	ssoOnly, err := s.ssoEnforcedEverywhere(ctx, account.ID)
	if err != nil {
		return uuid.Nil, err
	}

	if ssoOnly {
		return uuid.Nil, entity.ErrWorkspacePasswordAuthDisabled
	}

	if err := s.validateNewPassword(ctx, "password", account, password); err != nil {
		return uuid.Nil, err
	}

	hash, err := entity.HashPassword(password)
	if err != nil {
		return uuid.Nil, err
	}

	account.PasswordHash = hash

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.passwordResets.MarkUsed(ctx, reset.ID, now); err != nil {
			return err
		}

		if _, err := s.accounts.Update(ctx, account); err != nil {
			return err
		}

		if err := s.recordPasswordHistory(ctx, account.ID, hash); err != nil {
			return err
		}

		return s.sessions.RevokeAllByAccountID(identity.Into(ctx, account.ID), account.ID)
	})
	if err != nil {
		return uuid.Nil, err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		Action: entity.AuditPasswordReset,
		Actor: entity.AuditActor{
			Kind:      entity.ActorKindSystem,
			AccountID: account.ID,
		},
		ResourceKind: string(entity.ResourceAccount),
		ResourceID:   account.ID,
	})

	if err := s.throttle.Clear(ctx, entity.HashSignInSubject(entity.NormalizeEmail(account.Email))); err != nil {
		return uuid.Nil, err
	}

	return account.ID, nil
}

func (s *accountsService) SendPasswordReset(ctx context.Context, resetID uuid.UUID, token string) error {
	reset, err := s.passwordResets.GetByID(ctx, resetID)
	if err != nil {
		if errors.Is(err, entity.ErrPasswordResetNotFound) {
			return nil
		}

		return err
	}

	if reset.Used() || reset.ExpiredAt(time.Now().UTC()) {
		return nil
	}

	account, err := s.accounts.GetByID(ctx, reset.AccountID)
	if err != nil {
		if errors.Is(err, entity.ErrAccountNotFound) {
			return nil
		}

		return err
	}

	message, err := buildPasswordReset(s.app.BaseURL, account.DisplayName, account.Email, token)
	if err != nil {
		return err
	}

	return s.mailer.Send(ctx, message)
}

func (s *accountsService) SendPasswordResetSSONotice(ctx context.Context, accountID uuid.UUID) error {
	account, err := s.accounts.GetByID(ctx, accountID)
	if err != nil {
		if errors.Is(err, entity.ErrAccountNotFound) {
			return nil
		}

		return err
	}

	message, err := buildPasswordResetSSONotice(account.DisplayName, account.Email)
	if err != nil {
		return err
	}

	return s.mailer.Send(ctx, message)
}
