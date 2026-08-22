package account

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type accountsService struct {
	accounts        repository.Account
	emailChanges    repository.EmailChange
	passwordResets  repository.PasswordReset
	signUps         repository.SignUp
	passwordHistory repository.PasswordHistory
	memberships     repository.Membership
	workspaces      repository.Workspace
	authPolicies    repository.WorkspaceAuthPolicy
	breaches        repository.BreachCheck
	throttle        repository.SignInThrottle
	blobs           repository.Blob
	mailer          repository.Mailer
	producer        repository.JobProducer
	transactor      repository.Transactor
	sessions        service.Sessions
	authorizer      service.Authorizer
	app             config.App
	instance        config.Instance
	attachments     config.Attachments
	audit           service.Audit
}

func New(
	accounts repository.Account,
	emailChanges repository.EmailChange,
	passwordResets repository.PasswordReset,
	signUps repository.SignUp,
	passwordHistory repository.PasswordHistory,
	memberships repository.Membership,
	workspaces repository.Workspace,
	authPolicies repository.WorkspaceAuthPolicy,
	breaches repository.BreachCheck,
	throttle repository.SignInThrottle,
	blobs repository.Blob,
	mailer repository.Mailer,
	producer repository.JobProducer,
	transactor repository.Transactor,
	sessions service.Sessions,
	authorizer service.Authorizer,
	app config.App,
	instance config.Instance,
	attachments config.Attachments,
	audit service.Audit,
) service.Accounts {
	return &accountsService{
		accounts:        accounts,
		emailChanges:    emailChanges,
		passwordResets:  passwordResets,
		signUps:         signUps,
		passwordHistory: passwordHistory,
		memberships:     memberships,
		workspaces:      workspaces,
		authPolicies:    authPolicies,
		breaches:        breaches,
		throttle:        throttle,
		blobs:           blobs,
		mailer:          mailer,
		producer:        producer,
		transactor:      transactor,
		sessions:        sessions,
		authorizer:      authorizer,
		app:             app,
		instance:        instance,
		attachments:     attachments,
		audit:           audit,
	}
}

func (s *accountsService) authorizeSelf(ctx context.Context, action entity.Action, accountID uuid.UUID) error {
	_, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource: entity.ResourceAccount,
		Action:   action,
		Subject:  accountID,
	})

	return err
}

func (s *accountsService) Register(ctx context.Context, input service.RegisterAccountInput) (entity.Account, error) {
	email := entity.NormalizeEmail(input.Email)

	timezone := input.Timezone
	if timezone == "" {
		timezone = entity.DefaultTimezone
	}

	fields := []entity.FieldError{
		entity.ValidateEmail("email", email),
		entity.ValidateDisplayName("display_name", input.DisplayName),
		entity.ValidateTimezone("timezone", timezone),
	}

	if input.Password != "" {
		fields = append(fields, entity.ValidatePassword("password", input.Password))
	}

	if err := entity.NewValidationError(fields...); err != nil {
		return entity.Account{}, err
	}

	account := entity.Account{
		Status:      entity.AccountStatusActive,
		Email:       email,
		DisplayName: input.DisplayName,
		Timezone:    timezone,
	}

	if input.Password != "" {
		hash, err := entity.HashPassword(input.Password)
		if err != nil {
			return entity.Account{}, err
		}

		account.PasswordHash = hash
	}

	return s.accounts.Create(ctx, account)
}

func (s *accountsService) Get(ctx context.Context, accountID uuid.UUID) (entity.Account, error) {
	if err := s.authorizeSelf(ctx, entity.ActionUpdate, accountID); err != nil {
		return entity.Account{}, err
	}

	return s.accounts.GetByID(ctx, accountID)
}

func (s *accountsService) UpdateProfile(ctx context.Context, accountID uuid.UUID, input service.UpdateProfileInput) (entity.Account, error) {
	if err := s.authorizeSelf(ctx, entity.ActionUpdate, accountID); err != nil {
		return entity.Account{}, err
	}

	account, err := s.activeAccount(ctx, accountID)
	if err != nil {
		return entity.Account{}, err
	}

	var fields []entity.FieldError

	if input.DisplayName != nil {
		fields = append(fields, entity.ValidateDisplayName("display_name", *input.DisplayName))
	}

	if input.Timezone != nil {
		fields = append(fields, entity.ValidateTimezone("timezone", *input.Timezone))
	}

	if err := entity.NewValidationError(fields...); err != nil {
		return entity.Account{}, err
	}

	if input.DisplayName != nil {
		account.DisplayName = *input.DisplayName
	}

	if input.Timezone != nil {
		account.Timezone = *input.Timezone
	}

	return s.accounts.Update(ctx, account)
}

