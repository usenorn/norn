package session_test

import (
	"bytes"
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (h *harness) captureChallenge() *entity.SignInChallenge {
	held := &entity.SignInChallenge{}

	h.challenges.EXPECT().
		Put(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, challenge entity.SignInChallenge) error {
			*held = challenge

			return nil
		}).
		AnyTimes()

	h.challenges.EXPECT().
		Get(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string) (entity.SignInChallenge, error) {
			return *held, nil
		}).
		AnyTimes()

	h.challenges.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	return held
}

func (h *harness) captureSentCode() *string {
	code := new(string)

	h.producer.EXPECT().
		EnqueueSignInCode(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload entity.SignInCodePayload) error {
			*code = payload.Code

			return nil
		}).
		AnyTimes()

	return code
}

func TestAPasswordAloneIssuesNoSessionAndSendsACode(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()
	held := h.captureChallenge()
	code := h.captureSentCode()

	h.accounts.EXPECT().GetByEmail(gomock.Any(), "ada@example.com").Return(accountWithPassword(t, accountID), nil)

	issued, err := h.service.SignIn(context.Background(), service.SignInInput{
		Email:    "Ada@Example.com",
		Password: password,
		Client:   entity.SessionClient{UserAgent: "Mozilla/5.0"},
	})
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}

	if issued.ID == "" || issued.ExpiresAt.IsZero() {
		t.Fatal("the caller was given nothing to answer the challenge with")
	}

	if !bytes.Equal(held.CodeHash, entity.HashSignInCode(*code)) {
		t.Fatal(
			"the code that was sent is not the code the challenge will accept. Every sign-in " +
				"would be refused with the right code in hand.",
		)
	}

	if *code == "" {
		t.Fatal("no code was queued for delivery, so nobody could ever finish signing in")
	}
}

func TestTheCodeFinishesTheSignInAndKeepsTheClientThatStartedIt(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()
	ip := netip.MustParseAddr("203.0.113.7")
	held := h.captureChallenge()
	code := h.captureSentCode()

	h.accounts.EXPECT().GetByEmail(gomock.Any(), "ada@example.com").Return(accountWithPassword(t, accountID), nil)
	h.geoLocator.EXPECT().
		Locate(gomock.Any(), ip).
		Return(entity.Location{CountryCode: "GB", City: "London"}, nil)

	var captured entity.Session

	h.sessions.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, session entity.Session) error {
			captured = session

			return nil
		})

	challenge, err := h.service.SignIn(context.Background(), service.SignInInput{
		Email:    "ada@example.com",
		Password: password,
		Client:   entity.SessionClient{UserAgent: "Mozilla/5.0", IP: ip},
	})
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}

	issued, err := h.service.VerifySignInCode(context.Background(), service.VerifySignInCodeInput{
		ChallengeID: challenge.ID,
		Code:        *code,
	})
	if err != nil {
		t.Fatalf("VerifySignInCode: %v", err)
	}

	if captured.AccountID != accountID {
		t.Fatalf("session account = %v, want %v", captured.AccountID, accountID)
	}

	if captured.AuthMethod != entity.SessionAuthMethodPassword {
		t.Fatalf("auth method = %q, want password", captured.AuthMethod)
	}

	if captured.Client.UserAgent != "Mozilla/5.0" || captured.Client.IP != ip {
		t.Fatalf(
			"session client = %+v, want the one that typed the password. The code is answered "+
				"on a second request and the session must describe the browser, not that request.",
			captured.Client,
		)
	}

	if captured.Client.Location.CountryCode != "GB" {
		t.Fatalf("location = %+v, want the resolved one", captured.Client.Location)
	}

	if captured.TokenHash != entity.HashSessionToken(issued.Token) {
		t.Fatal("the stored hash does not match the issued token")
	}

	_ = held
}

func TestAWrongCodeIsCountedAndSaysWhatIsLeft(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()
	held := h.captureChallenge()
	h.captureSentCode()

	h.accounts.EXPECT().GetByEmail(gomock.Any(), "ada@example.com").Return(accountWithPassword(t, accountID), nil)

	challenge, err := h.service.SignIn(context.Background(), service.SignInInput{
		Email:    "ada@example.com",
		Password: password,
	})
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}

	_, err = h.service.VerifySignInCode(context.Background(), service.VerifySignInCodeInput{
		ChallengeID: challenge.ID,
		Code:        "000000",
	})

	var wrong entity.SignInCodeIncorrectError
	if !errors.As(err, &wrong) {
		t.Fatalf("VerifySignInCode error = %v, want an incorrect-code error", err)
	}

	if wrong.AttemptsLeft != entity.SignInChallengeMaxAttempts-1 {
		t.Errorf("attempts left = %d, want %d", wrong.AttemptsLeft, entity.SignInChallengeMaxAttempts-1)
	}

	if held.Attempts != 1 {
		t.Fatalf(
			"the challenge recorded %d attempts after one wrong code. A guess that is not "+
				"counted is a guess that can be repeated.",
			held.Attempts,
		)
	}
}

func TestACodeGuessedAtTooOftenEndsTheSignIn(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()
	h.captureChallenge()
	code := h.captureSentCode()

	h.accounts.EXPECT().GetByEmail(gomock.Any(), "ada@example.com").Return(accountWithPassword(t, accountID), nil)

	challenge, err := h.service.SignIn(context.Background(), service.SignInInput{
		Email:    "ada@example.com",
		Password: password,
	})
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}

	for range entity.SignInChallengeMaxAttempts {
		_, err = h.service.VerifySignInCode(context.Background(), service.VerifySignInCodeInput{
			ChallengeID: challenge.ID,
			Code:        "000000",
		})
	}

	if !errors.Is(err, entity.ErrSignInCodeExhausted) {
		t.Fatalf("the %dth wrong code returned %v, want the challenge to be over", entity.SignInChallengeMaxAttempts, err)
	}

	if _, err := h.service.VerifySignInCode(context.Background(), service.VerifySignInCodeInput{
		ChallengeID: challenge.ID,
		Code:        *code,
	}); err == nil {
		t.Fatal("the right code was accepted after the challenge had been spent")
	}
}

func TestALapsedChallengeIsRefusedEvenWithTheRightCode(t *testing.T) {
	h := newHarness(t)

	code, codeHash, err := entity.NewSignInCode()
	if err != nil {
		t.Fatalf("NewSignInCode: %v", err)
	}

	lapsed := time.Now().UTC().Add(-time.Minute)

	h.challenges.EXPECT().
		Get(gomock.Any(), "spent").
		Return(entity.SignInChallenge{CodeHash: codeHash, ExpiresAt: lapsed}, nil)
	h.challenges.EXPECT().Delete(gomock.Any(), "spent").Return(nil)

	if _, err := h.service.VerifySignInCode(context.Background(), service.VerifySignInCodeInput{
		ChallengeID: "spent",
		Code:        code,
	}); !errors.Is(err, entity.ErrSignInChallengeNotFound) {
		t.Fatalf("VerifySignInCode error = %v, want the challenge to be gone", err)
	}
}
