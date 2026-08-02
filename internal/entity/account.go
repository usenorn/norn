package entity

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	DisplayNameMinLen = 1
	DisplayNameMaxLen = 80
	EmailMaxLen       = 254
	DefaultTimezone   = "UTC"
)

var (
	ErrAccountNotFound           = errors.New("account not found")
	ErrAccountEmailTaken         = errors.New("account email already taken")
	ErrAccountDeactivated        = errors.New("account is deactivated")
	ErrAccountDeleted            = errors.New("account is deleted")
	ErrAccountForbidden          = errors.New("account operation not permitted")
	ErrAccountInvalidCredentials = errors.New("account credentials are invalid")
	ErrAccountPasswordSet        = errors.New("account already has a password")
	ErrAccountPasswordNotSet     = errors.New("account has no password")
	ErrAccountLastWorkspaceAdmin = errors.New("account is the last administrator of a workspace")
	ErrAccountStatusTransition   = errors.New("account status transition not allowed")
)

type LastWorkspaceAdminError struct {
	WorkspaceIDs []uuid.UUID
}

func (e LastWorkspaceAdminError) Error() string {
	return fmt.Sprintf("%s: %v", ErrAccountLastWorkspaceAdmin, e.WorkspaceIDs)
}

func (e LastWorkspaceAdminError) Unwrap() error {
	return ErrAccountLastWorkspaceAdmin
}

type AccountStatus string

const (
	AccountStatusActive      AccountStatus = "active"
	AccountStatusDeactivated AccountStatus = "deactivated"
	AccountStatusDeleted     AccountStatus = "deleted"
)

func (s AccountStatus) Valid() bool {
	switch s {
	case AccountStatusActive, AccountStatusDeactivated, AccountStatusDeleted:
		return true
	default:
		return false
	}
}

func (s AccountStatus) CanTransitionTo(target AccountStatus) bool {
	switch s {
	case AccountStatusActive:
		return target == AccountStatusDeactivated || target == AccountStatusDeleted
	case AccountStatusDeactivated:
		return target == AccountStatusActive || target == AccountStatusDeleted
	case AccountStatusDeleted:
		return false
	default:
		return false
	}
}

type Account struct {
	ID              uuid.UUID
	Status          AccountStatus
	Email           string
	DisplayName     string
	AvatarObjectKey string
	Timezone        string
	PasswordHash    string
	DeactivatedAt   *time.Time
	DeletedAt       *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (a Account) HasPassword() bool {
	return a.PasswordHash != ""
}

func (a Account) CanAuthenticate() bool {
	return a.Status == AccountStatusActive && a.HasPassword()
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func ValidateEmail(field, email string) FieldError {
	switch {
	case email == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case utf8.RuneCountInString(email) > EmailMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	}

	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || strings.Count(email, "@") != 1 {
		return FieldError{Field: field, Code: ValidationCodeMalformed}
	}

	return FieldError{}
}

func ValidateDisplayName(field, displayName string) FieldError {
	length := utf8.RuneCountInString(strings.TrimSpace(displayName))

	switch {
	case length < DisplayNameMinLen:
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case length > DisplayNameMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func ValidateTimezone(field, timezone string) FieldError {
	if timezone == "" {
		return FieldError{Field: field, Code: ValidationCodeRequired}
	}

	if _, err := time.LoadLocation(timezone); err != nil {
		return FieldError{Field: field, Code: ValidationCodeUnknownTimezone}
	}

	return FieldError{}
}
