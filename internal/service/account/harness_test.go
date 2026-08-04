package account_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	accountrepo "github.com/usenorn/norn/internal/repository/account"
	blobrepo "github.com/usenorn/norn/internal/repository/blob"
	breachcheckrepo "github.com/usenorn/norn/internal/repository/breachcheck"
	emailchangerepo "github.com/usenorn/norn/internal/repository/emailchange"
	jobqueuerepo "github.com/usenorn/norn/internal/repository/jobqueue"
	mailerrepo "github.com/usenorn/norn/internal/repository/mailer"
	membershiprepo "github.com/usenorn/norn/internal/repository/membership"
	passwordhistoryrepo "github.com/usenorn/norn/internal/repository/passwordhistory"
	passwordresetrepo "github.com/usenorn/norn/internal/repository/passwordreset"
	signinthrottlerepo "github.com/usenorn/norn/internal/repository/signinthrottle"
	signuprepo "github.com/usenorn/norn/internal/repository/signup"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	workspacerepo "github.com/usenorn/norn/internal/repository/workspace"
	workspaceauthpolicyrepo "github.com/usenorn/norn/internal/repository/workspaceauthpolicy"
	"github.com/usenorn/norn/internal/service"
	accountsvc "github.com/usenorn/norn/internal/service/account"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	sessionsvc "github.com/usenorn/norn/internal/service/session"
)

const (
	baseURL      = "https://norn.test"
	rotatedToken = "a-rotated-session-token"
)

type harness struct {
	accounts        *accountrepo.MockAccount
	emailChanges    *emailchangerepo.MockEmailChange
	passwordResets  *passwordresetrepo.MockPasswordReset
	signUps         *signuprepo.MockSignUp
	passwordHistory *passwordhistoryrepo.MockPasswordHistory
	memberships     *membershiprepo.MockMembership
	workspaces      *workspacerepo.MockWorkspace
	authPolicies    *workspaceauthpolicyrepo.MockWorkspaceAuthPolicy
	breaches        *breachcheckrepo.MockBreachCheck
	throttle        *signinthrottlerepo.MockSignInThrottle
	blobs           *blobrepo.MockBlob
	mailer          *mailerrepo.MockMailer
	producer        *jobqueuerepo.MockJobProducer
	transactor      *transactorrepo.MockTransactor
	sessions        *sessionsvc.MockSessions
	authorizer      *authorizersvc.MockAuthorizer
	service         service.Accounts
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		accounts:        accountrepo.NewMockAccount(ctrl),
		emailChanges:    emailchangerepo.NewMockEmailChange(ctrl),
		passwordResets:  passwordresetrepo.NewMockPasswordReset(ctrl),
		signUps:         signuprepo.NewMockSignUp(ctrl),
		passwordHistory: passwordhistoryrepo.NewMockPasswordHistory(ctrl),
		memberships:     membershiprepo.NewMockMembership(ctrl),
		workspaces:      workspacerepo.NewMockWorkspace(ctrl),
		authPolicies:    workspaceauthpolicyrepo.NewMockWorkspaceAuthPolicy(ctrl),
		breaches:        breachcheckrepo.NewMockBreachCheck(ctrl),
		throttle:        signinthrottlerepo.NewMockSignInThrottle(ctrl),
		blobs:           blobrepo.NewMockBlob(ctrl),
		mailer:          mailerrepo.NewMockMailer(ctrl),
		producer:        jobqueuerepo.NewMockJobProducer(ctrl),
		transactor:      transactorrepo.NewMockTransactor(ctrl),
		sessions:        sessionsvc.NewMockSessions(ctrl),
		authorizer:      authorizersvc.NewMockAuthorizer(ctrl),
	}

	h.transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
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

	h.service = newServiceWithSMTP(h, config.SMTP{Host: "smtp.test", FromAddress: "no-reply@norn.test"})

	return h
}

func newServiceWithSMTP(h *harness, smtp config.SMTP) service.Accounts {
	return newService(h, smtp, config.Instance{SignupsOpen: true, PasswordAuth: true})
}

func newServiceWithInstance(h *harness, instance config.Instance) service.Accounts {
	return newService(h, config.SMTP{Host: "smtp.test", FromAddress: "no-reply@norn.test"}, instance)
}

