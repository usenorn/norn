package account_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
)

func TestRequestEmailChangeLeavesTheCurrentAddressActive(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()
	account := activeAccount(accountID)

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(account, nil)
	h.accounts.EXPECT().GetByEmail(gomock.Any(), "grace@example.com").Return(entity.Account{}, entity.ErrAccountNotFound)
	h.emailChanges.EXPECT().DeletePendingByAccountID(gomock.Any(), accountID).Return(nil)

	var captured entity.EmailChange

	h.emailChanges.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, change entity.EmailChange) (entity.EmailChange, error) {
			captured = change
			change.ID = uuid.New()

			return change, nil
		})

	h.producer.EXPECT().
		EnqueueEmailChangeConfirmation(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload entity.EmailChangeConfirmationPayload) error {
			if !bytes.Equal(entity.HashEmailChangeToken(payload.Token), captured.TokenHash) {
				t.Error("the enqueued token does not match the stored token hash")
			}

			return nil
		})

	change, err := h.service.RequestEmailChange(actingAs(accountID), accountID, "Grace@Example.com")
	if err != nil {
		t.Fatalf("RequestEmailChange: %v", err)
	}

	if change.NewEmail != "grace@example.com" {
		t.Fatalf("pending address = %q, want normalized %q", change.NewEmail, "grace@example.com")
	}

	if captured.TokenHash == nil {
		t.Fatal("the raw token was stored instead of its hash")
	}
}

func TestRequestEmailChangeRejectsAnAddressAlreadyInUse(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(activeAccount(accountID), nil)
	h.accounts.EXPECT().GetByEmail(gomock.Any(), "grace@example.com").Return(activeAccount(uuid.New()), nil)

	_, err := h.service.RequestEmailChange(actingAs(accountID), accountID, "grace@example.com")
	if !errors.Is(err, entity.ErrAccountEmailTaken) {
		t.Fatalf("RequestEmailChange error = %v, want ErrAccountEmailTaken", err)
	}
}

func TestRequestEmailChangeRejectsTheCurrentAddress(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(activeAccount(accountID), nil)

	_, err := h.service.RequestEmailChange(actingAs(accountID), accountID, "ADA@example.com")
	if !errors.Is(err, entity.ErrEmailChangeSameAddress) {
		t.Fatalf("RequestEmailChange error = %v, want ErrEmailChangeSameAddress", err)
	}
}

func TestConfirmEmailChangeLooksTheTokenUpByItsHashAndWritesTheNewAddress(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()
	changeID := uuid.New()

	const token = "a-confirmation-token"

	h.emailChanges.EXPECT().
		GetByTokenHash(gomock.Any(), entity.HashEmailChangeToken(token)).
		Return(entity.EmailChange{
			ID:        changeID,
			AccountID: accountID,
			NewEmail:  "grace@example.com",
			ExpiresAt: time.Now().UTC().Add(time.Hour),
		}, nil)

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(activeAccount(accountID), nil)

	var captured entity.Account

	h.accounts.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, account entity.Account) (entity.Account, error) {
			captured = account

			return account, nil
		})

	h.emailChanges.EXPECT().MarkConfirmed(gomock.Any(), changeID, gomock.Any()).Return(nil)

	account, err := h.service.ConfirmEmailChange(context.Background(), token)
	if err != nil {
		t.Fatalf("ConfirmEmailChange: %v", err)
	}

	if captured.Email != "grace@example.com" {
		t.Fatalf("written email = %q, want %q", captured.Email, "grace@example.com")
	}

	if account.Email != "grace@example.com" {
		t.Fatalf("returned email = %q, want %q", account.Email, "grace@example.com")
	}
}

func TestConfirmEmailChangeWithAnExpiredTokenLeavesTheAddressUnchanged(t *testing.T) {
	h := newHarness(t)

	const token = "an-expired-token"

	h.emailChanges.EXPECT().
		GetByTokenHash(gomock.Any(), entity.HashEmailChangeToken(token)).
		Return(entity.EmailChange{
			ID:        uuid.New(),
			AccountID: uuid.New(),
			NewEmail:  "grace@example.com",
			ExpiresAt: time.Now().UTC().Add(-time.Second),
		}, nil)

	_, err := h.service.ConfirmEmailChange(context.Background(), token)
	if !errors.Is(err, entity.ErrEmailChangeExpired) {
		t.Fatalf("ConfirmEmailChange error = %v, want ErrEmailChangeExpired", err)
	}
}

