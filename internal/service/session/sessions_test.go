package session_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	accountrepo "github.com/usenorn/norn/internal/repository/account"
	geolocationrepo "github.com/usenorn/norn/internal/repository/geolocation"
	jobqueuerepo "github.com/usenorn/norn/internal/repository/jobqueue"
	mailerrepo "github.com/usenorn/norn/internal/repository/mailer"
	membershiprepo "github.com/usenorn/norn/internal/repository/membership"
	sessionrepo "github.com/usenorn/norn/internal/repository/session"
	signinchallengerepo "github.com/usenorn/norn/internal/repository/signinchallenge"
	signinthrottlerepo "github.com/usenorn/norn/internal/repository/signinthrottle"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	sessionsvc "github.com/usenorn/norn/internal/service/session"
)

const (
	password        = "correct horse battery staple"
	idleTimeout     = 168 * time.Hour
	absoluteTime    = 720 * time.Hour
	refreshInterval = time.Minute
)

type recordedActivity struct {
	accountID uuid.UUID
	activeAt  time.Time
	method    entity.SessionAuthMethod
}

type harness struct {
	sessions    *sessionrepo.MockSession
	accounts    *accountrepo.MockAccount
	memberships *membershiprepo.MockMembership
	geoLocator  *geolocationrepo.MockGeoLocator
	throttle    *signinthrottlerepo.MockSignInThrottle
	challenges  *signinchallengerepo.MockSignInChallenge
	mailer      *mailerrepo.MockMailer
	producer    *jobqueuerepo.MockJobProducer
	authorizer  *authorizersvc.MockAuthorizer
	activity    *[]recordedActivity
	activityErr error
	service     service.Sessions
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		sessions:    sessionrepo.NewMockSession(ctrl),
		accounts:    accountrepo.NewMockAccount(ctrl),
		memberships: membershiprepo.NewMockMembership(ctrl),
		geoLocator:  geolocationrepo.NewMockGeoLocator(ctrl),
		throttle:    signinthrottlerepo.NewMockSignInThrottle(ctrl),
		challenges:  signinchallengerepo.NewMockSignInChallenge(ctrl),
		mailer:      mailerrepo.NewMockMailer(ctrl),
		producer:    jobqueuerepo.NewMockJobProducer(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
	}

	h.activity = &[]recordedActivity{}

	h.memberships.EXPECT().
		RecordActivity(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, accountID uuid.UUID, activeAt time.Time, method entity.SessionAuthMethod) error {
			*h.activity = append(*h.activity, recordedActivity{
				accountID: accountID,
				activeAt:  activeAt,
				method:    method,
			})

			return h.activityErr
		}).
		AnyTimes()

	h.throttle.EXPECT().RecordAddressAttempt(gomock.Any(), gomock.Any()).Return(1, nil).AnyTimes()
	h.throttle.EXPECT().Get(gomock.Any(), gomock.Any()).Return(entity.SignInThrottle{}, nil).AnyTimes()
	h.throttle.EXPECT().Clear(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.throttle.EXPECT().
		RecordFailure(gomock.Any(), gomock.Any()).
		Return(entity.SignInThrottle{Failures: 1}, nil).
		AnyTimes()

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, request entity.AccessRequest) (entity.Decision, error) {
			actor, ok := identity.Actor(ctx)
			if !ok {
				return entity.Decision{}, entity.ErrAccountForbidden
			}

			if request.Subject != uuid.Nil && actor.AccountID != request.Subject {
				return entity.Decision{}, entity.ErrAccountForbidden
			}

			return entity.Decision{Actor: actor}, nil
		}).
		AnyTimes()

	h.service = sessionsvc.New(
		h.sessions,
		h.accounts,
		h.memberships,
		h.geoLocator,
		h.throttle,
		h.challenges,
		h.mailer,
		h.producer,
		config.Session{
			IdleTimeout:      idleTimeout,
			AbsoluteLifetime: absoluteTime,
			RefreshInterval:  refreshInterval,
		},
		config.App{BaseURL: "https://norn.test"},
		h.authorizer,
		silentAudit(ctrl),
	)

	return h
}

