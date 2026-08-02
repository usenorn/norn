package account_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

const resetEmail = "ada@example.com"

func resetInput() service.RequestPasswordResetInput {
	return service.RequestPasswordResetInput{Email: resetEmail}
}

func (h *harness) expectAddressAllowed() {
	h.throttle.EXPECT().RecordAddressAttempt(gomock.Any(), gomock.Any()).Return(1, nil)
}

func TestRequestingAResetForAnUnknownAddressLooksIdenticalToASuccessfulOne(t *testing.T) {
	known := newHarness(t)
	accountID := uuid.New()

	known.expectAddressAllowed()
	known.accounts.EXPECT().GetByEmail(gomock.Any(), resetEmail).Return(activeAccount(accountID), nil)
	known.expectNoSSOEnforcement(accountID)
	known.passwordResets.EXPECT().DeletePendingByAccountID(gomock.Any(), accountID).Return(nil)
	known.passwordResets.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, reset entity.PasswordReset) (entity.PasswordReset, error) {
			reset.ID = uuid.New()

			return reset, nil
		})
	known.producer.EXPECT().EnqueuePasswordReset(gomock.Any(), gomock.Any()).Return(nil)

	knownExpiry, knownErr := known.service.RequestPasswordReset(context.Background(), resetInput())

	unknown := newHarness(t)

	unknown.expectAddressAllowed()
	unknown.accounts.EXPECT().
		GetByEmail(gomock.Any(), resetEmail).
		Return(entity.Account{}, entity.ErrAccountNotFound)

	unknownExpiry, unknownErr := unknown.service.RequestPasswordReset(context.Background(), resetInput())

	if knownErr != nil || unknownErr != nil {
		t.Fatalf("errors differ: known = %v, unknown = %v", knownErr, unknownErr)
	}

	if knownExpiry.IsZero() || unknownExpiry.IsZero() {
		t.Fatal("one of the responses carried no expiry")
	}

	if delta := knownExpiry.Sub(unknownExpiry); delta > time.Second || delta < -time.Second {
		t.Fatalf("expiries differ by %s, want them indistinguishable", delta)
	}
}

func TestRequestingAResetSupersedesAnyPendingLink(t *testing.T) {
	h := newHarness(t)
	accountID := uuid.New()

	h.expectAddressAllowed()
	h.accounts.EXPECT().GetByEmail(gomock.Any(), resetEmail).Return(activeAccount(accountID), nil)
	h.expectNoSSOEnforcement(accountID)

	deleted := h.passwordResets.EXPECT().DeletePendingByAccountID(gomock.Any(), accountID).Return(nil)

	h.passwordResets.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, reset entity.PasswordReset) (entity.PasswordReset, error) {
			reset.ID = uuid.New()

			return reset, nil
		}).
		After(deleted)

	h.producer.EXPECT().EnqueuePasswordReset(gomock.Any(), gomock.Any()).Return(nil)

	if _, err := h.service.RequestPasswordReset(context.Background(), resetInput()); err != nil {
		t.Fatalf("RequestPasswordReset: %v", err)
	}
}

func TestTheEnqueuedResetTokenMatchesTheStoredHash(t *testing.T) {
	h := newHarness(t)
	accountID := uuid.New()

	h.expectAddressAllowed()
	h.accounts.EXPECT().GetByEmail(gomock.Any(), resetEmail).Return(activeAccount(accountID), nil)
	h.expectNoSSOEnforcement(accountID)
	h.passwordResets.EXPECT().DeletePendingByAccountID(gomock.Any(), accountID).Return(nil)

	var stored entity.PasswordReset

	h.passwordResets.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, reset entity.PasswordReset) (entity.PasswordReset, error) {
			reset.ID = uuid.New()
			stored = reset

			return reset, nil
		})

	var enqueued entity.PasswordResetPayload

	h.producer.EXPECT().
		EnqueuePasswordReset(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload entity.PasswordResetPayload) error {
			enqueued = payload

			return nil
		})

	if _, err := h.service.RequestPasswordReset(context.Background(), resetInput()); err != nil {
		t.Fatalf("RequestPasswordReset: %v", err)
	}

	if enqueued.Token == "" {
		t.Fatal("no raw token reached the job")
	}

	if !bytes.Equal(entity.HashPasswordResetToken(enqueued.Token), stored.TokenHash) {
		t.Fatal("the stored hash is not the hash of the enqueued token")
	}

	if bytes.Contains(stored.TokenHash, []byte(enqueued.Token)) {
		t.Fatal("the raw token was persisted")
	}
}

func TestRequestingAResetForAnSSOOnlyAccountSendsAnExplanationInsteadOfALink(t *testing.T) {
	h := newHarness(t)
	accountID := uuid.New()

	h.expectAddressAllowed()
	h.accounts.EXPECT().GetByEmail(gomock.Any(), resetEmail).Return(activeAccount(accountID), nil)
	h.expectSSOEnforcedEverywhere(accountID)

	h.producer.EXPECT().
		EnqueuePasswordResetSSONotice(gomock.Any(), entity.PasswordResetSSONoticePayload{AccountID: accountID}).
		Return(nil)

	if _, err := h.service.RequestPasswordReset(context.Background(), resetInput()); err != nil {
		t.Fatalf("RequestPasswordReset: %v", err)
	}
}

