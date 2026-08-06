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

var personalEmailProviders = map[string]bool{
	"aol":     true,
	"gmail":   true,
	"hotmail": true,
	"icloud":  true,
	"live":    true,
	"outlook": true,
	"proton":  true,
	"yahoo":   true,
}

type LastWorkspaceAdminError struct {
	WorkspaceIDs []uuid.UUID
}

func (e LastWorkspaceAdminError) Error() string {
	return fmt.Sprintf("%s: %v", ErrAccountLastWorkspaceAdmin, e.WorkspaceIDs)
}

func (e LastWorkspaceAdminError) Unwrap() error {
	return ErrAccountLastWorkspaceAdmin
}

type AccountKind string

const (
	AccountKindPerson AccountKind = "person"
	AccountKindAgent  AccountKind = "agent"
)

func (k AccountKind) Valid() bool {
	switch k {
	case AccountKindPerson, AccountKindAgent:
		return true
	default:
		return false
	}
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
	Kind            AccountKind
	Email           string
	DisplayName     string
	AvatarObjectKey string
	Timezone        string
	PasswordHash    string
	InstanceAdmin   bool
	DeactivatedAt   *time.Time
	DeletedAt       *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (a Account) Agent() bool {
	return a.Kind == AccountKindAgent
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

func PersonalEmail(email string) bool {
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return false
	}

	domain := strings.ToLower(email[at+1:])
	label, _, found := strings.Cut(domain, ".")

	return found && personalEmailProviders[label]
}

func ValidateWorkEmail(field, email string) FieldError {
	if problem := ValidateEmail(field, email); problem.Code != "" {
		return problem
	}

	if PersonalEmail(email) {
		return FieldError{Field: field, Code: ValidationCodePersonalEmail}
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
