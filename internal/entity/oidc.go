package entity

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrOIDCConnectionNotFound = errors.New("no single sign-on provider is configured for this workspace")
	ErrOIDCStateNotFound      = errors.New("this sign-in attempt has expired or was already used")
	ErrOIDCNotVerified        = errors.New("this provider has not completed a successful test")
)

type OIDCStage string

const (
	OIDCStageDiscovery     OIDCStage = "discovery"
	OIDCStageEndpoints     OIDCStage = "endpoints"
	OIDCStageJWKS          OIDCStage = "jwks"
	OIDCStageAuthorization OIDCStage = "authorization"
	OIDCStageTokenExchange OIDCStage = "token_exchange"
	OIDCStageIDToken       OIDCStage = "id_token"
	OIDCStageClaims        OIDCStage = "claims"
	OIDCStageMatching      OIDCStage = "matching"
	OIDCStageProvisioning  OIDCStage = "provisioning"
)

func (s OIDCStage) Valid() bool {
	switch s {
	case OIDCStageDiscovery, OIDCStageEndpoints, OIDCStageJWKS, OIDCStageAuthorization,
		OIDCStageTokenExchange, OIDCStageIDToken, OIDCStageClaims, OIDCStageMatching,
		OIDCStageProvisioning:
		return true
	default:
		return false
	}
}

type OIDCError struct {
	Stage   OIDCStage
	Message string
	Detail  string
	Subject string
	cause   error
}

func NewOIDCError(stage OIDCStage, message string) *OIDCError {
	return &OIDCError{Stage: stage, Message: message}
}

func OIDCFailure(stage OIDCStage, message string, cause error) *OIDCError {
	failure := &OIDCError{Stage: stage, Message: message, cause: cause}
	if cause != nil {
		failure.Detail = cause.Error()
	}

	return failure
}

func (e *OIDCError) Error() string {
	if e.Detail == "" {
		return fmt.Sprintf("%s: %s", e.Stage, e.Message)
	}

	return fmt.Sprintf("%s: %s: %s", e.Stage, e.Message, e.Detail)
}

func (e *OIDCError) Unwrap() error { return e.cause }

func AsOIDCError(err error) (*OIDCError, bool) {
	var failure *OIDCError

	return failure, errors.As(err, &failure)
}

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
			return NewOIDCError(OIDCStageEndpoints, "The "+endpoint.label+" is missing.")
		}

		if err := requireHTTPSURL(endpoint.value); err != nil {
			return NewOIDCError(
				OIDCStageEndpoints,
				"The "+endpoint.label+" is not a usable https address.",
			)
		}
	}

	return nil
}

func ValidateOIDCIssuer(issuer string) error {
	if strings.TrimSpace(issuer) == "" {
		return NewOIDCError(OIDCStageDiscovery, "Enter the issuer URL from your provider.")
	}

	if err := requireHTTPSURL(issuer); err != nil {
		return NewOIDCError(
			OIDCStageDiscovery,
			"The issuer must be an https URL, for example https://login.example.com.",
		)
	}

	return nil
}

func requireHTTPSURL(value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return err
	}

	if parsed.Host == "" {
		return errors.New("no host")
	}

	if parsed.Scheme == "https" {
		return nil
	}

	if parsed.Scheme == "http" && LoopbackHost(parsed.Hostname()) {
		return nil
	}

	return errors.New("not https")
}

func LoopbackHost(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "::1", "host.docker.internal":
		return true
	default:
		return false
	}
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
		return NewOIDCError(OIDCStageEndpoints, "Enter the client ID your provider issued.")
	}

	if strings.TrimSpace(c.ClientSecret) == "" {
		return NewOIDCError(OIDCStageEndpoints, "Enter the client secret your provider issued.")
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
		return NewOIDCError(
			OIDCStageClaims,
			"The provider did not return a subject claim identifying the user.",
		)
	}

	if strings.TrimSpace(claims.Email) == "" {
		return NewOIDCError(
			OIDCStageClaims,
			"The provider did not return an email address. Add the email scope to the client.",
		)
	}

	if claims.EmailVerified != nil && !*claims.EmailVerified {
		return NewOIDCError(
			OIDCStageClaims,
			"The provider reports "+NormalizeEmail(claims.Email)+" as unverified.",
		)
	}

	return nil
}

type MatchOutcome string

const (
	MatchOutcomeSignIn    MatchOutcome = "sign_in"
	MatchOutcomeProvision MatchOutcome = "provision"
	MatchOutcomeNotMember MatchOutcome = "not_member"
	MatchOutcomeNoAccount MatchOutcome = "no_account"
)

func ResolveMatch(accountExists, isMember, provisioning bool) MatchOutcome {
	switch {
	case accountExists && isMember:
		return MatchOutcomeSignIn
	case accountExists:
		return MatchOutcomeNotMember
	case provisioning:
		return MatchOutcomeProvision
	default:
		return MatchOutcomeNoAccount
	}
}

func (o MatchOutcome) Admits() bool {
	return o == MatchOutcomeSignIn || o == MatchOutcomeProvision
}

func (o MatchOutcome) Refusal(email string) error {
	switch o {
	case MatchOutcomeNotMember:
		failure := NewOIDCError(
			OIDCStageMatching,
			NormalizeEmail(email)+" signed in with your provider but is not a member of this workspace.",
		)
		failure.Subject = NormalizeEmail(email)

		return failure
	case MatchOutcomeNoAccount:
		failure := NewOIDCError(
			OIDCStageProvisioning,
			"There is no Norn account for "+NormalizeEmail(email)+
				", and just-in-time provisioning is turned off.",
		)
		failure.Subject = NormalizeEmail(email)

		return failure
	default:
		return nil
	}
}

type OIDCPurpose string

const (
	OIDCPurposeLogin OIDCPurpose = "login"
	OIDCPurposeTest  OIDCPurpose = "test"
)

func (p OIDCPurpose) Valid() bool {
	switch p {
	case OIDCPurposeLogin, OIDCPurposeTest:
		return true
	default:
		return false
	}
}

type OIDCState struct {
	Purpose     OIDCPurpose
	WorkspaceID uuid.UUID
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

type OIDCExchange struct {
	Purpose       OIDCPurpose
	WorkspaceID   uuid.UUID
	WorkspaceSlug string
	ReturnTo      string
	Claims        OIDCClaims
	Session       Session
	Token         string
	Provisioned   bool
}
