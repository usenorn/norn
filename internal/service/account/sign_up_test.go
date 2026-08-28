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

const (
	signUpEmail    = "rae@example.com"
	signUpName     = "Rae Okafor"
	signUpPassword = "an-unguessed-passphrase"
)

func signUpInput() service.RequestSignUpInput {
	return service.RequestSignUpInput{
		Email:       "  Rae@Example.com ",
		DisplayName: signUpName,
		Timezone:    "Europe/London",
		Password:    signUpPassword,
	}
}

func (h *harness) expectAddressFree() {
	h.accounts.EXPECT().
		GetByEmail(gomock.Any(), signUpEmail).
		Return(entity.Account{}, entity.ErrAccountNotFound)
}

func (h *harness) expectSignUpStored(stored *entity.SignUp) {
	h.signUps.EXPECT().DeletePendingByEmail(gomock.Any(), signUpEmail).Return(nil)
	h.signUps.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, signUp entity.SignUp) (entity.SignUp, error) {
			signUp.ID = uuid.New()
			*stored = signUp

			return signUp, nil
		})
}

func pendingSignUp() entity.SignUp {
	hash, err := entity.HashPassword(signUpPassword)
	if err != nil {
		panic(err)
	}

	now := time.Now().UTC()

	return entity.SignUp{
		ID:           uuid.New(),
		Email:        signUpEmail,
		DisplayName:  signUpName,
		Timezone:     "Europe/London",
		PasswordHash: hash,
		TokenHash:    entity.HashSignUpToken("a-pending-token"),
		RequestedAt:  now,
		ExpiresAt:    now.Add(entity.SignUpTokenTTL),
	}
}

func (h *harness) expectSessionStarted(accountID uuid.UUID) service.IssuedSession {
	issued := service.IssuedSession{
		Session: entity.Session{ID: uuid.New(), AccountID: accountID},
		Token:   "a-fresh-session-token",
	}

	h.sessions.EXPECT().
		Start(gomock.Any(), service.StartSessionInput{
			AccountID:  accountID,
			AuthMethod: entity.SessionAuthMethodPassword,
		}).
		Return(issued, nil)

	return issued
}

func TestSigningUpStoresAPendingRowAndCreatesNoAccount(t *testing.T) {
	h := newHarness(t)

	var stored entity.SignUp

	h.expectAddressAllowed()
	h.expectAddressFree()
	h.breaches.EXPECT().Compromised(gomock.Any(), signUpPassword).Return(false, nil)
	h.expectSignUpStored(&stored)
	h.producer.EXPECT().EnqueueSignUpVerification(gomock.Any(), gomock.Any()).Return(nil)

	if _, err := h.service.RequestSignUp(context.Background(), signUpInput()); err != nil {
		t.Fatalf("RequestSignUp: %v", err)
	}

	if stored.Email != signUpEmail {
		t.Fatalf("stored email = %q, want the normalized %q", stored.Email, signUpEmail)
	}

	if !strings.HasPrefix(stored.PasswordHash, "$argon2id$") {
		t.Fatalf("stored password hash = %q, want an argon2id hash", stored.PasswordHash)
	}

	matches, err := entity.VerifyPassword(stored.PasswordHash, signUpPassword)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}

	if !matches {
		t.Fatal("the stored hash does not verify the password that was signed up with")
	}

	if ttl := stored.ExpiresAt.Sub(stored.RequestedAt); ttl != entity.SignUpTokenTTL {
		t.Fatalf("link lasts %s, want %s", ttl, entity.SignUpTokenTTL)
	}
}

