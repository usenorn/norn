package entity

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	WorkspaceDomainMaxLen      = 253
	WorkspaceDomainLabelMaxLen = 63
	WorkspaceDomainTokenBytes  = 24
	WorkspaceDomainRecordName  = "norn-verification"

	WorkspaceDomainLookupTimeout = 5 * time.Second
)

var (
	ErrWorkspaceDomainNotFound     = errors.New("this workspace has not claimed that domain")
	ErrWorkspaceDomainTaken        = errors.New("this workspace has already claimed that domain")
	ErrWorkspaceDomainLookupFailed = errors.New("the domain's dns records could not be read")
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

type WorkspaceDomain struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	Domain      string
	Token       string
	VerifiedAt  *time.Time
	CheckedAt   *time.Time
	CreatedAt   time.Time
}

func (d WorkspaceDomain) Verified() bool {
	return d.VerifiedAt != nil
}

func (d WorkspaceDomain) Record() string {
	return WorkspaceDomainRecordName + "=" + d.Token
}

func NewWorkspaceDomainToken() (string, error) {
	raw := make([]byte, WorkspaceDomainTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate domain verification token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func NormalizeDomain(raw string) (string, bool) {
	domain := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(raw)), ".")

	if domain == "" || len(domain) > WorkspaceDomainMaxLen {
		return "", false
	}

	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return "", false
	}

	for _, label := range labels {
		if !hostnameLabel(label) {
			return "", false
		}
	}

	return domain, true
}

func hostnameLabel(label string) bool {
	if label == "" || len(label) > WorkspaceDomainLabelMaxLen {
		return false
	}

	if label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}

	for _, r := range label {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}

	return true
}

func EmailDomain(email string) (string, bool) {
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return "", false
	}

	return NormalizeDomain(email[at+1:])
}

func PersonalEmailDomain(domain string) bool {
	label, _, found := strings.Cut(domain, ".")

	return found && personalEmailProviders[label]
}

func ValidateWorkspaceDomain(field, raw string) (string, FieldError) {
	if strings.TrimSpace(raw) == "" {
		return "", FieldError{Field: field, Code: ValidationCodeRequired}
	}

	domain, ok := NormalizeDomain(raw)
	if !ok {
		return "", FieldError{Field: field, Code: ValidationCodeMalformed}
	}

	if PersonalEmailDomain(domain) {
		return "", FieldError{Field: field, Code: ValidationCodePersonalDomain}
	}

	return domain, FieldError{}
}
