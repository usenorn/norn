package account_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

var pngContent = []byte{
	0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A,
	0x00, 0x00, 0x00, 0x0D, 'I', 'H', 'D', 'R',
}

func TestUpdateProfileLeavesUnsuppliedFieldsUntouched(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()
	before := activeAccount(accountID)

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(before, nil)

	var captured entity.Account

	h.accounts.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, account entity.Account) (entity.Account, error) {
			captured = account

			return account, nil
		})

	name := "Ada B. Lovelace"

	if _, err := h.service.UpdateProfile(actingAs(accountID), accountID, service.UpdateProfileInput{
		DisplayName: &name,
	}); err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}

	if captured.DisplayName != name {
		t.Fatalf("display name = %q, want %q", captured.DisplayName, name)
	}

	if captured.Timezone != before.Timezone {
		t.Fatalf("timezone = %q, want it untouched at %q", captured.Timezone, before.Timezone)
	}
}

func TestUpdateProfileRejectsAnUnknownTimezoneBeforeWriting(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(activeAccount(accountID), nil)

	zone := "Mars/Olympus"

	_, err := h.service.UpdateProfile(actingAs(accountID), accountID, service.UpdateProfileInput{Timezone: &zone})

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("UpdateProfile error = %v, want a ValidationError", err)
	}
}

func TestUpdateProfileIsRefusedForADeactivatedAccount(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()

	deactivated := activeAccount(accountID)
	deactivated.Status = entity.AccountStatusDeactivated

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(deactivated, nil)

	name := "Ada B. Lovelace"

	_, err := h.service.UpdateProfile(actingAs(accountID), accountID, service.UpdateProfileInput{DisplayName: &name})
	if !errors.Is(err, entity.ErrAccountDeactivated) {
		t.Fatalf("UpdateProfile error = %v, want ErrAccountDeactivated", err)
	}
}

func TestUploadAvatarRejectsAnOversizedDeclaredLength(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()

	_, err := h.service.UploadAvatar(actingAs(accountID), accountID, service.AvatarUpload{
		DeclaredSize: entity.AvatarMaxBytes + 1,
		Body:         bytes.NewReader(pngContent),
	})
	if !errors.Is(err, entity.ErrAvatarTooLarge) {
		t.Fatalf("UploadAvatar error = %v, want ErrAvatarTooLarge", err)
	}
}

func TestUploadAvatarRejectsContentThatIsNotASupportedImage(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(activeAccount(accountID), nil)

	_, err := h.service.UploadAvatar(actingAs(accountID), accountID, service.AvatarUpload{
		Body: strings.NewReader("%PDF-1.7 not an image at all"),
	})
	if !errors.Is(err, entity.ErrAvatarUnsupportedType) {
		t.Fatalf("UploadAvatar error = %v, want ErrAvatarUnsupportedType", err)
	}
}

func TestUploadAvatarStoresTheObjectThenReplacesThePreviousOne(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()

	before := activeAccount(accountID)
	before.AvatarObjectKey = "avatars/previous.png"

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(before, nil)

	var storedKey string

	put := h.blobs.EXPECT().
		Put(gomock.Any(), gomock.Any(), "image/png", gomock.Any(), int64(len(pngContent))).
		DoAndReturn(func(_ context.Context, key, _ string, _ any, _ int64) error {
			storedKey = key

			return nil
		})

	update := h.accounts.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, account entity.Account) (entity.Account, error) {
			if account.AvatarObjectKey != storedKey {
				t.Errorf("account points at %q, want the freshly stored %q", account.AvatarObjectKey, storedKey)
			}

			return account, nil
		}).
		After(put)

	h.blobs.EXPECT().Delete(gomock.Any(), "avatars/previous.png").Return(nil).After(update)

	if _, err := h.service.UploadAvatar(actingAs(accountID), accountID, service.AvatarUpload{
		Body: bytes.NewReader(pngContent),
	}); err != nil {
		t.Fatalf("UploadAvatar: %v", err)
	}
}

func TestUploadAvatarRemovesTheUploadedObjectWhenTheProfileUpdateFails(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(activeAccount(accountID), nil)

	var storedKey string

	h.blobs.EXPECT().
		Put(gomock.Any(), gomock.Any(), "image/png", gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, key, _ string, _ any, _ int64) error {
			storedKey = key

			return nil
		})

	h.accounts.EXPECT().Update(gomock.Any(), gomock.Any()).Return(entity.Account{}, errors.New("write failed"))

	h.blobs.EXPECT().
		Delete(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, key string) error {
			if key != storedKey {
				t.Errorf("cleaned up %q, want the orphaned upload %q", key, storedKey)
			}

			return nil
		})

	if _, err := h.service.UploadAvatar(actingAs(accountID), accountID, service.AvatarUpload{
		Body: bytes.NewReader(pngContent),
	}); err == nil {
		t.Fatal("UploadAvatar returned no error when the profile update failed")
	}
}

func TestSetPasswordIsRefusedWhenAPasswordAlreadyExists(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()

	existing := activeAccount(accountID)
	existing.PasswordHash = "$argon2id$stored"

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(existing, nil)

	if _, err := h.service.SetPassword(actingAs(accountID), accountID, password); !errors.Is(err, entity.ErrAccountPasswordSet) {
		t.Fatalf("SetPassword error = %v, want ErrAccountPasswordSet", err)
	}
}

func TestSetPasswordEstablishesAHashForAnAccountCreatedWithout(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(activeAccount(accountID), nil)
	h.expectNoSSOEnforcement(accountID)
	h.expectPasswordAccepted(accountID)
	h.expectPasswordRecorded(accountID)

	var captured entity.Account

	h.accounts.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, account entity.Account) (entity.Account, error) {
			captured = account

			return account, nil
		})

	h.expectSessionRotated(accountID)

	if _, err := h.service.SetPassword(actingAs(accountID), accountID, password); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}

	matches, err := entity.VerifyPassword(captured.PasswordHash, password)
	if err != nil || !matches {
		t.Fatalf("the stored hash does not verify the new password: %v", err)
	}
}

func TestChangePasswordIsRefusedWhenNoPasswordIsSet(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(activeAccount(accountID), nil)

	_, err := h.service.ChangePassword(actingAs(accountID), accountID, password, "another password value")
	if !errors.Is(err, entity.ErrAccountPasswordNotSet) {
		t.Fatalf("ChangePassword error = %v, want ErrAccountPasswordNotSet", err)
	}
}

func TestChangePasswordRejectsAWrongCurrentPasswordWithoutWriting(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()

	hash, err := entity.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	existing := activeAccount(accountID)
	existing.PasswordHash = hash

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(existing, nil)

	_, err = h.service.ChangePassword(actingAs(accountID), accountID, "the wrong password", "another password value")
	if !errors.Is(err, entity.ErrAccountInvalidCredentials) {
		t.Fatalf("ChangePassword error = %v, want ErrAccountInvalidCredentials", err)
	}
}