func TestTheEnqueuedSignUpTokenMatchesTheStoredHash(t *testing.T) {
	h := newHarness(t)

	var (
		stored  entity.SignUp
		payload entity.SignUpVerificationPayload
	)

	h.expectAddressAllowed()
	h.expectAddressFree()
	h.breaches.EXPECT().Compromised(gomock.Any(), signUpPassword).Return(false, nil)
	h.expectSignUpStored(&stored)
	h.producer.EXPECT().
		EnqueueSignUpVerification(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, sent entity.SignUpVerificationPayload) error {
			payload = sent

			return nil
		})

	if _, err := h.service.RequestSignUp(context.Background(), signUpInput()); err != nil {
		t.Fatalf("RequestSignUp: %v", err)
	}

	if payload.SignUpID != stored.ID {
		t.Fatalf("enqueued sign-up id = %v, want %v", payload.SignUpID, stored.ID)
	}

	if !bytes.Equal(entity.HashSignUpToken(payload.Token), stored.TokenHash) {
		t.Fatal("the enqueued token does not hash to the stored token hash")
	}
}

func TestSigningUpWithATakenAddressIsRefusedBeforeAnythingIsStored(t *testing.T) {
	h := newHarness(t)

	h.expectAddressAllowed()
	h.accounts.EXPECT().GetByEmail(gomock.Any(), signUpEmail).Return(activeAccount(uuid.New()), nil)

	if _, err := h.service.RequestSignUp(context.Background(), signUpInput()); !errors.Is(err, entity.ErrAccountEmailTaken) {
		t.Fatalf("RequestSignUp error = %v, want %v", err, entity.ErrAccountEmailTaken)
	}
}

func TestTheAddressSomebodyActuallyUsesCanSignUp(t *testing.T) {
	for _, email := range []string{
		"rae@gmail.com",
		"rae@outlook.com",
		"rae@yahoo.co.uk",
		"rae@proton.me",
		"Rae@ICloud.com",
	} {
		t.Run(email, func(t *testing.T) {
			h := newHarness(t)

			var stored entity.SignUp

			normalized := entity.NormalizeEmail(email)

			h.expectAddressAllowed()
			h.accounts.EXPECT().
				GetByEmail(gomock.Any(), normalized).
				Return(entity.Account{}, entity.ErrAccountNotFound)
			h.breaches.EXPECT().Compromised(gomock.Any(), signUpPassword).Return(false, nil)
			h.signUps.EXPECT().DeletePendingByEmail(gomock.Any(), normalized).Return(nil)
			h.signUps.EXPECT().
				Create(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, signUp entity.SignUp) (entity.SignUp, error) {
					signUp.ID = uuid.New()
					stored = signUp

					return signUp, nil
				})
			h.producer.EXPECT().EnqueueSignUpVerification(gomock.Any(), gomock.Any()).Return(nil)

			input := signUpInput()
			input.Email = email

			if _, err := h.service.RequestSignUp(context.Background(), input); err != nil {
				t.Fatalf(
					"signing up with %s failed: %v. Most people arriving from the landing page "+
						"type the address they read mail at; refusing it spends the click and "+
						"leaves no account behind.",
					email, err,
				)
			}

			if stored.Email != normalized {
				t.Fatalf("stored email = %q, want the normalized %q", stored.Email, normalized)
			}
		})
	}
}

func TestASignUpThatCannotBeMailedSaysSoRatherThanLookingSent(t *testing.T) {
	h := newHarness(t)

	var stored entity.SignUp

	h.expectAddressAllowed()
	h.expectAddressFree()
	h.breaches.EXPECT().Compromised(gomock.Any(), signUpPassword).Return(false, nil)
	h.expectSignUpStored(&stored)
	h.producer.EXPECT().
		EnqueueSignUpVerification(gomock.Any(), gomock.Any()).
		Return(errors.New("the queue is unreachable"))

	if _, err := h.service.RequestSignUp(context.Background(), signUpInput()); !errors.Is(err, entity.ErrSignUpUndeliverable) {
		t.Fatalf("RequestSignUp error = %v, want %v", err, entity.ErrSignUpUndeliverable)
	}
}

