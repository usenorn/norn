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
	audit       service.Audit
}

func New(
	sessions repository.Session,
	accounts repository.Account,
	memberships repository.Membership,
	geoLocator repository.GeoLocator,
	throttle repository.SignInThrottle,
	cfg config.Session,
	authorizer service.Authorizer,
	audit service.Audit,
) service.Sessions {
	return &sessionsService{
		sessions:    sessions,
		accounts:    accounts,
		memberships: memberships,
		geoLocator:  geoLocator,
		throttle:    throttle,
		cfg:         cfg,
		authorizer:  authorizer,
		audit:       audit,
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

func (s *sessionsService) rehashIfStale(ctx context.Context, account entity.Account, password string) {
	if !entity.PasswordNeedsRehash(account.PasswordHash) {
		return
	}

	hashed, err := entity.HashPassword(password)
	if err != nil {
		return
	}

	account.PasswordHash = hashed

	if _, err := s.accounts.Update(ctx, account); err != nil {
		logging.From(ctx).WarnContext(ctx, "could not restrengthen a password hash", "error", err.Error())
	}
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
			entity.BurnPasswordGuess(input.Password)

			return service.IssuedSession{}, s.recordFailure(ctx, subject, email, uuid.Nil)
		}

		return service.IssuedSession{}, err
	}

	if !account.CanAuthenticate() {
		entity.BurnPasswordGuess(input.Password)

		return service.IssuedSession{}, s.recordFailure(ctx, subject, email, account.ID)
	}

	matches, err := entity.VerifyPassword(account.PasswordHash, input.Password)
	if err != nil {
		return service.IssuedSession{}, err
	}

	if !matches {
		return service.IssuedSession{}, s.recordFailure(ctx, subject, email, account.ID)
	}

	s.rehashIfStale(ctx, account, input.Password)

	if err := s.throttle.Clear(ctx, subject); err != nil {
		return service.IssuedSession{}, err
	}

	return s.issue(ctx, account.ID, uuid.Nil, entity.SessionAuthMethodPassword, input.Client)
}

func (s *sessionsService) Start(ctx context.Context, input service.StartSessionInput) (service.IssuedSession, error) {
	if !input.AuthMethod.Valid() {
		return service.IssuedSession{}, entity.ErrSessionAuthMethodUnknown
	}

	return s.issue(ctx, input.AccountID, input.WorkspaceID, input.AuthMethod, input.Client)
}

func (s *sessionsService) recordFailure(
	ctx context.Context,
	subject, email string,
	accountID uuid.UUID,
) error {
	s.audit.Record(ctx, entity.AuditEntry{
		Action:  entity.AuditSignInFailed,
		Outcome: entity.AuditFailed,
		Actor: entity.AuditActor{
			Kind:       entity.ActorKindUser,
			AccountID:  accountID,
			AuthMethod: entity.SessionAuthMethodPassword,
		},
		ResourceKind: string(entity.ResourceAccount),
		ResourceID:   accountID,
		Detail:       map[string]string{"email": email},
	})

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
	accountID, workspaceID uuid.UUID,
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

	slot, err := entity.NewSessionSlot()
	if err != nil {
		return service.IssuedSession{}, err
	}

	now := time.Now().UTC()

	session := entity.Session{
		ID:                uuid.New(),
		Slot:              slot,
		TokenHash:         tokenHash,
		AccountID:         accountID,
		WorkspaceID:       workspaceID,
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

	s.audit.Record(ctx, entity.AuditEntry{
		Action: entity.AuditSignedIn,
		Actor: entity.AuditActor{
			Kind:       entity.ActorKindUser,
			AccountID:  accountID,
			AuthMethod: method,
		},
		SourceIP:     client.IP,
		UserAgent:    client.UserAgent,
		ResourceKind: string(entity.ResourceSession),
		ResourceID:   session.ID,
	})

	return service.IssuedSession{Session: session, Token: token}, nil
}

func (s *sessionsService) Inspect(ctx context.Context, token string) (entity.Session, error) {
	if token == "" {
		return entity.Session{}, entity.ErrSessionNotFound
	}

	session, err := s.sessions.Get(ctx, entity.HashSessionToken(token))
	if err != nil {
		return entity.Session{}, err
	}

	if session.ExpiredAt(time.Now().UTC()) {
		return entity.Session{}, entity.ErrSessionNotFound
	}

	revokedAt, err := s.sessions.RevokedAt(ctx, session.AccountID)
	if err != nil {
		return entity.Session{}, err
	}

	if session.RevokedAt(revokedAt) {
		return entity.Session{}, entity.ErrSessionRevoked
	}

	return session, nil
}

func (s *sessionsService) Validate(ctx context.Context, token string) (entity.Session, error) {
	session, err := s.Inspect(ctx, token)
	if err != nil {
		return entity.Session{}, err
	}

	now := time.Now().UTC()

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

	if err := s.sessions.DeleteByID(ctx, accountID, sessionID); err != nil {
		return err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		Action:       entity.AuditSignedOut,
		ResourceKind: string(entity.ResourceSession),
		ResourceID:   sessionID,
	})

	return nil
}

func (s *sessionsService) SignOutAll(ctx context.Context) error {
	held := identity.SignedIn(ctx)
	if len(held) == 0 {
		return entity.ErrAccountForbidden
	}

	for _, session := range held {
		if err := s.sessions.DeleteByID(ctx, session.AccountID, session.ID); err != nil {
			return err
		}

		s.audit.Record(ctx, entity.AuditEntry{
			Action: entity.AuditSignedOut,
			Actor: entity.AuditActor{
				Kind:      entity.ActorKindUser,
				AccountID: session.AccountID,
			},
			ResourceKind: string(entity.ResourceSession),
			ResourceID:   session.ID,
		})
	}

	return nil
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

	if err := s.sessions.DeleteByID(ctx, accountID, sessionID); err != nil {
		return err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		Action:       entity.AuditSessionRevoked,
		ResourceKind: string(entity.ResourceSession),
		ResourceID:   sessionID,
		Detail:       map[string]string{"account_id": accountID.String()},
	})

	return nil
}

func (s *sessionsService) RevokeAllByAccountID(ctx context.Context, accountID uuid.UUID) error {
	if err := s.authorizeSelf(ctx, entity.ActionDelete, accountID); err != nil {
		return err
	}

	if err := s.sessions.MarkRevoked(ctx, accountID, time.Now().UTC()); err != nil {
		return err
	}

	if err := s.sessions.DeleteByAccountID(ctx, accountID); err != nil {
		return err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		Action:       entity.AuditSessionRevoked,
		ResourceKind: string(entity.ResourceAccount),
		ResourceID:   accountID,
		Detail:       map[string]string{"scope": "all"},
	})

	return nil
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

	return s.issue(ctx, accountID, current.WorkspaceID, current.AuthMethod, current.Client)
}