func (s *accountsService) PendingEmailChange(ctx context.Context, accountID uuid.UUID) (entity.EmailChange, error) {
	if err := s.authorizeSelf(ctx, entity.ActionUpdate, accountID); err != nil {
		return entity.EmailChange{}, err
	}

	return s.emailChanges.GetPendingByAccountID(ctx, accountID)
}

func (s *accountsService) RequestEmailChange(ctx context.Context, accountID uuid.UUID, newEmail string) (entity.EmailChange, error) {
	if err := s.authorizeSelf(ctx, entity.ActionUpdate, accountID); err != nil {
		return entity.EmailChange{}, err
	}

	email := entity.NormalizeEmail(newEmail)

	if err := entity.NewValidationError(entity.ValidateEmail("email", email)); err != nil {
		return entity.EmailChange{}, err
	}

	account, err := s.activeAccount(ctx, accountID)
	if err != nil {
		return entity.EmailChange{}, err
	}

	if entity.NormalizeEmail(account.Email) == email {
		return entity.EmailChange{}, entity.ErrEmailChangeSameAddress
	}

	if _, err := s.accounts.GetByEmail(ctx, email); err == nil {
		return entity.EmailChange{}, entity.ErrAccountEmailTaken
	} else if !errors.Is(err, entity.ErrAccountNotFound) {
		return entity.EmailChange{}, err
	}

	token, tokenHash, err := entity.NewEmailChangeToken()
	if err != nil {
		return entity.EmailChange{}, err
	}

	now := time.Now().UTC()

	var change entity.EmailChange

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.emailChanges.DeletePendingByAccountID(ctx, accountID); err != nil {
			return err
		}

		created, err := s.emailChanges.Create(ctx, entity.EmailChange{
			AccountID:   accountID,
			NewEmail:    email,
			TokenHash:   tokenHash,
			RequestedAt: now,
			ExpiresAt:   now.Add(entity.EmailChangeTokenTTL),
		})
		if err != nil {
			return err
		}

		change = created

		return nil
	})
	if err != nil {
		return entity.EmailChange{}, err
	}

	if err := s.producer.EnqueueEmailChangeConfirmation(ctx, entity.EmailChangeConfirmationPayload{
		EmailChangeID: change.ID,
		Token:         token,
	}); err != nil {
		return entity.EmailChange{}, err
	}

	return change, nil
}

func (s *accountsService) ConfirmEmailChange(ctx context.Context, token string) (entity.Account, error) {
	if token == "" {
		return entity.Account{}, entity.ErrEmailChangeTokenInvalid
	}

	var account entity.Account

	err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		change, err := s.emailChanges.GetByTokenHash(ctx, entity.HashEmailChangeToken(token))
		if err != nil {
			return err
		}

		if change.Confirmed() {
			return entity.ErrEmailChangeAlreadyDone
		}

		now := time.Now().UTC()

		if change.ExpiredAt(now) {
			return entity.ErrEmailChangeExpired
		}

		current, err := s.activeAccount(ctx, change.AccountID)
		if err != nil {
			return err
		}

		current.Email = change.NewEmail

		updated, err := s.accounts.Update(ctx, current)
		if err != nil {
			return err
		}

		if err := s.emailChanges.MarkConfirmed(ctx, change.ID, now); err != nil {
			return err
		}

		account = updated

		return nil
	})
	if err != nil {
		return entity.Account{}, err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		Action: entity.AuditEmailChanged,
		Actor: entity.AuditActor{
			Kind:      entity.ActorKindUser,
			AccountID: account.ID,
		},
		ResourceKind: string(entity.ResourceAccount),
		ResourceID:   account.ID,
		Detail:       map[string]string{"email": account.Email},
	})

	return account, nil
}

func (s *accountsService) SendEmailChangeConfirmation(ctx context.Context, changeID uuid.UUID, token string) error {
	change, err := s.emailChanges.GetByID(ctx, changeID)
	if err != nil {
		if errors.Is(err, entity.ErrEmailChangeNotFound) {
			return nil
		}

		return err
	}

	if change.Confirmed() || change.ExpiredAt(time.Now().UTC()) {
		return nil
	}

	account, err := s.accounts.GetByID(ctx, change.AccountID)
	if err != nil {
		if errors.Is(err, entity.ErrAccountNotFound) {
			return nil
		}

		return err
	}

	message, err := buildEmailChangeConfirmation(s.app.BaseURL, account.DisplayName, change.NewEmail, token)
	if err != nil {
		return err
	}

	return s.mailer.Send(ctx, message)
}