func TestARequestedSignUpCarriesWhenItWasAskedForAndWhenItLapses(t *testing.T) {
	h := newHarness(t)

	var stored entity.SignUp

	h.expectAddressAllowed()
	h.expectAddressFree()
	h.breaches.EXPECT().Compromised(gomock.Any(), signUpPassword).Return(false, nil)
	h.expectSignUpStored(&stored)
	h.producer.EXPECT().EnqueueSignUpVerification(gomock.Any(), gomock.Any()).Return(nil)

	requested, err := h.service.RequestSignUp(context.Background(), signUpInput())
	if err != nil {
		t.Fatalf("RequestSignUp: %v", err)
	}

	if !requested.RequestedAt.Equal(stored.RequestedAt) {
		t.Fatalf("requested at = %s, want the stored %s", requested.RequestedAt, stored.RequestedAt)
	}

	if !requested.ExpiresAt.Equal(stored.ExpiresAt) {
		t.Fatalf("expires at = %s, want the stored %s", requested.ExpiresAt, stored.ExpiresAt)
	}
}

func TestSigningUpIsRefusedWhenTheInstanceHasSignUpsClosed(t *testing.T) {
	h := newHarness(t)
	svc := newServiceWithInstance(h, config.Instance{SignupsOpen: false, PasswordAuth: true})

	if _, err := svc.RequestSignUp(context.Background(), signUpInput()); !errors.Is(err, entity.ErrSignUpsClosed) {
		t.Fatalf("RequestSignUp error = %v, want %v", err, entity.ErrSignUpsClosed)
	}
}

func TestASecondSignUpForTheSameAddressSupersedesTheFirst(t *testing.T) {
	h := newHarness(t)

	var stored []entity.SignUp

	h.expectAddressAllowed()
	h.expectAddressAllowed()
	h.accounts.EXPECT().
		GetByEmail(gomock.Any(), signUpEmail).
		Return(entity.Account{}, entity.ErrAccountNotFound).
		Times(2)
	h.breaches.EXPECT().Compromised(gomock.Any(), signUpPassword).Return(false, nil).Times(2)
	h.signUps.EXPECT().DeletePendingByEmail(gomock.Any(), signUpEmail).Return(nil).Times(2)
	h.signUps.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, signUp entity.SignUp) (entity.SignUp, error) {
			signUp.ID = uuid.New()
			stored = append(stored, signUp)

			return signUp, nil
		}).
		Times(2)
	h.producer.EXPECT().EnqueueSignUpVerification(gomock.Any(), gomock.Any()).Return(nil).Times(2)

	for range 2 {
		if _, err := h.service.RequestSignUp(context.Background(), signUpInput()); err != nil {
			t.Fatalf("RequestSignUp: %v", err)
		}
	}

	if bytes.Equal(stored[0].TokenHash, stored[1].TokenHash) {
		t.Fatal("the second sign-up reused the first token, so the first link still works")
	}
}

func TestConfirmingASignUpCreatesTheAccountAndIssuesASession(t *testing.T) {
	h := newHarness(t)
	pending := pendingSignUp()
	accountID := uuid.New()

	var created entity.Account

	h.signUps.EXPECT().
		GetByTokenHash(gomock.Any(), entity.HashSignUpToken("a-pending-token")).
		Return(pending, nil)
	h.signUps.EXPECT().MarkConfirmed(gomock.Any(), pending.ID, gomock.Any()).Return(nil)
	h.accounts.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, account entity.Account) (entity.Account, error) {
			account.ID = accountID
			created = account

			return account, nil
		})

	issued := h.expectSessionStarted(accountID)

	confirmed, err := h.service.ConfirmSignUp(context.Background(), service.ConfirmSignUpInput{Token: "a-pending-token"})
	if err != nil {
		t.Fatalf("ConfirmSignUp: %v", err)
	}

	if created.Status != entity.AccountStatusActive {
		t.Fatalf("created account status = %q, want %q", created.Status, entity.AccountStatusActive)
	}

	if created.Email != pending.Email || created.DisplayName != pending.DisplayName {
		t.Fatalf("created account = %q/%q, want the pending %q/%q",
			created.Email, created.DisplayName, pending.Email, pending.DisplayName)
	}

	if created.PasswordHash != pending.PasswordHash {
		t.Fatal("confirm re-hashed the password instead of carrying the stored hash across")
	}

	if confirmed.Session.Token != issued.Token {
		t.Fatalf("session token = %q, want the issued %q", confirmed.Session.Token, issued.Token)
	}
}