func TestRequestingAResetFailsLoudlyWhenMailDeliveryIsNotConfigured(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := newHarness(t)

	h.service = newServiceWithSMTP(h, config.SMTP{})

	h.expectAddressAllowed()

	_, err := h.service.RequestPasswordReset(context.Background(), resetInput())
	if !errors.Is(err, entity.ErrMailDeliveryNotConfigured) {
		t.Fatalf("RequestPasswordReset error = %v, want ErrMailDeliveryNotConfigured", err)
	}

	ctrl.Finish()
}

func TestRequestingAResetIsRefusedOnceTheAddressLimitIsPassed(t *testing.T) {
	h := newHarness(t)

	h.throttle.EXPECT().
		RecordAddressAttempt(gomock.Any(), gomock.Any()).
		Return(entity.SignInAddressMaxAttempts+1, nil)

	_, err := h.service.RequestPasswordReset(context.Background(), resetInput())
	if !errors.Is(err, entity.ErrSignInRateLimited) {
		t.Fatalf("RequestPasswordReset error = %v, want ErrSignInRateLimited", err)
	}
}

func TestConfirmingAResetMarksTheLinkUsedAndEndsEverySession(t *testing.T) {
	h := newHarness(t)
	accountID := uuid.New()
	token := "a-reset-token"

	h.passwordResets.EXPECT().
		GetByTokenHash(gomock.Any(), entity.HashPasswordResetToken(token)).
		Return(pendingReset(accountID), nil)

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(activeAccount(accountID), nil)
	h.expectNoSSOEnforcement(accountID)
	h.expectPasswordAccepted(accountID)

	used := h.passwordResets.EXPECT().MarkUsed(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	h.accounts.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, account entity.Account) (entity.Account, error) {
			return account, nil
		}).
		After(used)

	h.expectPasswordRecorded(accountID)
	h.expectSessionsRevoked(accountID)
	h.throttle.EXPECT().Clear(gomock.Any(), entity.HashSignInSubject(resetEmail)).Return(nil)

	if err := h.service.ConfirmPasswordReset(context.Background(), token, "a brand new password"); err != nil {
		t.Fatalf("ConfirmPasswordReset: %v", err)
	}
}

func TestConfirmingAResetStoresAVerifiableHashOfTheNewPassword(t *testing.T) {
	h := newHarness(t)
	accountID := uuid.New()
	token := "a-reset-token"
	newPassword := "a brand new password"

	h.passwordResets.EXPECT().
		GetByTokenHash(gomock.Any(), gomock.Any()).
		Return(pendingReset(accountID), nil)

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(activeAccount(accountID), nil)
	h.expectNoSSOEnforcement(accountID)
	h.expectPasswordAccepted(accountID)
	h.passwordResets.EXPECT().MarkUsed(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	var captured entity.Account

	h.accounts.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, account entity.Account) (entity.Account, error) {
			captured = account

			return account, nil
		})

	h.expectPasswordRecorded(accountID)
	h.expectSessionsRevoked(accountID)
	h.throttle.EXPECT().Clear(gomock.Any(), gomock.Any()).Return(nil)

	if err := h.service.ConfirmPasswordReset(context.Background(), token, newPassword); err != nil {
		t.Fatalf("ConfirmPasswordReset: %v", err)
	}

	matches, err := entity.VerifyPassword(captured.PasswordHash, newPassword)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}

	if !matches {
		t.Fatal("the stored hash does not verify the new password")
	}
}

func TestAnExpiredResetLinkLeavesThePasswordUnchanged(t *testing.T) {
	h := newHarness(t)
	accountID := uuid.New()

	expired := pendingReset(accountID)
	expired.ExpiresAt = time.Now().UTC().Add(-time.Minute)

	h.passwordResets.EXPECT().GetByTokenHash(gomock.Any(), gomock.Any()).Return(expired, nil)

	err := h.service.ConfirmPasswordReset(context.Background(), "a-reset-token", "a brand new password")
	if !errors.Is(err, entity.ErrPasswordResetExpired) {
		t.Fatalf("ConfirmPasswordReset error = %v, want ErrPasswordResetExpired", err)
	}
}

func TestAnAlreadyUsedResetLinkIsRejected(t *testing.T) {
	h := newHarness(t)
	accountID := uuid.New()

	usedAt := time.Now().UTC().Add(-time.Minute)
	used := pendingReset(accountID)
	used.UsedAt = &usedAt

	h.passwordResets.EXPECT().GetByTokenHash(gomock.Any(), gomock.Any()).Return(used, nil)

	err := h.service.ConfirmPasswordReset(context.Background(), "a-reset-token", "a brand new password")
	if !errors.Is(err, entity.ErrPasswordResetAlreadyUsed) {
		t.Fatalf("ConfirmPasswordReset error = %v, want ErrPasswordResetAlreadyUsed", err)
	}
}