func accountWithPassword(t *testing.T, id uuid.UUID) entity.Account {
	t.Helper()

	hash, err := entity.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	return entity.Account{
		ID:           id,
		Status:       entity.AccountStatusActive,
		Email:        "ada@example.com",
		DisplayName:  "Ada Lovelace",
		PasswordHash: hash,
	}
}

func liveSession(accountID uuid.UUID, lastUsed time.Time) entity.Session {
	return entity.Session{
		ID:                uuid.New(),
		AccountID:         accountID,
		AuthMethod:        entity.SessionAuthMethodPassword,
		IssuedAt:          lastUsed,
		LastUsedAt:        lastUsed,
		IdleExpiresAt:     lastUsed.Add(idleTimeout),
		AbsoluteExpiresAt: lastUsed.Add(absoluteTime),
	}
}

func TestSignInIsRefusedForAnAccountWithoutAPassword(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()

	h.accounts.EXPECT().GetByEmail(gomock.Any(), "ada@example.com").Return(entity.Account{
		ID:     accountID,
		Status: entity.AccountStatusActive,
	}, nil)

	_, err := h.service.SignIn(context.Background(), service.SignInInput{Email: "ada@example.com", Password: password})
	if !errors.Is(err, entity.ErrAccountInvalidCredentials) {
		t.Fatalf("SignIn error = %v, want ErrAccountInvalidCredentials", err)
	}
}

func TestSignInIsRefusedForADeactivatedAccount(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()

	account := accountWithPassword(t, accountID)
	account.Status = entity.AccountStatusDeactivated

	h.accounts.EXPECT().GetByEmail(gomock.Any(), "ada@example.com").Return(account, nil)

	_, err := h.service.SignIn(context.Background(), service.SignInInput{Email: "ada@example.com", Password: password})
	if !errors.Is(err, entity.ErrAccountInvalidCredentials) {
		t.Fatalf("SignIn error = %v, want ErrAccountInvalidCredentials", err)
	}
}