func TestAConfirmedSignUpProducesAnAccountThatCanSignIn(t *testing.T) {
	h := newHarness(t)

	var stored entity.SignUp

	h.expectAddressAllowed()
	h.expectAddressFree()
	h.breaches.EXPECT().Compromised(gomock.Any(), signUpPassword).Return(false, nil)
	h.expectSignUpStored(&stored)
	h.producer.EXPECT().EnqueueSignUpVerification(gomock.Any(), gomock.Any()).Return(nil)

	if _, err := h.service.RequestSignUp(context.Background(), signUpInput()); err != nil {
		t.Fatalf("RequestSignUp: %v", err)
	}

	accountID := uuid.New()

	var created entity.Account

	h.signUps.EXPECT().GetByTokenHash(gomock.Any(), gomock.Any()).Return(stored, nil)
	h.signUps.EXPECT().MarkConfirmed(gomock.Any(), stored.ID, gomock.Any()).Return(nil)
	h.accounts.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, account entity.Account) (entity.Account, error) {
			account.ID = accountID
			created = account

			return account, nil
		})
	h.expectSessionStarted(accountID)

	if _, err := h.service.ConfirmSignUp(context.Background(), service.ConfirmSignUpInput{Token: "any-token"}); err != nil {
		t.Fatalf("ConfirmSignUp: %v", err)
	}

	if !created.CanAuthenticate() {
		t.Fatal("the confirmed account cannot authenticate")
	}

	matches, err := entity.VerifyPassword(created.PasswordHash, signUpPassword)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}

	if !matches {
		t.Fatal("the confirmed account does not accept the password chosen at sign-up")
	}
}

func TestConfirmingAnExpiredSignUpIsRefused(t *testing.T) {
	h := newHarness(t)
	pending := pendingSignUp()
	pending.ExpiresAt = time.Now().UTC().Add(-time.Minute)

	h.signUps.EXPECT().GetByTokenHash(gomock.Any(), gomock.Any()).Return(pending, nil)

	_, err := h.service.ConfirmSignUp(context.Background(), service.ConfirmSignUpInput{Token: "a-pending-token"})
	if !errors.Is(err, entity.ErrSignUpExpired) {
		t.Fatalf("ConfirmSignUp error = %v, want %v", err, entity.ErrSignUpExpired)
	}
}

func TestConfirmingAConsumedSignUpIsRefused(t *testing.T) {
	h := newHarness(t)
	pending := pendingSignUp()
	confirmedAt := pending.RequestedAt.Add(time.Minute)
	pending.ConfirmedAt = &confirmedAt
	pending.ExpiresAt = time.Now().UTC().Add(-time.Minute)

	h.signUps.EXPECT().GetByTokenHash(gomock.Any(), gomock.Any()).Return(pending, nil)

	_, err := h.service.ConfirmSignUp(context.Background(), service.ConfirmSignUpInput{Token: "a-pending-token"})
	if !errors.Is(err, entity.ErrSignUpAlreadyConfirmed) {
		t.Fatalf("ConfirmSignUp error = %v, want %v", err, entity.ErrSignUpAlreadyConfirmed)
	}
}

