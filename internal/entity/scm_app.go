package entity

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const SCMPrivateKeyMaxLen = 16384

var (
	ErrSCMAppNotFound          = errors.New("no source control application is registered")
	ErrSCMAppExists            = errors.New("an application is already registered for that forge")
	ErrSCMInstallationNotFound = errors.New("that installation is not available to you")
	ErrSCMAppUnsupported       = errors.New("this platform has no application to install")
	ErrSCMPrivateKeyInvalid    = errors.New("that is not a private key this instance can read")
	ErrSCMAppTokenUnavailable  = errors.New("the forge issued no token for that installation")
)

type SCMAuthKind string

const (
	SCMAuthToken SCMAuthKind = "token"
	SCMAuthApp   SCMAuthKind = "app"
)

func SCMAuthKinds() []SCMAuthKind {
	return []SCMAuthKind{SCMAuthToken, SCMAuthApp}
}

func (k SCMAuthKind) Valid() bool {
	return k == SCMAuthToken || k == SCMAuthApp
}

type SCMApp struct {
	ID            uuid.UUID
	Provider      SCMProvider
	BaseURL       string
	Slug          string
	ExternalAppID string
	ClientID      string
	ClientSecret  string
	PrivateKey    string
	WebhookSecret string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (a SCMApp) Registered() bool {
	return a.ExternalAppID != ""
}

func (a SCMApp) WebURL() string {
	base := strings.TrimRight(strings.TrimSpace(a.BaseURL), "/")
	if base == "" {
		return "https://github.com"
	}

	if trimmed, found := strings.CutSuffix(base, "/api/v3"); found {
		return trimmed
	}

	if scheme, host, found := strings.Cut(base, "://"); found && host == "api.github.com" {
		return scheme + "://github.com"
	}

	return base
}

func (a SCMApp) InstallURL() string {
	if a.Slug == "" {
		return a.WebURL() + "/settings/installations"
	}

	return a.WebURL() + "/apps/" + a.Slug + "/installations/new"
}

const SCMCredentialSkew = 5 * time.Minute

type SCMCredential struct {
	Token     string
	ExpiresAt time.Time
}

func (c SCMCredential) Usable(now time.Time) bool {
	if c.Token == "" {
		return false
	}

	if c.ExpiresAt.IsZero() {
		return true
	}

	return now.Add(SCMCredentialSkew).Before(c.ExpiresAt)
}

type SCMInstallation struct {
	ExternalID   string
	AccountLogin string
	AccountKind  string
	Repositories int
}

func SupportsApp(provider SCMProvider) bool {
	return provider == SCMProviderGitHub
}

func ValidateSCMPrivateKey(field, key string) FieldError {
	trimmed := strings.TrimSpace(key)

	switch {
	case trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case len(trimmed) > SCMPrivateKeyMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	case !strings.Contains(trimmed, "PRIVATE KEY"):
		return FieldError{Field: field, Code: ValidationCodeUnsupportedValue}
	default:
		return FieldError{}
	}
}
