package entity

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrSSOConnectionNotFound = errors.New("no single sign-on provider is configured for this workspace")
	ErrSSOStateNotFound      = errors.New("this sign-in attempt has expired or was already used")
	ErrSSONotVerified        = errors.New("this provider has not completed a successful test")

	ErrSSOEncryptionKeyMissing = errors.New(
		"this instance has no encryption key, so a provider secret cannot be stored. " +
			"Set NORN_SECURITY_ENCRYPTION_KEY to 32 base64-encoded random bytes and restart",
	)
)

type SSOProtocol string

const (
	SSOProtocolOIDC SSOProtocol = "oidc"
	SSOProtocolSAML SSOProtocol = "saml"
)

func (p SSOProtocol) Valid() bool {
	switch p {
	case SSOProtocolOIDC, SSOProtocolSAML:
		return true
	default:
		return false
	}
}

type SSOStage string

const (
	SSOStageDiscovery     SSOStage = "discovery"
	SSOStageEndpoints     SSOStage = "endpoints"
	SSOStageJWKS          SSOStage = "jwks"
	SSOStageAuthorization SSOStage = "authorization"
	SSOStageTokenExchange SSOStage = "token_exchange"
	SSOStageIDToken       SSOStage = "id_token"
	SSOStageClaims        SSOStage = "claims"
	SSOStageMatching      SSOStage = "matching"
	SSOStageProvisioning  SSOStage = "provisioning"

	SSOStageMetadata    SSOStage = "metadata"
	SSOStageCertificate SSOStage = "certificate"
	SSOStageRequest     SSOStage = "request"
	SSOStageResponse    SSOStage = "response"
	SSOStageSignature   SSOStage = "signature"
	SSOStageConditions  SSOStage = "conditions"
	SSOStageReplay      SSOStage = "replay"
	SSOStageAttributes  SSOStage = "attributes"
)

func (s SSOStage) Valid() bool {
	switch s {
	case SSOStageDiscovery, SSOStageEndpoints, SSOStageJWKS, SSOStageAuthorization,
		SSOStageTokenExchange, SSOStageIDToken, SSOStageClaims, SSOStageMatching,
		SSOStageProvisioning, SSOStageMetadata, SSOStageCertificate, SSOStageRequest,
		SSOStageResponse, SSOStageSignature, SSOStageConditions, SSOStageReplay,
		SSOStageAttributes:
		return true
	default:
		return false
	}
}

type SSOError struct {
	Stage   SSOStage
	Message string
	Detail  string
	Subject string
	cause   error
}

func NewSSOError(stage SSOStage, message string) *SSOError {
	return &SSOError{Stage: stage, Message: message}
}

func SSOFailure(stage SSOStage, message string, cause error) *SSOError {
	failure := &SSOError{Stage: stage, Message: message, cause: cause}
	if cause != nil {
		failure.Detail = cause.Error()
	}

	return failure
}

func (e *SSOError) Error() string {
	if e.Detail == "" {
		return fmt.Sprintf("%s: %s", e.Stage, e.Message)
	}

	return fmt.Sprintf("%s: %s: %s", e.Stage, e.Message, e.Detail)
}

func (e *SSOError) Unwrap() error { return e.cause }

func AsSSOError(err error) (*SSOError, bool) {
	var failure *SSOError

	return failure, errors.As(err, &failure)
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
		failure := NewSSOError(
			SSOStageMatching,
			NormalizeEmail(email)+" signed in with your provider but is not a member of this workspace.",
		)
		failure.Subject = NormalizeEmail(email)

		return failure
	case MatchOutcomeNoAccount:
		failure := NewSSOError(
			SSOStageProvisioning,
			"There is no Norn account for "+NormalizeEmail(email)+
				", and just-in-time provisioning is turned off.",
		)
		failure.Subject = NormalizeEmail(email)

		return failure
	default:
		return nil
	}
}

type SSOPurpose string

const (
	SSOPurposeLogin SSOPurpose = "login"
	SSOPurposeTest  SSOPurpose = "test"
)

func (p SSOPurpose) Valid() bool {
	switch p {
	case SSOPurposeLogin, SSOPurposeTest:
		return true
	default:
		return false
	}
}

type SSOExchange struct {
	Protocol      SSOProtocol
	Purpose       SSOPurpose
	WorkspaceID   uuid.UUID
	WorkspaceSlug string
	ReturnTo      string
	Email         string
	Session       Session
	Token         string
	Provisioned   bool
}