func TestConfirmEmailChangeWithAnAlreadyConfirmedTokenIsRejected(t *testing.T) {
	h := newHarness(t)

	const token = "a-used-token"

	confirmedAt := time.Now().UTC().Add(-time.Hour)

	h.emailChanges.EXPECT().
		GetByTokenHash(gomock.Any(), entity.HashEmailChangeToken(token)).
		Return(entity.EmailChange{
			ID:          uuid.New(),
			AccountID:   uuid.New(),
			NewEmail:    "grace@example.com",
			ExpiresAt:   time.Now().UTC().Add(time.Hour),
			ConfirmedAt: &confirmedAt,
		}, nil)

	_, err := h.service.ConfirmEmailChange(context.Background(), token)
	if !errors.Is(err, entity.ErrEmailChangeAlreadyDone) {
		t.Fatalf("ConfirmEmailChange error = %v, want ErrEmailChangeAlreadyDone", err)
	}
}

func TestConfirmEmailChangePassesTheEmailTakenSentinelThrough(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()

	const token = "a-racing-token"

	h.emailChanges.EXPECT().
		GetByTokenHash(gomock.Any(), gomock.Any()).
		Return(entity.EmailChange{
			ID:        uuid.New(),
			AccountID: accountID,
			NewEmail:  "grace@example.com",
			ExpiresAt: time.Now().UTC().Add(time.Hour),
		}, nil)

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(activeAccount(accountID), nil)
	h.accounts.EXPECT().Update(gomock.Any(), gomock.Any()).Return(entity.Account{}, entity.ErrAccountEmailTaken)

	_, err := h.service.ConfirmEmailChange(context.Background(), token)
	if !errors.Is(err, entity.ErrAccountEmailTaken) {
		t.Fatalf("ConfirmEmailChange error = %v, want ErrAccountEmailTaken", err)
	}
}

func TestSendEmailChangeConfirmationAddressesTheNewAddressAndCarriesTheToken(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()
	changeID := uuid.New()

	const token = "a-confirmation-token"

	h.emailChanges.EXPECT().GetByID(gomock.Any(), changeID).Return(entity.EmailChange{
		ID:        changeID,
		AccountID: accountID,
		NewEmail:  "grace@example.com",
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}, nil)

	h.accounts.EXPECT().GetByID(gomock.Any(), accountID).Return(activeAccount(accountID), nil)

	var captured entity.Mail

	h.mailer.EXPECT().
		Send(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, mail entity.Mail) error {
			captured = mail

			return nil
		})

	if err := h.service.SendEmailChangeConfirmation(context.Background(), changeID, token); err != nil {
		t.Fatalf("SendEmailChangeConfirmation: %v", err)
	}

	if captured.To != "grace@example.com" {
		t.Fatalf("recipient = %q, want the new address", captured.To)
	}

	if !bytes.Contains([]byte(captured.PlainBody), []byte(token)) {
		t.Fatal("the confirmation body does not carry the token")
	}

	if !bytes.Contains([]byte(captured.PlainBody), []byte(baseURL)) {
		t.Fatal("the confirmation body does not use the configured base url")
	}
}

func TestSendEmailChangeConfirmationIsDroppedForAConfirmedChange(t *testing.T) {
	h := newHarness(t)

	changeID := uuid.New()
	confirmedAt := time.Now().UTC()

	h.emailChanges.EXPECT().GetByID(gomock.Any(), changeID).Return(entity.EmailChange{
		ID:          changeID,
		ExpiresAt:   time.Now().UTC().Add(time.Hour),
		ConfirmedAt: &confirmedAt,
	}, nil)

	if err := h.service.SendEmailChangeConfirmation(context.Background(), changeID, "token"); err != nil {
		t.Fatalf("SendEmailChangeConfirmation on a confirmed change = %v, want nil", err)
	}
}

func TestSendEmailChangeConfirmationIsDroppedForAVanishedChange(t *testing.T) {
	h := newHarness(t)

	changeID := uuid.New()

	h.emailChanges.EXPECT().GetByID(gomock.Any(), changeID).Return(entity.EmailChange{}, entity.ErrEmailChangeNotFound)

	if err := h.service.SendEmailChangeConfirmation(context.Background(), changeID, "token"); err != nil {
		t.Fatalf("SendEmailChangeConfirmation on a missing change = %v, want nil", err)
	}
}
