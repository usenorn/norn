package session_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	accountrepo "github.com/usenorn/norn/internal/repository/account"
	geolocationrepo "github.com/usenorn/norn/internal/repository/geolocation"
	membershiprepo "github.com/usenorn/norn/internal/repository/membership"
	sessionrepo "github.com/usenorn/norn/internal/repository/session"
	signinthrottlerepo "github.com/usenorn/norn/internal/repository/signinthrottle"
	"github.com/usenorn/norn/internal/service"
	sessionsvc "github.com/usenorn/norn/internal/service/session"
)

func newBareHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		sessions:    sessionrepo.NewMockSession(ctrl),
		accounts:    accountrepo.NewMockAccount(ctrl),
		memberships: membershiprepo.NewMockMembership(ctrl),
		geoLocator:  geolocationrepo.NewMockGeoLocator(ctrl),
		throttle:    signinthrottlerepo.NewMockSignInThrottle(ctrl),
	}

	h.memberships.EXPECT().
		RecordActivity(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()

	h.service = sessionsvc.New(h.sessions, h.accounts, h.memberships, h.geoLocator, h.throttle, config.Session{
		IdleTimeout:      idleTimeout,
		AbsoluteLifetime: absoluteTime,
		RefreshInterval:  refreshInterval,
	}, h.authorizer)

	return h
}

func nowPlus(d time.Duration) time.Time {
	return time.Now().UTC().Add(d)
}

func signIn(h *harness, password string) (service.IssuedSession, error) {
	return h.service.SignIn(context.Background(), service.SignInInput{
		Email:    "ada@example.com",
		Password: password,
	})
}

func TestTheTenthConsecutiveFailureLocksTheAccount(t *testing.T) {
	h := newBareHarness(t)
	accountID := uuid.New()

	unlocksAt := entity.SignInLockDuration

	h.throttle.EXPECT().RecordAddressAttempt(gomock.Any(), gomock.Any()).Return(1, nil)
	h.throttle.EXPECT().Get(gomock.Any(), gomock.Any()).Return(entity.SignInThrottle{}, nil)
	h.accounts.EXPECT().GetByEmail(gomock.Any(), "ada@example.com").Return(accountWithPassword(t, accountID), nil)
	h.throttle.EXPECT().
		RecordFailure(gomock.Any(), gomock.Any()).
		Return(entity.SignInThrottle{
			Failures:    entity.SignInMaxFailures,
			LockedUntil: nowPlus(unlocksAt),
		}, nil)

	_, err := signIn(h, "the wrong password")

	var locked entity.AccountLockedError
	if !errors.As(err, &locked) {
		t.Fatalf("SignIn error = %v, want an AccountLockedError", err)
	}

	if locked.UnlocksAt.IsZero() {
		t.Fatal("the lock error carried no unlock time")
	}

	if !errors.Is(err, entity.ErrAccountLocked) {
		t.Fatal("the lock error does not unwrap to ErrAccountLocked")
	}
}

func TestAFailedSignInReportsHowManyAttemptsRemain(t *testing.T) {
	h := newBareHarness(t)
	accountID := uuid.New()

	h.throttle.EXPECT().RecordAddressAttempt(gomock.Any(), gomock.Any()).Return(1, nil)
	h.throttle.EXPECT().Get(gomock.Any(), gomock.Any()).Return(entity.SignInThrottle{}, nil)
	h.accounts.EXPECT().GetByEmail(gomock.Any(), "ada@example.com").Return(accountWithPassword(t, accountID), nil)
	h.throttle.EXPECT().
		RecordFailure(gomock.Any(), gomock.Any()).
		Return(entity.SignInThrottle{Failures: 3}, nil)

	_, err := signIn(h, "the wrong password")

	var invalid entity.InvalidCredentialsError
	if !errors.As(err, &invalid) {
		t.Fatalf("SignIn error = %v, want an InvalidCredentialsError", err)
	}

	if invalid.AttemptsLeft != entity.SignInMaxFailures-3 {
		t.Fatalf("AttemptsLeft = %d, want %d", invalid.AttemptsLeft, entity.SignInMaxFailures-3)
	}

	if !errors.Is(err, entity.ErrAccountInvalidCredentials) {
		t.Fatal("the failure does not unwrap to ErrAccountInvalidCredentials")
	}
}

