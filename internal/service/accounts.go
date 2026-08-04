package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=accounts.go -destination=account/mock_accounts.go -package=account -mock_names=Accounts=MockAccounts

type Accounts interface {
	Register(ctx context.Context, input RegisterAccountInput) (entity.Account, error)
	RequestSignUp(ctx context.Context, input RequestSignUpInput) (RequestedSignUp, error)
	ConfirmSignUp(ctx context.Context, input ConfirmSignUpInput) (ConfirmedSignUp, error)
	SendSignUpVerification(ctx context.Context, signUpID uuid.UUID, token string) error
	Get(ctx context.Context, accountID uuid.UUID) (entity.Account, error)
	UpdateProfile(ctx context.Context, accountID uuid.UUID, input UpdateProfileInput) (entity.Account, error)
	PendingEmailChange(ctx context.Context, accountID uuid.UUID) (entity.EmailChange, error)
	RequestEmailChange(ctx context.Context, accountID uuid.UUID, newEmail string) (entity.EmailChange, error)
	ConfirmEmailChange(ctx context.Context, token string) (entity.Account, error)
	SendEmailChangeConfirmation(ctx context.Context, changeID uuid.UUID, token string) error
	RequestPasswordReset(ctx context.Context, input RequestPasswordResetInput) (time.Time, error)
	ConfirmPasswordReset(ctx context.Context, token, password string) error
	SendPasswordReset(ctx context.Context, resetID uuid.UUID, token string) error
	SendPasswordResetSSONotice(ctx context.Context, accountID uuid.UUID) error
	UploadAvatar(ctx context.Context, accountID uuid.UUID, upload AvatarUpload) (entity.Account, error)
	RemoveAvatar(ctx context.Context, accountID uuid.UUID) (entity.Account, error)
	SetPassword(ctx context.Context, accountID uuid.UUID, password string) (IssuedSession, error)
	ChangePassword(ctx context.Context, accountID uuid.UUID, currentPassword, newPassword string) (IssuedSession, error)
	Deactivate(ctx context.Context, accountID uuid.UUID) (entity.Account, error)
	Delete(ctx context.Context, accountID uuid.UUID) error
}