func TestConfirmingASignUpWhoseAddressWasTakenMeanwhileIsRefused(t *testing.T) {
	h := newHarness(t)
	pending := pendingSignUp()

	h.signUps.EXPECT().GetByTokenHash(gomock.Any(), gomock.Any()).Return(pending, nil)
	h.signUps.EXPECT().MarkConfirmed(gomock.Any(), pending.ID, gomock.Any()).Return(nil)
	h.accounts.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(entity.Account{}, entity.ErrAccountEmailTaken)

	_, err := h.service.ConfirmSignUp(context.Background(), service.ConfirmSignUpInput{Token: "a-pending-token"})
	if !errors.Is(err, entity.ErrAccountEmailTaken) {
		t.Fatalf("ConfirmSignUp error = %v, want %v", err, entity.ErrAccountEmailTaken)
	}
}

func TestAnUnrecognisedSignUpTokenIsRefused(t *testing.T) {
	empty := newHarness(t)

	_, err := empty.service.ConfirmSignUp(context.Background(), service.ConfirmSignUpInput{})
	if !errors.Is(err, entity.ErrSignUpTokenInvalid) {
		t.Fatalf("ConfirmSignUp error = %v, want %v", err, entity.ErrSignUpTokenInvalid)
	}

	unknown := newHarness(t)
	unknown.signUps.EXPECT().
		GetByTokenHash(gomock.Any(), gomock.Any()).
		Return(entity.SignUp{}, entity.ErrSignUpNotFound)

	_, err = unknown.service.ConfirmSignUp(context.Background(), service.ConfirmSignUpInput{Token: "no-such-token"})
	if !errors.Is(err, entity.ErrSignUpNotFound) {
		t.Fatalf("ConfirmSignUp error = %v, want %v", err, entity.ErrSignUpNotFound)
	}
}

func TestTheVerificationMailAddressesThePendingSignUpAndCarriesTheOneTimeLink(t *testing.T) {
	h := newHarness(t)
	pending := pendingSignUp()

	var sent entity.Mail

	h.signUps.EXPECT().GetByID(gomock.Any(), pending.ID).Return(pending, nil)
	h.mailer.EXPECT().
		Send(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, mail entity.Mail) error {
			sent = mail

			return nil
		})

	if err := h.service.SendSignUpVerification(context.Background(), pending.ID, "a-pending-token"); err != nil {
		t.Fatalf("SendSignUpVerification: %v", err)
	}

	if sent.To != pending.Email {
		t.Fatalf("mail addressed to %q, want %q", sent.To, pending.Email)
	}

	for _, body := range []string{sent.PlainBody, sent.HTMLBody} {
		if !strings.Contains(body, "a-pending-token") {
			t.Fatalf("body does not carry the one-time link: %q", body)
		}
	}
}

func TestAStaleVerificationJobSendsNothing(t *testing.T) {
	confirmedAt := time.Now().UTC()

	cases := map[string]func(entity.SignUp) entity.SignUp{
		"already confirmed": func(s entity.SignUp) entity.SignUp {
			s.ConfirmedAt = &confirmedAt

			return s
		},
		"expired": func(s entity.SignUp) entity.SignUp {
			s.ExpiresAt = time.Now().UTC().Add(-time.Minute)

			return s
		},
	}

	for name, stale := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			pending := stale(pendingSignUp())

			h.signUps.EXPECT().GetByID(gomock.Any(), pending.ID).Return(pending, nil)

			if err := h.service.SendSignUpVerification(context.Background(), pending.ID, "a-pending-token"); err != nil {
				t.Fatalf("SendSignUpVerification: %v", err)
			}
		})
	}

	t.Run("no such sign-up", func(t *testing.T) {
		h := newHarness(t)
		signUpID := uuid.New()

		h.signUps.EXPECT().GetByID(gomock.Any(), signUpID).Return(entity.SignUp{}, entity.ErrSignUpNotFound)

		if err := h.service.SendSignUpVerification(context.Background(), signUpID, "a-pending-token"); err != nil {
			t.Fatalf("SendSignUpVerification: %v", err)
		}
	})
}
