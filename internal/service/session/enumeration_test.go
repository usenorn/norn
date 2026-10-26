package session_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	accountrepo "github.com/usenorn/norn/internal/repository/account"
	geolocationrepo "github.com/usenorn/norn/internal/repository/geolocation"
	jobqueuerepo "github.com/usenorn/norn/internal/repository/jobqueue"
	mailerrepo "github.com/usenorn/norn/internal/repository/mailer"
	membershiprepo "github.com/usenorn/norn/internal/repository/membership"
	sessionrepo "github.com/usenorn/norn/internal/repository/session"
	signinchallengerepo "github.com/usenorn/norn/internal/repository/signinchallenge"
	signinthrottlerepo "github.com/usenorn/norn/internal/repository/signinthrottle"
	"github.com/usenorn/norn/internal/service"
	auditsvc "github.com/usenorn/norn/internal/service/audit"
	sessionsvc "github.com/usenorn/norn/internal/service/session"
)

func traceRefusedSignIn(t *testing.T, account *entity.Account) ([]string, error) {
	t.Helper()

	ctrl := gomock.NewController(t)
	accounts := accountrepo.NewMockAccount(ctrl)
	throttle := signinthrottlerepo.NewMockSignInThrottle(ctrl)
	audit := auditsvc.NewMockAudit(ctrl)

	var trace []string

	throttle.EXPECT().RecordAddressAttempt(gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, any) (int, error) {
			trace = append(trace, "count address")

			return 1, nil
		})
	throttle.EXPECT().Get(gomock.Any(), entity.HashSignInSubject("ada@example.com")).
		DoAndReturn(func(context.Context, string) (entity.SignInThrottle, error) {
			trace = append(trace, "read throttle")

			return entity.SignInThrottle{Failures: 1}, nil
		})
	accounts.EXPECT().GetByEmail(gomock.Any(), "ada@example.com").
		DoAndReturn(func(context.Context, string) (entity.Account, error) {
			trace = append(trace, "read account")

			if account == nil {
				return entity.Account{}, entity.ErrAccountNotFound
			}

			return *account, nil
		})
	audit.EXPECT().Record(gomock.Any(), gomock.Any()).
		Do(func(_ context.Context, entry entity.AuditEntry) {
			trace = append(trace, "audit "+string(entry.Action)+" "+entry.Detail["email"])
		})
	throttle.EXPECT().RecordFailure(gomock.Any(), entity.HashSignInSubject("ada@example.com")).
		DoAndReturn(func(context.Context, string) (entity.SignInThrottle, error) {
			trace = append(trace, "count failure")

			return entity.SignInThrottle{Failures: 2}, nil
		})

	sessions := sessionsvc.New(
		sessionrepo.NewMockSession(ctrl),
		accounts,
		membershiprepo.NewMockMembership(ctrl),
		geolocationrepo.NewMockGeoLocator(ctrl),
		throttle,
		signinchallengerepo.NewMockSignInChallenge(ctrl),
		mailerrepo.NewMockMailer(ctrl),
		jobqueuerepo.NewMockJobProducer(ctrl),
		config.Session{IdleTimeout: idleTimeout, AbsoluteLifetime: absoluteTime, RefreshInterval: refreshInterval},
		config.App{BaseURL: "https://norn.test"},
		nil,
		audit,
	)

	_, err := sessions.SignIn(context.Background(), service.SignInInput{
		Email:    " Ada@Example.com ",
		Password: "the wrong password",
	})

	return trace, err
}

func TestARefusedSignInLooksTheSameWhetherOrNotTheAccountExists(t *testing.T) {
	known := accountWithPassword(t, uuid.New())

	withoutPassword := known
	withoutPassword.PasswordHash = ""

	deactivated := known
	deactivated.Status = entity.AccountStatusDeactivated

	corrupted := known
	corrupted.PasswordHash = "$argon2id$v=19$garbage"

	cases := map[string]*entity.Account{
		"no account holds the address":     nil,
		"the account has no password":      &withoutPassword,
		"the account is deactivated":       &deactivated,
		"the password is wrong":            &known,
		"the stored hash cannot be parsed": &corrupted,
	}

	want := []string{
		"count address",
		"read throttle",
		"read account",
		"audit " + string(entity.AuditSignInFailed) + " ada@example.com",
		"count failure",
	}

	for name, account := range cases {
		t.Run(name, func(t *testing.T) {
			trace, err := traceRefusedSignIn(t, account)

			var invalid entity.InvalidCredentialsError
			if !errors.As(err, &invalid) || invalid.AttemptsLeft != entity.SignInMaxFailures-2 {
				t.Fatalf("err = %v, want InvalidCredentialsError with %d attempts left", err, entity.SignInMaxFailures-2)
			}

			if !reflect.DeepEqual(trace, want) {
				t.Errorf("trace = %v\nwant    %v", trace, want)
			}
		})
	}
}
