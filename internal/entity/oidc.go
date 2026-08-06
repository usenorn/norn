package entity

import (
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

var DefaultOIDCScopes = []string{"openid", "email", "profile"}

const RequiredOIDCScope = "openid"

func NormalizeOIDCScopes(scopes []string) []string {
	seen := make(map[string]struct{}, len(scopes)+1)
	normalized := make([]string, 0, len(scopes)+1)

	for _, scope := range scopes {
		trimmed := strings.TrimSpace(scope)
		if trimmed == "" {
			continue
		}

		if _, repeated := seen[trimmed]; repeated {
			continue
		}

		seen[trimmed] = struct{}{}

		normalized = append(normalized, trimmed)
	}

	if len(normalized) == 0 {
		return slices.Clone(DefaultOIDCScopes)
	}

	if _, present := seen[RequiredOIDCScope]; !present {
		normalized = append([]string{RequiredOIDCScope}, normalized...)
	}

	return normalized
}

type OIDCEndpoints struct {
	Issuer                string
	AuthorizationEndpoint string
	TokenEndpoint         string
	JWKSURI               string
	UserinfoEndpoint      string
}

func (e OIDCEndpoints) Validate() error {
	for _, endpoint := range []struct {
		label string
		value string
	}{
		{"authorization endpoint", e.AuthorizationEndpoint},
		{"token endpoint", e.TokenEndpoint},
		{"JWKS URI", e.JWKSURI},
	} {
		if strings.TrimSpace(endpoint.value) == "" {
			return NewSSOError(SSOStageEndpoints, "The "+endpoint.label+" is missing.")
		}

		if err := requireHTTPSURL(endpoint.value); err != nil {
			return NewSSOError(
				SSOStageEndpoints,
				"The "+endpoint.label+" is not a usable https address.",
			)
		}
	}

	return nil
}

func ValidateOIDCIssuer(issuer string) error {
	if strings.TrimSpace(issuer) == "" {
		return NewSSOError(SSOStageDiscovery, "Enter the issuer URL from your provider.")
	}

	if err := requireHTTPSURL(issuer); err != nil {
		return NewSSOError(
			SSOStageDiscovery,
			"The issuer must be an https URL, for example https://login.example.com.",
		)
	}

	return nil
}

type OIDCConnection struct {
	WorkspaceID  uuid.UUID
	Endpoints    OIDCEndpoints
	Discovered   bool
	ClientID     string
	ClientSecret string
	Scopes       []string
	GroupsClaim  string
	Provisioning bool
	VerifiedAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (c OIDCConnection) Verified() bool { return c.VerifiedAt != nil }

func (c OIDCConnection) Validate() error {
	if err := ValidateOIDCIssuer(c.Endpoints.Issuer); err != nil {
		return err
	}

	if err := c.Endpoints.Validate(); err != nil {
		return err
	}

	if strings.TrimSpace(c.ClientID) == "" {
		return NewSSOError(SSOStageEndpoints, "Enter the client ID your provider issued.")
	}

	if strings.TrimSpace(c.ClientSecret) == "" {
		return NewSSOError(SSOStageEndpoints, "Enter the client secret your provider issued.")
	}

	return nil
}

type OIDCClaims struct {
	Subject       string
	Email         string
	EmailVerified *bool
	Name          string
	Groups        []string
}

func ValidateClaims(claims OIDCClaims) error {
	if strings.TrimSpace(claims.Subject) == "" {
		return NewSSOError(
			SSOStageClaims,
			"The provider did not return a subject claim identifying the user.",
		)
	}

	if strings.TrimSpace(claims.Email) == "" {
		return NewSSOError(
			SSOStageClaims,
			"The provider did not return an email address. Add the email scope to the client.",
		)
	}

	if claims.EmailVerified != nil && !*claims.EmailVerified {
		return NewSSOError(
			SSOStageClaims,
			"The provider reports "+NormalizeEmail(claims.Email)+" as unverified.",
		)
	}

	return nil
}

type OIDCState struct {
	Purpose     SSOPurpose
	WorkspaceID uuid.UUID
	AccountID   uuid.UUID
	Nonce       string
	Verifier    string
	ReturnTo    string
	CreatedAt   time.Time
}

type OIDCAuthorization struct {
	State    string
	Nonce    string
	Verifier string
}

type OIDCRedemption struct {
	Code     string
	Nonce    string
	Verifier string
}