func TestValidateLooksTheTokenUpByItsHashAndNeverReadsTheAccount(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()
	session := liveSession(accountID, time.Now().UTC())

	const token = "a-session-token"

	h.sessions.EXPECT().Get(gomock.Any(), entity.HashSessionToken(token)).Return(session, nil)
	h.sessions.EXPECT().RevokedAt(gomock.Any(), accountID).Return(time.Time{}, nil)

	validated, err := h.service.Validate(context.Background(), token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if validated.ID != session.ID {
		t.Fatalf("validated session = %v, want %v", validated.ID, session.ID)
	}
}

func TestValidateRefusesASessionExpiredByInactivity(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()
	session := liveSession(accountID, time.Now().UTC().Add(-2*idleTimeout))

	h.sessions.EXPECT().Get(gomock.Any(), gomock.Any()).Return(session, nil)

	_, err := h.service.Validate(context.Background(), "a-session-token")
	if !errors.Is(err, entity.ErrSessionNotFound) {
		t.Fatalf("Validate error = %v, want ErrSessionNotFound", err)
	}
}

func TestValidateRefusesASessionPastItsAbsoluteLifetimeEvenWhenActive(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()
	now := time.Now().UTC()

	session := entity.Session{
		ID:                uuid.New(),
		AccountID:         accountID,
		IssuedAt:          now.Add(-absoluteTime),
		LastUsedAt:        now,
		IdleExpiresAt:     now.Add(idleTimeout),
		AbsoluteExpiresAt: now.Add(-time.Second),
	}

	h.sessions.EXPECT().Get(gomock.Any(), gomock.Any()).Return(session, nil)

	_, err := h.service.Validate(context.Background(), "a-session-token")
	if !errors.Is(err, entity.ErrSessionNotFound) {
		t.Fatalf("Validate error = %v, want ErrSessionNotFound", err)
	}
}

func TestValidateRefusesASessionIssuedBeforeTheAccountWasRevoked(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()
	now := time.Now().UTC()
	session := liveSession(accountID, now.Add(-time.Hour))

	h.sessions.EXPECT().Get(gomock.Any(), gomock.Any()).Return(session, nil)
	h.sessions.EXPECT().RevokedAt(gomock.Any(), accountID).Return(now.Add(-time.Minute), nil)

	_, err := h.service.Validate(context.Background(), "a-session-token")
	if !errors.Is(err, entity.ErrSessionRevoked) {
		t.Fatalf("Validate error = %v, want ErrSessionRevoked", err)
	}
}

func TestValidateAcceptsASessionIssuedAfterTheAccountWasRevoked(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()
	now := time.Now().UTC()
	session := liveSession(accountID, now)

	h.sessions.EXPECT().Get(gomock.Any(), gomock.Any()).Return(session, nil)
	h.sessions.EXPECT().RevokedAt(gomock.Any(), accountID).Return(now.Add(-time.Hour), nil)

	if _, err := h.service.Validate(context.Background(), "a-session-token"); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidateDoesNotWriteWithinTheRefreshInterval(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()
	session := liveSession(accountID, time.Now().UTC())

	h.sessions.EXPECT().Get(gomock.Any(), gomock.Any()).Return(session, nil)
	h.sessions.EXPECT().RevokedAt(gomock.Any(), accountID).Return(time.Time{}, nil)

	if _, err := h.service.Validate(context.Background(), "a-session-token"); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidateExtendsTheIdleDeadlineOncePastTheRefreshInterval(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()
	session := liveSession(accountID, time.Now().UTC().Add(-10*time.Minute))

	h.sessions.EXPECT().Get(gomock.Any(), gomock.Any()).Return(session, nil)
	h.sessions.EXPECT().RevokedAt(gomock.Any(), accountID).Return(time.Time{}, nil)

	var captured entity.Session

	h.sessions.EXPECT().
		Touch(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, refreshed entity.Session) error {
			captured = refreshed

			return nil
		})

	validated, err := h.service.Validate(context.Background(), "a-session-token")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if !captured.IdleExpiresAt.After(session.IdleExpiresAt) {
		t.Fatal("refreshing did not extend the idle deadline")
	}

	if !captured.LastUsedAt.After(session.LastUsedAt) {
		t.Fatal("refreshing did not stamp last use")
	}

	if !validated.IdleExpiresAt.Equal(captured.IdleExpiresAt) {
		t.Fatal("Validate returned the stale session rather than the refreshed one")
	}
}

func TestValidateSurfacesALostRaceWithRevocationRatherThanRecreatingTheSession(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()
	session := liveSession(accountID, time.Now().UTC().Add(-10*time.Minute))

	h.sessions.EXPECT().Get(gomock.Any(), gomock.Any()).Return(session, nil)
	h.sessions.EXPECT().RevokedAt(gomock.Any(), accountID).Return(time.Time{}, nil)
	h.sessions.EXPECT().Touch(gomock.Any(), gomock.Any()).Return(entity.ErrSessionNotFound)

	_, err := h.service.Validate(context.Background(), "a-session-token")
	if !errors.Is(err, entity.ErrSessionNotFound) {
		t.Fatalf("Validate error = %v, want ErrSessionNotFound", err)
	}
}

func TestListAndRevokeRefuseAnotherAccountsSessions(t *testing.T) {
	accountID := uuid.New()
	intruder := uuid.New()

	operations := map[string]func(h *harness, ctx context.Context) error{
		"List": func(h *harness, ctx context.Context) error {
			_, err := h.service.List(ctx, accountID)

			return err
		},
		"Revoke": func(h *harness, ctx context.Context) error {
			return h.service.Revoke(ctx, accountID, uuid.New())
		},
		"RevokeAllByAccountID": func(h *harness, ctx context.Context) error {
			return h.service.RevokeAllByAccountID(ctx, accountID)
		},
	}

	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)

			if err := operation(h, identity.Into(context.Background(), intruder)); !errors.Is(err, entity.ErrAccountForbidden) {
				t.Fatalf("%s error = %v, want ErrAccountForbidden", name, err)
			}

			if err := operation(h, context.Background()); !errors.Is(err, entity.ErrAccountForbidden) {
				t.Fatalf("%s without an identity error = %v, want ErrAccountForbidden", name, err)
			}
		})
	}
}