func TestAnEmptyResetTokenIsRejectedWithoutALookup(t *testing.T) {
	h := newHarness(t)

	err := h.service.ConfirmPasswordReset(context.Background(), "", "a brand new password")
	if !errors.Is(err, entity.ErrPasswordResetTokenInvalid) {
		t.Fatalf("ConfirmPasswordReset error = %v, want ErrPasswordResetTokenInvalid", err)
	}
}

func TestConfirmingAResetIsRefusedWhenEveryWorkspaceEnforcesSSO(t *testing.T) {
	h := newHarness(t)
	accountID := uuid.New()

	h.passwordResets.EXPECT().GetByTokenHash(gomock.Any(), gomock.Any()).Return(pendingReset(accountID), nil)
	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(activeAccount(accountID), nil)
	h.expectSSOEnforcedEverywhere(accountID)

	err := h.service.ConfirmPasswordReset(context.Background(), "a-reset-token", "a brand new password")
	if !errors.Is(err, entity.ErrWorkspacePasswordAuthDisabled) {
		t.Fatalf("ConfirmPasswordReset error = %v, want ErrWorkspacePasswordAuthDisabled", err)
	}
}

func TestTheResetEmailCarriesTheTokenAndTheConfiguredBaseURL(t *testing.T) {
	h := newHarness(t)
	accountID := uuid.New()
	resetID := uuid.New()
	token := "a-reset-token"

	reset := pendingReset(accountID)
	reset.ID = resetID

	h.passwordResets.EXPECT().GetByID(gomock.Any(), resetID).Return(reset, nil)
	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(activeAccount(accountID), nil)

	var sent entity.Mail

	h.mailer.EXPECT().
		Send(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, mail entity.Mail) error {
			sent = mail

			return nil
		})

	if err := h.service.SendPasswordReset(context.Background(), resetID, token); err != nil {
		t.Fatalf("SendPasswordReset: %v", err)
	}

	if sent.To != resetEmail {
		t.Fatalf("mail addressed to %q, want %q", sent.To, resetEmail)
	}

	if !strings.Contains(sent.PlainBody, token) {
		t.Fatal("the reset mail does not carry the token")
	}

	if !strings.Contains(sent.PlainBody, baseURL) {
		t.Fatal("the reset mail does not carry the configured base url")
	}

	if !strings.Contains(sent.PlainBody, "/reset-password") {
		t.Fatal("the reset mail does not point at the reset screen")
	}
}

func TestSendingAResetIsDroppedForAUsedLink(t *testing.T) {
	h := newHarness(t)
	accountID := uuid.New()
	resetID := uuid.New()

	usedAt := time.Now().UTC()
	used := pendingReset(accountID)
	used.ID = resetID
	used.UsedAt = &usedAt

	h.passwordResets.EXPECT().GetByID(gomock.Any(), resetID).Return(used, nil)

	if err := h.service.SendPasswordReset(context.Background(), resetID, "a-reset-token"); err != nil {
		t.Fatalf("SendPasswordReset: %v", err)
	}
}

func TestSendingAResetIsDroppedForAVanishedLink(t *testing.T) {
	h := newHarness(t)
	resetID := uuid.New()

	h.passwordResets.EXPECT().
		GetByID(gomock.Any(), resetID).
		Return(entity.PasswordReset{}, entity.ErrPasswordResetNotFound)

	if err := h.service.SendPasswordReset(context.Background(), resetID, "a-reset-token"); err != nil {
		t.Fatalf("SendPasswordReset: %v", err)
	}
}

func TestTheSSONoticeExplainsThatThereIsNoPasswordToReset(t *testing.T) {
	h := newHarness(t)
	accountID := uuid.New()

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(activeAccount(accountID), nil)

	var sent entity.Mail

	h.mailer.EXPECT().
		Send(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, mail entity.Mail) error {
			sent = mail

			return nil
		})

	if err := h.service.SendPasswordResetSSONotice(context.Background(), accountID); err != nil {
		t.Fatalf("SendPasswordResetSSONotice: %v", err)
	}

	if !strings.Contains(sent.PlainBody, "single sign-on") {
		t.Fatal("the sso notice does not mention single sign-on")
	}

	if strings.Contains(sent.PlainBody, "/reset-password") {
		t.Fatal("the sso notice carries a reset link")
	}
}

func pendingReset(accountID uuid.UUID) entity.PasswordReset {
	now := time.Now().UTC()

	return entity.PasswordReset{
		ID:          uuid.New(),
		AccountID:   accountID,
		RequestedAt: now,
		ExpiresAt:   now.Add(entity.PasswordResetTokenTTL),
	}
}
