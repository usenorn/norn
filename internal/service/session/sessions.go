package session

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type sessionsService struct {
	sessions    repository.Session
	accounts    repository.Account
	memberships repository.Membership
	geoLocator  repository.GeoLocator
	throttle    repository.SignInThrottle
	cfg         config.Session
	authorizer  service.Authorizer
}

func New(
	sessions repository.Session,
	accounts repository.Account,
	memberships repository.Membership,
	geoLocator repository.GeoLocator,
	throttle repository.SignInThrottle,
	cfg config.Session,
	authorizer service.Authorizer,
) service.Sessions {
	return &sessionsService{
		sessions:    sessions,
		accounts:    accounts,
		memberships: memberships,
		geoLocator:  geoLocator,
		throttle:    throttle,
		cfg:         cfg,
		authorizer:  authorizer,
	}
}

func (s *sessionsService) authorizeSelf(ctx context.Context, action entity.Action, accountID uuid.UUID) error {
	_, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource: entity.ResourceSession,
		Action:   action,
		Subject:  accountID,
	})

	return err
}

func (s *sessionsService) SignIn(ctx context.Context, input service.SignInInput) (service.IssuedSession, error) {
	attempts, err := s.throttle.RecordAddressAttempt(ctx, input.Client.IP)
	if err != nil {
		return service.IssuedSession{}, err
	}

	if attempts > entity.SignInAddressMaxAttempts {
		return service.IssuedSession{}, entity.ErrSignInRateLimited
	}

	email := entity.NormalizeEmail(input.Email)
	subject := entity.HashSignInSubject(email)

	throttle, err := s.throttle.Get(ctx, subject)
	if err != nil {
		return service.IssuedSession{}, err
	}

	if throttle.Locked(time.Now().UTC()) {
		return service.IssuedSession{}, entity.AccountLockedError{UnlocksAt: throttle.LockedUntil}
	}

	if err := pause(ctx, throttle.Delay()); err != nil {
		return service.IssuedSession{}, err
	}

	account, err := s.accounts.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, entity.ErrAccountNotFound) {
			return service.IssuedSession{}, s.recordFailure(ctx, subject)
		}

		return service.IssuedSession{}, err
	}

	if !account.CanAuthenticate() {
		return service.IssuedSession{}, s.recordFailure(ctx, subject)
	}

	matches, err := entity.VerifyPassword(account.PasswordHash, input.Password)
	if err != nil {
		return service.IssuedSession{}, err
	}

	if !matches {
		return service.IssuedSession{}, s.recordFailure(ctx, subject)
	}

	if err := s.throttle.Clear(ctx, subject); err != nil {
		return service.IssuedSession{}, err
	}

	return s.issue(ctx, account.ID, entity.SessionAuthMethodPassword, input.Client)
}

func (s *sessionsService) Start(ctx context.Context, input service.StartSessionInput) (service.IssuedSession, error) {
	return s.issue(ctx, input.AccountID, entity.SessionAuthMethodPassword, input.Client)
}

func (s *sessionsService) recordFailure(ctx context.Context, subject string) error {
	throttle, err := s.throttle.RecordFailure(ctx, subject)
	if err != nil {
		return err
	}

	if throttle.Locked(time.Now().UTC()) {
		return entity.AccountLockedError{UnlocksAt: throttle.LockedUntil}
	}

	return entity.InvalidCredentialsError{AttemptsLeft: throttle.AttemptsLeft()}
}

func pause(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *sessionsService) issue(
	ctx context.Context,
	accountID uuid.UUID,
	method entity.SessionAuthMethod,
	client entity.SessionClient,
) (service.IssuedSession, error) {
	located, err := s.geoLocator.Locate(ctx, client.IP)
	if err != nil {
		return service.IssuedSession{}, err
	}

	client.Location = located
	client.UserAgent = entity.TruncateUserAgent(client.UserAgent)

	token, tokenHash, err := entity.NewSessionToken()
	if err != nil {
		return service.IssuedSession{}, err
	}

	now := time.Now().UTC()

	session := entity.Session{
		ID:                uuid.New(),
		TokenHash:         tokenHash,
		AccountID:         accountID,
		AuthMethod:        method,
		Client:            client,
		IssuedAt:          now,
		LastUsedAt:        now,
		IdleExpiresAt:     now.Add(s.cfg.IdleTimeout),
		AbsoluteExpiresAt: now.Add(s.cfg.AbsoluteLifetime),
	}

	if err := s.sessions.Create(ctx, session); err != nil {
		return service.IssuedSession{}, err
	}

	s.recordMembershipActivity(ctx, session)

	return service.IssuedSession{Session: session, Token: token}, nil
}

