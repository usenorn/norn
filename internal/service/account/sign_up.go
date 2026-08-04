package account

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (s *accountsService) signUpDelivery() entity.SignUpDelivery {
	if s.smtp.Configured() {
		return entity.SignUpDeliveryMailed
	}

	return entity.SignUpDeliveryLinkOnly
}

func (s *accountsService) RequestSignUp(ctx context.Context, input service.RequestSignUpInput) (service.RequestedSignUp, error) {
	if !s.instance.SignupsOpen {
		return service.RequestedSignUp{}, entity.ErrSignUpsClosed
	}

	attempts, err := s.throttle.RecordAddressAttempt(ctx, input.Client.IP)
	if err != nil {
		return service.RequestedSignUp{}, err
	}

	if attempts > entity.SignInAddressMaxAttempts {
		return service.RequestedSignUp{}, entity.ErrSignInRateLimited
	}

	email := entity.NormalizeEmail(input.Email)

	timezone := input.Timezone
	if timezone == "" {
		timezone = entity.DefaultTimezone
	}

	if err := entity.NewValidationError(
		entity.ValidateEmail("email", email),
		entity.ValidateDisplayName("display_name", input.DisplayName),
		entity.ValidateTimezone("timezone", timezone),
		entity.ValidatePassword("password", input.Password),
	); err != nil {
		return service.RequestedSignUp{}, err
	}

	if _, err := s.accounts.GetByEmail(ctx, email); err == nil {
		return service.RequestedSignUp{}, entity.ErrAccountEmailTaken
	} else if !errors.Is(err, entity.ErrAccountNotFound) {
		return service.RequestedSignUp{}, err
	}

	if err := s.screenPassword(ctx, "password", input.Password); err != nil {
		return service.RequestedSignUp{}, err
	}

	hash, err := entity.HashPassword(input.Password)
	if err != nil {
		return service.RequestedSignUp{}, err
	}

	token, tokenHash, err := entity.NewSignUpToken()
	if err != nil {
		return service.RequestedSignUp{}, err
	}

	now := time.Now().UTC()

	var signUp entity.SignUp

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.signUps.DeletePendingByEmail(ctx, email); err != nil {
			return err
		}

		created, err := s.signUps.Create(ctx, entity.SignUp{
			Email:        email,
			DisplayName:  input.DisplayName,
			Timezone:     timezone,
			PasswordHash: hash,
			TokenHash:    tokenHash,
			RequestedAt:  now,
			ExpiresAt:    now.Add(entity.SignUpTokenTTL),
		})
		if err != nil {
			return err
		}

		signUp = created

		return nil
	})
	if err != nil {
		return service.RequestedSignUp{}, err
	}

	requested := service.RequestedSignUp{
		Email:     signUp.Email,
		ExpiresAt: signUp.ExpiresAt,
		Delivery:  s.signUpDelivery(),
	}

	if requested.Delivery == entity.SignUpDeliveryLinkOnly {
		requested.URL = s.signUpURL(token)

		return requested, nil
	}

	if err := s.producer.EnqueueSignUpVerification(ctx, entity.SignUpVerificationPayload{
		SignUpID: signUp.ID,
		Token:    token,
	}); err != nil {
		return service.RequestedSignUp{}, err
	}

	return requested, nil
}

func (s *accountsService) ConfirmSignUp(ctx context.Context, input service.ConfirmSignUpInput) (service.ConfirmedSignUp, error) {
	if input.Token == "" {
		return service.ConfirmedSignUp{}, entity.ErrSignUpTokenInvalid
	}

	signUp, err := s.signUps.GetByTokenHash(ctx, entity.HashSignUpToken(input.Token))
	if err != nil {
		return service.ConfirmedSignUp{}, err
	}

	if signUp.Confirmed() {
		return service.ConfirmedSignUp{}, entity.ErrSignUpAlreadyConfirmed
	}

	now := time.Now().UTC()

	if signUp.ExpiredAt(now) {
		return service.ConfirmedSignUp{}, entity.ErrSignUpExpired
	}

	var account entity.Account

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.signUps.MarkConfirmed(ctx, signUp.ID, now); err != nil {
			return err
		}

		created, err := s.accounts.Create(ctx, entity.Account{
			Status:       entity.AccountStatusActive,
			Email:        signUp.Email,
			DisplayName:  signUp.DisplayName,
			Timezone:     signUp.Timezone,
			PasswordHash: signUp.PasswordHash,
		})
		if err != nil {
			return err
		}

		account = created

		return nil
	})
	if err != nil {
		return service.ConfirmedSignUp{}, err
	}

	issued, err := s.sessions.Start(ctx, service.StartSessionInput{
		AccountID:  account.ID,
		AuthMethod: entity.SessionAuthMethodPassword,
		Client:     input.Client,
	})
	if err != nil {
		return service.ConfirmedSignUp{}, err
	}

	return service.ConfirmedSignUp{Account: account, Session: issued}, nil
}

func (s *accountsService) SendSignUpVerification(ctx context.Context, signUpID uuid.UUID, token string) error {
	signUp, err := s.signUps.GetByID(ctx, signUpID)
	if err != nil {
		if errors.Is(err, entity.ErrSignUpNotFound) {
			return nil
		}

		return err
	}

	if signUp.Confirmed() || signUp.ExpiredAt(time.Now().UTC()) {
		return nil
	}

	message, err := buildSignUpVerification(s.signUpURL(token), signUp.DisplayName, signUp.Email)
	if err != nil {
		return err
	}

	return s.mailer.Send(ctx, message)
}