func newService(h *harness, smtp config.SMTP, instance config.Instance) service.Accounts {
	return accountsvc.New(
		h.accounts,
		h.emailChanges,
		h.passwordResets,
		h.signUps,
		h.passwordHistory,
		h.memberships,
		h.workspaces,
		h.authPolicies,
		h.breaches,
		h.throttle,
		h.blobs,
		h.mailer,
		h.producer,
		h.transactor,
		h.sessions,
		h.authorizer,
		config.App{BaseURL: baseURL},
		smtp,
		instance,
		config.Attachments{LinkTTL: 5 * time.Minute},
	)
}

func (h *harness) expectPasswordAccepted(accountID uuid.UUID) {
	h.breaches.EXPECT().Compromised(gomock.Any(), gomock.Any()).Return(false, nil)
	h.passwordHistory.EXPECT().
		ListRecentByAccountID(gomock.Any(), accountID, entity.PasswordHistoryDepth).
		Return(nil, nil)
}

func (h *harness) expectPasswordRecorded(accountID uuid.UUID) {
	h.passwordHistory.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, entry entity.PasswordHistoryEntry) (entity.PasswordHistoryEntry, error) {
			return entry, nil
		})
	h.passwordHistory.EXPECT().
		PruneByAccountID(gomock.Any(), accountID, entity.PasswordHistoryDepth).
		Return(nil)
}

func (h *harness) expectNoSSOEnforcement(accountID uuid.UUID) {
	h.authPolicies.EXPECT().
		ListEnforcementsByAccountID(gomock.Any(), accountID).
		Return([]entity.AuthEnforcement{entity.AuthEnforcementAny}, nil)
}

func (h *harness) expectSSOEnforcedEverywhere(accountID uuid.UUID) {
	h.authPolicies.EXPECT().
		ListEnforcementsByAccountID(gomock.Any(), accountID).
		Return([]entity.AuthEnforcement{entity.AuthEnforcementSSO}, nil)
}

func (h *harness) expectSessionsRevoked(accountID uuid.UUID) {
	h.sessions.EXPECT().RevokeAllByAccountID(gomock.Any(), accountID).Return(nil)
}

func rotatedSession() service.IssuedSession {
	return service.IssuedSession{
		Session: entity.Session{ID: uuid.New(), AuthMethod: entity.SessionAuthMethodPassword},
		Token:   rotatedToken,
	}
}

func (h *harness) expectSessionRotated(accountID uuid.UUID) {
	h.sessions.EXPECT().RotateAfterCredentialChange(gomock.Any(), accountID).Return(rotatedSession(), nil)
}

func (h *harness) expectNoAdminWorkspaces(accountID uuid.UUID) {
	h.memberships.EXPECT().ListAdminWorkspaceIDs(gomock.Any(), accountID).Return(nil, nil)
}

func (h *harness) expectSoleAdminOf(accountID, workspaceID uuid.UUID) {
	h.memberships.EXPECT().ListAdminWorkspaceIDs(gomock.Any(), accountID).Return([]uuid.UUID{workspaceID}, nil)
	h.workspaces.EXPECT().LockByIDs(gomock.Any(), []uuid.UUID{workspaceID}).Return(nil)
	h.memberships.EXPECT().
		ListWorkspaceIDsWithoutOtherActiveAdmin(gomock.Any(), accountID).
		Return([]uuid.UUID{workspaceID}, nil)
}

func (h *harness) expectAdminWithPeers(accountID, workspaceID uuid.UUID) {
	h.memberships.EXPECT().ListAdminWorkspaceIDs(gomock.Any(), accountID).Return([]uuid.UUID{workspaceID}, nil)
	h.workspaces.EXPECT().LockByIDs(gomock.Any(), []uuid.UUID{workspaceID}).Return(nil)
	h.memberships.EXPECT().
		ListWorkspaceIDsWithoutOtherActiveAdmin(gomock.Any(), accountID).
		Return(nil, nil)
}

func actingAs(accountID uuid.UUID) context.Context {
	return identity.Into(context.Background(), accountID)
}

func activeAccount(id uuid.UUID) entity.Account {
	return entity.Account{
		ID:          id,
		Status:      entity.AccountStatusActive,
		Email:       "ada@example.com",
		DisplayName: "Ada Lovelace",
		Timezone:    "Europe/London",
		CreatedAt:   time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
}