func TestAnUnknownAddressFailsWithTheSameShapeAsAKnownOne(t *testing.T) {
	unknown := newBareHarness(t)

	unknown.throttle.EXPECT().RecordAddressAttempt(gomock.Any(), gomock.Any()).Return(1, nil)
	unknown.throttle.EXPECT().Get(gomock.Any(), gomock.Any()).Return(entity.SignInThrottle{}, nil)
	unknown.accounts.EXPECT().
		GetByEmail(gomock.Any(), "ada@example.com").
		Return(entity.Account{}, entity.ErrAccountNotFound)
	unknown.throttle.EXPECT().
		RecordFailure(gomock.Any(), gomock.Any()).
		Return(entity.SignInThrottle{Failures: 1}, nil)

	_, unknownErr := signIn(unknown, password)

	known := newBareHarness(t)
	accountID := uuid.New()

	known.throttle.EXPECT().RecordAddressAttempt(gomock.Any(), gomock.Any()).Return(1, nil)
	known.throttle.EXPECT().Get(gomock.Any(), gomock.Any()).Return(entity.SignInThrottle{}, nil)
	known.accounts.EXPECT().
		GetByEmail(gomock.Any(), "ada@example.com").
		Return(accountWithPassword(t, accountID), nil)
	known.throttle.EXPECT().
		RecordFailure(gomock.Any(), gomock.Any()).
		Return(entity.SignInThrottle{Failures: 1}, nil)

	_, knownErr := signIn(known, "the wrong password")

	if unknownErr.Error() != knownErr.Error() {
		t.Fatalf("errors differ: unknown = %q, known = %q", unknownErr, knownErr)
	}
}

func TestALockedSubjectIsRefusedWithoutTouchingTheAccountRepository(t *testing.T) {
	h := newBareHarness(t)

	h.throttle.EXPECT().RecordAddressAttempt(gomock.Any(), gomock.Any()).Return(1, nil)
	h.throttle.EXPECT().
		Get(gomock.Any(), gomock.Any()).
		Return(entity.SignInThrottle{
			Failures:    entity.SignInMaxFailures,
			LockedUntil: nowPlus(entity.SignInLockDuration),
		}, nil)

	_, err := signIn(h, password)
	if !errors.Is(err, entity.ErrAccountLocked) {
		t.Fatalf("SignIn error = %v, want ErrAccountLocked", err)
	}
}

func TestTooManyAttemptsFromOneAddressAreRefusedBeforeTheSubjectLookup(t *testing.T) {
	h := newBareHarness(t)

	h.throttle.EXPECT().
		RecordAddressAttempt(gomock.Any(), gomock.Any()).
		Return(entity.SignInAddressMaxAttempts+1, nil)

	_, err := signIn(h, password)
	if !errors.Is(err, entity.ErrSignInRateLimited) {
		t.Fatalf("SignIn error = %v, want ErrSignInRateLimited", err)
	}
}

func TestASuccessfulSignInClearsTheFailureCounter(t *testing.T) {
	h := newBareHarness(t)
	accountID := uuid.New()

	h.throttle.EXPECT().RecordAddressAttempt(gomock.Any(), gomock.Any()).Return(1, nil)
	h.throttle.EXPECT().Get(gomock.Any(), gomock.Any()).Return(entity.SignInThrottle{}, nil)
	h.accounts.EXPECT().GetByEmail(gomock.Any(), "ada@example.com").Return(accountWithPassword(t, accountID), nil)
	h.throttle.EXPECT().Clear(gomock.Any(), entity.HashSignInSubject("ada@example.com")).Return(nil)
	h.geoLocator.EXPECT().Locate(gomock.Any(), gomock.Any()).Return(entity.Location{}, nil)
	h.sessions.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	if _, err := signIn(h, password); err != nil {
		t.Fatalf("SignIn: %v", err)
	}
}

func TestTheFailureCounterIsKeyedOnTheNormalizedAddress(t *testing.T) {
	h := newBareHarness(t)

	var subject string

	h.throttle.EXPECT().RecordAddressAttempt(gomock.Any(), gomock.Any()).Return(1, nil)
	h.throttle.EXPECT().
		Get(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, hash string) (entity.SignInThrottle, error) {
			subject = hash

			return entity.SignInThrottle{}, nil
		})
	h.accounts.EXPECT().
		GetByEmail(gomock.Any(), "ada@example.com").
		Return(entity.Account{}, entity.ErrAccountNotFound)
	h.throttle.EXPECT().
		RecordFailure(gomock.Any(), gomock.Any()).
		Return(entity.SignInThrottle{Failures: 1}, nil)

	if _, err := h.service.SignIn(context.Background(), service.SignInInput{
		Email:    "  ADA@Example.COM  ",
		Password: password,
	}); err == nil {
		t.Fatal("SignIn accepted an unknown address")
	}

	if subject != entity.HashSignInSubject("ada@example.com") {
		t.Fatal("the throttle subject is not the hash of the normalized address")
	}
}
