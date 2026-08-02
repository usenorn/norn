package account_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
)

func TestChangingAPasswordRevokesEveryOtherSessionAndReturnsAReplacement(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()

	hash, err := entity.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	existing := activeAccount(accountID)
	existing.PasswordHash = hash

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(existing, nil)
	h.expectPasswordAccepted(accountID)
	h.expectPasswordRecorded(accountID)

	stored := h.accounts.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, account entity.Account) (entity.Account, error) {
			return account, nil
		})

	h.sessions.EXPECT().
		RotateAfterCredentialChange(gomock.Any(), accountID).
		Return(rotatedSession(), nil).
		After(stored)

	issued, err := h.service.ChangePassword(actingAs(accountID), accountID, password, "a brand new password")
	if err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	if issued.Token != rotatedToken {
		t.Fatalf("issued token = %q, want the rotated one", issued.Token)
	}
}

func accountWithStoredPassword(t *testing.T, accountID uuid.UUID) entity.Account {
	t.Helper()

	hash, err := entity.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	existing := activeAccount(accountID)
	existing.PasswordHash = hash

	return existing
}

func fieldCode(t *testing.T, err error) string {
	t.Helper()

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %v, want a validation error", err)
	}

	if len(validation.Fields) != 1 {
		t.Fatalf("validation carried %d fields, want 1", len(validation.Fields))
	}

	return validation.Fields[0].Code
}

func TestACompromisedPasswordIsRejectedWithABreachedFieldError(t *testing.T) {
	h := newHarness(t)
	accountID := uuid.New()

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(accountWithStoredPassword(t, accountID), nil)
	h.breaches.EXPECT().Compromised(gomock.Any(), gomock.Any()).Return(true, nil)

	_, err := h.service.ChangePassword(actingAs(accountID), accountID, password, "a brand new password")
	if code := fieldCode(t, err); code != entity.ValidationCodeBreached {
		t.Fatalf("validation code = %q, want %q", code, entity.ValidationCodeBreached)
	}
}

func TestAPasswordIsRejectedWhenTheBreachCheckCannotComplete(t *testing.T) {
	h := newHarness(t)
	accountID := uuid.New()

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(accountWithStoredPassword(t, accountID), nil)
	h.breaches.EXPECT().
		Compromised(gomock.Any(), gomock.Any()).
		Return(false, entity.ErrPasswordBreachCheckUnavailable)

	_, err := h.service.ChangePassword(actingAs(accountID), accountID, password, "a brand new password")
	if !errors.Is(err, entity.ErrPasswordBreachCheckUnavailable) {
		t.Fatalf("ChangePassword error = %v, want ErrPasswordBreachCheckUnavailable", err)
	}
}

func TestReusingAStoredPasswordIsRejected(t *testing.T) {
	h := newHarness(t)
	accountID := uuid.New()

	reused := "a previously used password"

	hash, err := entity.HashPassword(reused)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(accountWithStoredPassword(t, accountID), nil)
	h.breaches.EXPECT().Compromised(gomock.Any(), gomock.Any()).Return(false, nil)
	h.passwordHistory.EXPECT().
		ListRecentByAccountID(gomock.Any(), accountID, entity.PasswordHistoryDepth).
		Return([]entity.PasswordHistoryEntry{{AccountID: accountID, PasswordHash: hash}}, nil)

	_, err = h.service.ChangePassword(actingAs(accountID), accountID, password, reused)
	if code := fieldCode(t, err); code != entity.ValidationCodeReused {
		t.Fatalf("validation code = %q, want %q", code, entity.ValidationCodeReused)
	}
}