func TestRevokingEveryStoredSessionAlsoRecordsTheRevocationInstant(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()

	marked := h.sessions.EXPECT().MarkRevoked(gomock.Any(), accountID, gomock.Any()).Return(nil)
	h.sessions.EXPECT().DeleteByAccountID(gomock.Any(), accountID).Return(nil).After(marked)

	if err := h.service.RevokeAllByAccountID(identity.Into(context.Background(), accountID), accountID); err != nil {
		t.Fatalf("RevokeAllByAccountID: %v", err)
	}
}

func TestRotatingAfterACredentialChangeRevokesEverythingThenIssuesAFreshSession(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()
	current := liveSession(accountID, time.Now().UTC())
	current.Client = entity.SessionClient{UserAgent: "Mozilla/5.0", IP: netip.MustParseAddr("203.0.113.7")}

	marked := h.sessions.EXPECT().MarkRevoked(gomock.Any(), accountID, gomock.Any()).Return(nil)
	deleted := h.sessions.EXPECT().DeleteByAccountID(gomock.Any(), accountID).Return(nil).After(marked)
	h.geoLocator.EXPECT().Locate(gomock.Any(), current.Client.IP).Return(entity.Location{CountryCode: "GB"}, nil)

	var captured entity.Session

	h.sessions.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, session entity.Session) error {
			captured = session

			return nil
		}).
		After(deleted)

	ctx := identity.WithSession(context.Background(), current)

	issued, err := h.service.RotateAfterCredentialChange(ctx, accountID)
	if err != nil {
		t.Fatalf("RotateAfterCredentialChange: %v", err)
	}

	if issued.Token == "" {
		t.Fatal("rotation did not issue a replacement token")
	}

	if captured.ID == current.ID || captured.TokenHash == current.TokenHash {
		t.Fatal("rotation reused the previous session identity")
	}

	if captured.Client.UserAgent != current.Client.UserAgent {
		t.Fatalf("rotated client = %+v, want the current session's client", captured.Client)
	}
}

func TestRefreshingASessionRecordsMembershipActivity(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()
	session := liveSession(accountID, time.Now().UTC().Add(-10*time.Minute))

	h.sessions.EXPECT().Get(gomock.Any(), gomock.Any()).Return(session, nil)
	h.sessions.EXPECT().RevokedAt(gomock.Any(), accountID).Return(time.Time{}, nil)
	h.sessions.EXPECT().Touch(gomock.Any(), gomock.Any()).Return(nil)

	refreshed, err := h.service.Validate(context.Background(), "a-session-token")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if len(*h.activity) != 1 {
		t.Fatalf("recorded %d activity writes, want exactly one", len(*h.activity))
	}

	recorded := (*h.activity)[0]

	if recorded.accountID != accountID {
		t.Fatalf("recorded account = %v, want %v", recorded.accountID, accountID)
	}

	if !recorded.activeAt.Equal(refreshed.LastUsedAt) {
		t.Fatalf("recorded activity at %v, want the refreshed last-used time %v", recorded.activeAt, refreshed.LastUsedAt)
	}

	if recorded.method != session.AuthMethod {
		t.Fatalf("recorded auth method = %q, want %q", recorded.method, session.AuthMethod)
	}
}