func (s *accountsService) Deactivate(ctx context.Context, accountID uuid.UUID) (entity.Account, error) {
	if err := s.authorizeSelf(ctx, entity.ActionUpdate, accountID); err != nil {
		return entity.Account{}, err
	}

	var account entity.Account

	err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		current, err := s.accounts.GetByID(ctx, accountID)
		if err != nil {
			return err
		}

		if !current.Status.CanTransitionTo(entity.AccountStatusDeactivated) {
			return entity.ErrAccountStatusTransition
		}

		if err := s.guardLastWorkspaceAdmin(ctx, accountID); err != nil {
			return err
		}

		if err := s.sessions.RevokeAllByAccountID(ctx, accountID); err != nil {
			return err
		}

		now := time.Now().UTC()
		current.Status = entity.AccountStatusDeactivated
		current.DeactivatedAt = &now

		updated, err := s.accounts.Update(ctx, current)
		if err != nil {
			return err
		}

		account = updated

		return nil
	})
	if err != nil {
		return entity.Account{}, err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		Action:       entity.AuditAccountDisabled,
		ResourceKind: string(entity.ResourceAccount),
		ResourceID:   account.ID,
	})

	return account, nil
}

func (s *accountsService) Delete(ctx context.Context, accountID uuid.UUID) error {
	if err := s.authorizeSelf(ctx, entity.ActionUpdate, accountID); err != nil {
		return err
	}

	var avatarObjectKey string

	err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		current, err := s.accounts.GetByID(ctx, accountID)
		if err != nil {
			return err
		}

		if !current.Status.CanTransitionTo(entity.AccountStatusDeleted) {
			return entity.ErrAccountStatusTransition
		}

		if err := s.guardLastWorkspaceAdmin(ctx, accountID); err != nil {
			return err
		}

		if err := s.sessions.RevokeAllByAccountID(ctx, accountID); err != nil {
			return err
		}

		if err := s.emailChanges.DeletePendingByAccountID(ctx, accountID); err != nil {
			return err
		}

		if err := s.passwordResets.DeletePendingByAccountID(ctx, accountID); err != nil {
			return err
		}

		if err := s.passwordHistory.DeleteByAccountID(ctx, accountID); err != nil {
			return err
		}

		if err := s.memberships.DeleteByAccountID(ctx, accountID); err != nil {
			return err
		}

		avatarObjectKey = current.AvatarObjectKey

		now := time.Now().UTC()
		current.Status = entity.AccountStatusDeleted
		current.Email = ""
		current.DisplayName = ""
		current.Timezone = ""
		current.PasswordHash = ""
		current.AvatarObjectKey = ""
		current.DeactivatedAt = nil
		current.DeletedAt = &now

		if _, err := s.accounts.Update(ctx, current); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	s.discardAvatar(ctx, avatarObjectKey)

	s.audit.Record(ctx, entity.AuditEntry{
		Action:       entity.AuditAccountDeleted,
		ResourceKind: string(entity.ResourceAccount),
		ResourceID:   accountID,
	})

	return nil
}

func (s *accountsService) guardLastWorkspaceAdmin(ctx context.Context, accountID uuid.UUID) error {
	adminWorkspaceIDs, err := s.memberships.ListAdminWorkspaceIDs(ctx, accountID)
	if err != nil {
		return err
	}

	if len(adminWorkspaceIDs) == 0 {
		return nil
	}

	if err := s.workspaces.LockByIDs(ctx, adminWorkspaceIDs); err != nil {
		return err
	}

	soleAdminWorkspaceIDs, err := s.memberships.ListWorkspaceIDsWithoutOtherActiveAdmin(ctx, accountID)
	if err != nil {
		return err
	}

	if len(soleAdminWorkspaceIDs) > 0 {
		return entity.LastWorkspaceAdminError{WorkspaceIDs: soleAdminWorkspaceIDs}
	}

	return nil
}

func (s *accountsService) activeAccount(ctx context.Context, accountID uuid.UUID) (entity.Account, error) {
	account, err := s.accounts.GetByID(ctx, accountID)
	if err != nil {
		return entity.Account{}, err
	}

	switch account.Status {
	case entity.AccountStatusActive:
		return account, nil
	case entity.AccountStatusDeactivated:
		return entity.Account{}, entity.ErrAccountDeactivated
	case entity.AccountStatusDeleted:
		return entity.Account{}, entity.ErrAccountDeleted
	default:
		return entity.Account{}, fmt.Errorf("account %s has unknown status %q", accountID, account.Status)
	}
}