func TestReusingTheCurrentPasswordIsRejectedWhenNoHistoryExists(t *testing.T) {
	h := newHarness(t)
	accountID := uuid.New()

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(accountWithStoredPassword(t, accountID), nil)
	h.breaches.EXPECT().Compromised(gomock.Any(), gomock.Any()).Return(false, nil)
	h.passwordHistory.EXPECT().
		ListRecentByAccountID(gomock.Any(), accountID, entity.PasswordHistoryDepth).
		Return(nil, nil)

	_, err := h.service.ChangePassword(actingAs(accountID), accountID, password, password)
	if code := fieldCode(t, err); code != entity.ValidationCodeReused {
		t.Fatalf("validation code = %q, want %q", code, entity.ValidationCodeReused)
	}
}

func TestASuccessfulChangeRecordsTheNewHashAndPrunesTheHistory(t *testing.T) {
	h := newHarness(t)
	accountID := uuid.New()
	newPassword := "a brand new password"

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(accountWithStoredPassword(t, accountID), nil)
	h.expectPasswordAccepted(accountID)

	h.accounts.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, account entity.Account) (entity.Account, error) {
			return account, nil
		})

	var recorded entity.PasswordHistoryEntry

	h.passwordHistory.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, entry entity.PasswordHistoryEntry) (entity.PasswordHistoryEntry, error) {
			recorded = entry

			return entry, nil
		})

	h.passwordHistory.EXPECT().
		PruneByAccountID(gomock.Any(), accountID, entity.PasswordHistoryDepth).
		Return(nil)

	h.expectSessionRotated(accountID)

	if _, err := h.service.ChangePassword(actingAs(accountID), accountID, password, newPassword); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	matches, err := entity.VerifyPassword(recorded.PasswordHash, newPassword)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}

	if !matches {
		t.Fatal("the recorded history entry does not verify the new password")
	}
}

func TestChangingAPasswordVerifiesTheCurrentOneBeforeTheBreachCheck(t *testing.T) {
	h := newHarness(t)
	accountID := uuid.New()

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(accountWithStoredPassword(t, accountID), nil)

	_, err := h.service.ChangePassword(actingAs(accountID), accountID, "the wrong password", "a brand new password")
	if !errors.Is(err, entity.ErrAccountInvalidCredentials) {
		t.Fatalf("ChangePassword error = %v, want ErrAccountInvalidCredentials", err)
	}
}

func TestSettingAPasswordIsRefusedWhenEveryWorkspaceEnforcesSSO(t *testing.T) {
	h := newHarness(t)
	accountID := uuid.New()

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(activeAccount(accountID), nil)
	h.expectSSOEnforcedEverywhere(accountID)

	_, err := h.service.SetPassword(actingAs(accountID), accountID, password)
	if !errors.Is(err, entity.ErrWorkspacePasswordAuthDisabled) {
		t.Fatalf("SetPassword error = %v, want ErrWorkspacePasswordAuthDisabled", err)
	}
}

func TestSettingAPasswordIsAllowedWhenOneWorkspaceStillAcceptsPasswords(t *testing.T) {
	h := newHarness(t)
	accountID := uuid.New()

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(activeAccount(accountID), nil)
	h.authPolicies.EXPECT().
		ListEnforcementsByAccountID(gomock.Any(), accountID).
		Return([]entity.AuthEnforcement{entity.AuthEnforcementSSO, entity.AuthEnforcementAny}, nil)
	h.expectPasswordAccepted(accountID)
	h.expectPasswordRecorded(accountID)

	h.accounts.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, account entity.Account) (entity.Account, error) {
			return account, nil
		})

	h.expectSessionRotated(accountID)

	if _, err := h.service.SetPassword(actingAs(accountID), accountID, password); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}
}

func TestAFailedPasswordChangeLeavesEverySessionAlone(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()

	hash, err := entity.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	existing := activeAccount(accountID)
	existing.PasswordHash = hash

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(existing, nil)

	if _, err := h.service.ChangePassword(actingAs(accountID), accountID, "the wrong password", "a brand new password"); err == nil {
		t.Fatal("ChangePassword accepted a wrong current password")
	}
}