func TestASessionBelowTheRefreshIntervalRecordsNoActivity(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()
	session := liveSession(accountID, time.Now().UTC())

	h.sessions.EXPECT().Get(gomock.Any(), gomock.Any()).Return(session, nil)
	h.sessions.EXPECT().RevokedAt(gomock.Any(), accountID).Return(time.Time{}, nil)

	if _, err := h.service.Validate(context.Background(), "a-session-token"); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if len(*h.activity) != 0 {
		t.Fatalf("recorded %d activity writes, want none; the refresh throttle is what bounds the write rate", len(*h.activity))
	}
}

func TestAFailedActivityWriteStillAuthenticates(t *testing.T) {
	h := newHarness(t)
	h.activityErr = errors.New("record membership activity: connection refused")

	accountID := uuid.New()
	session := liveSession(accountID, time.Now().UTC().Add(-10*time.Minute))

	h.sessions.EXPECT().Get(gomock.Any(), gomock.Any()).Return(session, nil)
	h.sessions.EXPECT().RevokedAt(gomock.Any(), accountID).Return(time.Time{}, nil)
	h.sessions.EXPECT().Touch(gomock.Any(), gomock.Any()).Return(nil)

	refreshed, err := h.service.Validate(context.Background(), "a-session-token")
	if err != nil {
		t.Fatalf("a bookkeeping failure must not break authentication, got %v", err)
	}

	if refreshed.AccountID != accountID {
		t.Fatalf("Validate returned account %v, want %v", refreshed.AccountID, accountID)
	}
}

func TestFinishingASignInRecordsMembershipActivityImmediately(t *testing.T) {
	h := newHarness(t)
	accountID := uuid.New()

	h.captureChallenge()
	code := h.captureSentCode()

	h.accounts.EXPECT().GetByEmail(gomock.Any(), "ada@example.com").Return(accountWithPassword(t, accountID), nil)
	h.geoLocator.EXPECT().Locate(gomock.Any(), gomock.Any()).Return(entity.Location{}, nil)
	h.sessions.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	challenge, err := signIn(h, password)
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}

	if _, err := h.service.VerifySignInCode(context.Background(), service.VerifySignInCodeInput{
		ChallengeID: challenge.ID,
		Code:        *code,
	}); err != nil {
		t.Fatalf("VerifySignInCode: %v", err)
	}

	if len(*h.activity) != 1 {
		t.Fatalf("recorded %d activity writes, want one; a member who just signed in must not read as never active", len(*h.activity))
	}

	if (*h.activity)[0].method != entity.SessionAuthMethodPassword {
		t.Fatalf("recorded auth method = %q, want password", (*h.activity)[0].method)
	}
}

func TestStartingASessionRecordsHowThePersonActuallyAuthenticated(t *testing.T) {
	for _, method := range []entity.SessionAuthMethod{
		entity.SessionAuthMethodPassword,
		entity.SessionAuthMethodSSO,
	} {
		h := newHarness(t)
		accountID := uuid.New()

		h.geoLocator.EXPECT().Locate(gomock.Any(), gomock.Any()).Return(entity.Location{}, nil)

		var stored entity.Session

		h.sessions.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, session entity.Session) error {
				stored = session

				return nil
			})

		issued, err := h.service.Start(context.Background(), service.StartSessionInput{
			AccountID:  accountID,
			AuthMethod: method,
		})
		if err != nil {
			t.Fatalf("Start with %q: %v", method, err)
		}

		if stored.AuthMethod != method {
			t.Errorf(
				"a session started with %q was stored as %q. A workspace that requires single "+
					"sign-on decides on this field alone.",
				method, stored.AuthMethod,
			)
		}

		if issued.Session.AuthMethod != method {
			t.Errorf("the returned session says %q, want %q", issued.Session.AuthMethod, method)
		}
	}
}

func TestASessionCannotBeStartedWithoutSayingHowItWasAuthenticated(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.Start(context.Background(), service.StartSessionInput{AccountID: uuid.New()})
	if !errors.Is(err, entity.ErrSessionAuthMethodUnknown) {
		t.Fatalf(
			"a session was started with no auth method (%v). Defaulting silently would let a "+
				"new caller mint a password session for an exchange that was nothing of the kind.",
			err,
		)
	}
}