func (s *sessionsService) Validate(ctx context.Context, token string) (entity.Session, error) {
	if token == "" {
		return entity.Session{}, entity.ErrSessionNotFound
	}

	session, err := s.sessions.Get(ctx, entity.HashSessionToken(token))
	if err != nil {
		return entity.Session{}, err
	}

	now := time.Now().UTC()

	if session.ExpiredAt(now) {
		return entity.Session{}, entity.ErrSessionNotFound
	}

	revokedAt, err := s.sessions.RevokedAt(ctx, session.AccountID)
	if err != nil {
		return entity.Session{}, err
	}

	if session.RevokedAt(revokedAt) {
		return entity.Session{}, entity.ErrSessionRevoked
	}

	if !session.NeedsRefresh(now, s.cfg.RefreshInterval) {
		return session, nil
	}

	refreshed := session.Refreshed(now, s.cfg.IdleTimeout)

	if err := s.sessions.Touch(ctx, refreshed); err != nil {
		return entity.Session{}, err
	}

	s.recordMembershipActivity(ctx, refreshed)

	return refreshed, nil
}

func (s *sessionsService) recordMembershipActivity(ctx context.Context, session entity.Session) {
	if err := s.memberships.RecordActivity(ctx, session.AccountID, session.LastUsedAt, session.AuthMethod); err != nil {
		logging.From(ctx).WarnContext(
			ctx,
			"recording membership activity failed",
			"account_id", session.AccountID.String(),
			"error", err.Error(),
		)
	}
}

func (s *sessionsService) SignOut(ctx context.Context, sessionID uuid.UUID) error {
	accountID, ok := identity.From(ctx)
	if !ok {
		return entity.ErrAccountForbidden
	}

	return s.sessions.DeleteByID(ctx, accountID, sessionID)
}

func (s *sessionsService) List(ctx context.Context, accountID uuid.UUID) ([]entity.Session, error) {
	if err := s.authorizeSelf(ctx, entity.ActionRead, accountID); err != nil {
		return nil, err
	}

	return s.sessions.ListByAccountID(ctx, accountID)
}

func (s *sessionsService) Revoke(ctx context.Context, accountID, sessionID uuid.UUID) error {
	if err := s.authorizeSelf(ctx, entity.ActionDelete, accountID); err != nil {
		return err
	}

	return s.sessions.DeleteByID(ctx, accountID, sessionID)
}

func (s *sessionsService) RevokeAllByAccountID(ctx context.Context, accountID uuid.UUID) error {
	if err := s.authorizeSelf(ctx, entity.ActionDelete, accountID); err != nil {
		return err
	}

	if err := s.sessions.MarkRevoked(ctx, accountID, time.Now().UTC()); err != nil {
		return err
	}

	return s.sessions.DeleteByAccountID(ctx, accountID)
}

func (s *sessionsService) RotateAfterCredentialChange(ctx context.Context, accountID uuid.UUID) (service.IssuedSession, error) {
	current, hasCurrent := identity.CurrentSession(ctx)

	if err := s.sessions.MarkRevoked(ctx, accountID, time.Now().UTC()); err != nil {
		return service.IssuedSession{}, err
	}

	if err := s.sessions.DeleteByAccountID(ctx, accountID); err != nil {
		return service.IssuedSession{}, err
	}

	if !hasCurrent {
		return service.IssuedSession{}, nil
	}

	return s.issue(ctx, accountID, current.AuthMethod, current.Client)
}
