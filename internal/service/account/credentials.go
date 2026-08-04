package account

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (s *accountsService) SetPassword(ctx context.Context, accountID uuid.UUID, password string) (service.IssuedSession, error) {
	if err := s.authorizeSelf(ctx, entity.ActionUpdate, accountID); err != nil {
		return service.IssuedSession{}, err
	}

	account, err := s.activeAccount(ctx, accountID)
	if err != nil {
		return service.IssuedSession{}, err
	}

	if account.HasPassword() {
		return service.IssuedSession{}, entity.ErrAccountPasswordSet
	}

	ssoOnly, err := s.ssoEnforcedEverywhere(ctx, accountID)
	if err != nil {
		return service.IssuedSession{}, err
	}

	if ssoOnly {
		return service.IssuedSession{}, entity.ErrWorkspacePasswordAuthDisabled
	}

	if err := s.validateNewPassword(ctx, "password", account, password); err != nil {
		return service.IssuedSession{}, err
	}

	return s.storePassword(ctx, account, password)
}

func (s *accountsService) ChangePassword(ctx context.Context, accountID uuid.UUID, currentPassword, newPassword string) (service.IssuedSession, error) {
	if err := s.authorizeSelf(ctx, entity.ActionUpdate, accountID); err != nil {
		return service.IssuedSession{}, err
	}

	account, err := s.activeAccount(ctx, accountID)
	if err != nil {
		return service.IssuedSession{}, err
	}

	if !account.HasPassword() {
		return service.IssuedSession{}, entity.ErrAccountPasswordNotSet
	}

	matches, err := entity.VerifyPassword(account.PasswordHash, currentPassword)
	if err != nil {
		return service.IssuedSession{}, err
	}

	if !matches {
		return service.IssuedSession{}, entity.ErrAccountInvalidCredentials
	}

	if err := s.validateNewPassword(ctx, "new_password", account, newPassword); err != nil {
		return service.IssuedSession{}, err
	}

	return s.storePassword(ctx, account, newPassword)
}

func (s *accountsService) screenPassword(ctx context.Context, field, password string) error {
	if err := entity.NewValidationError(entity.ValidatePassword(field, password)); err != nil {
		return err
	}

	compromised, err := s.breaches.Compromised(ctx, password)
	if err != nil {
		return err
	}

	if compromised {
		return entity.NewValidationError(entity.FieldError{Field: field, Code: entity.ValidationCodeBreached})
	}

	return nil
}

func (s *accountsService) validateNewPassword(ctx context.Context, field string, account entity.Account, password string) error {
	if err := s.screenPassword(ctx, field, password); err != nil {
		return err
	}

	reused, err := s.passwordReused(ctx, account, password)
	if err != nil {
		return err
	}

	if reused {
		return entity.NewValidationError(entity.FieldError{Field: field, Code: entity.ValidationCodeReused})
	}

	return nil
}

func (s *accountsService) passwordReused(ctx context.Context, account entity.Account, password string) (bool, error) {
	entries, err := s.passwordHistory.ListRecentByAccountID(ctx, account.ID, entity.PasswordHistoryDepth)
	if err != nil {
		return false, err
	}

	hashes := make([]string, 0, len(entries)+1)

	for _, entry := range entries {
		hashes = append(hashes, entry.PasswordHash)
	}

	if len(entries) == 0 && account.HasPassword() {
		hashes = append(hashes, account.PasswordHash)
	}

	for _, hash := range hashes {
		matches, err := entity.VerifyPassword(hash, password)
		if err != nil {
			if errors.Is(err, entity.ErrPasswordHashMalformed) {
				continue
			}

			return false, err
		}

		if matches {
			return true, nil
		}
	}

	return false, nil
}

func (s *accountsService) storePassword(ctx context.Context, account entity.Account, password string) (service.IssuedSession, error) {
	hash, err := entity.HashPassword(password)
	if err != nil {
		return service.IssuedSession{}, err
	}

	account.PasswordHash = hash

	var issued service.IssuedSession

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if _, err := s.accounts.Update(ctx, account); err != nil {
			return err
		}

		if err := s.recordPasswordHistory(ctx, account.ID, hash); err != nil {
			return err
		}

		rotated, err := s.sessions.RotateAfterCredentialChange(ctx, account.ID)
		if err != nil {
			return err
		}

		issued = rotated

		return nil
	})
	if err != nil {
		return service.IssuedSession{}, err
	}

	return issued, nil
}

func (s *accountsService) recordPasswordHistory(ctx context.Context, accountID uuid.UUID, hash string) error {
	if _, err := s.passwordHistory.Create(ctx, entity.PasswordHistoryEntry{
		AccountID:    accountID,
		PasswordHash: hash,
	}); err != nil {
		return err
	}

	return s.passwordHistory.PruneByAccountID(ctx, accountID, entity.PasswordHistoryDepth)
}

func (s *accountsService) ssoEnforcedEverywhere(ctx context.Context, accountID uuid.UUID) (bool, error) {
	enforcements, err := s.authPolicies.ListEnforcementsByAccountID(ctx, accountID)
	if err != nil {
		return false, err
	}

	return entity.SSOEnforcedEverywhere(enforcements), nil
}
