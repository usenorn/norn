package entity

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	RunnerAccessPrefix  = "nrs_"
	RunnerRefreshPrefix = "nrr_"
	RunnerTicketPrefix  = "nrt_"
	RunnerSecretBytes   = 32

	RunnerNameMaxLen     = 200
	RunnerHostnameMaxLen = 255
	RunnerPlatformMaxLen = 64
	RunnerVersionMaxLen  = 64

	RunnerAssertionVersion  = "v1"
	RunnerAssertionAudience = "norn-runner"
	RunnerNonceMinLen       = 16
	RunnerNonceMaxLen       = 128
)

var (
	ErrRunnerNotFound          = errors.New("runner not found")
	ErrRunnerRevoked           = errors.New("this runner has been revoked")
	ErrRunnerNameTaken         = errors.New("runner name already used by this agent")
	ErrRunnerEnrolmentNotAgent = errors.New("only an agent may enrol a runner")
	ErrRunnerKeyMalformed      = errors.New("runner device key is not an ed25519 public key")
	ErrRunnerCredentialInvalid = errors.New("runner credential is not valid")
	ErrRunnerAssertionForged   = errors.New("runner assertion was not signed by the device key")
	ErrRunnerAssertionStale    = errors.New("runner assertion is outside the permitted clock skew")
	ErrRunnerAssertionReplayed = errors.New("runner assertion nonce has already been spent")
	ErrRunnerAssertionMismatch = errors.New("runner assertion names a different runner")
)

type RunnerStatus string

const (
	RunnerStatusActive  RunnerStatus = "active"
	RunnerStatusRevoked RunnerStatus = "revoked"
)

func RunnerStatuses() []RunnerStatus {
	return []RunnerStatus{RunnerStatusActive, RunnerStatusRevoked}
}

func (s RunnerStatus) Valid() bool {
	return slices.Contains(RunnerStatuses(), s)
}

func (s RunnerStatus) CanTransitionTo(target RunnerStatus) bool {
	return s.Valid() && target.Valid() && s != target
}

type RunnerHost struct {
	Hostname string
	OS       string
	Arch     string
	Version  string
}

type Runner struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	AgentID     uuid.UUID
	AgentName   string
	Name        string
	Host        RunnerHost
	Authority   RequestedAuthority
	PublicKey   ed25519.PublicKey
	RefreshHash []byte
	Status      RunnerStatus
	EnrolledAt  time.Time
	LastSeenAt  *time.Time
	RevokedAt   *time.Time
	UpdatedAt   time.Time
}

func (r Runner) Revoked() bool {
	return r.Status == RunnerStatusRevoked
}

type RunnerAssertion struct {
	RunnerID  uuid.UUID
	Nonce     string
	IssuedAt  time.Time
	Audience  string
	Signature []byte
}

func (a RunnerAssertion) SigningPayload() []byte {
	return []byte(strings.Join([]string{
		RunnerAssertionVersion,
		a.RunnerID.String(),
		a.Nonce,
		a.IssuedAt.UTC().Format(time.RFC3339Nano),
		a.Audience,
	}, "\n"))
}

func (a RunnerAssertion) Fresh(now time.Time, skew time.Duration) bool {
	drift := now.Sub(a.IssuedAt)
	if drift < 0 {
		drift = -drift
	}

	return drift <= skew
}

func (a RunnerAssertion) Verify(key ed25519.PublicKey) error {
	if len(key) != ed25519.PublicKeySize {
		return ErrRunnerKeyMalformed
	}

	if !ed25519.Verify(key, a.SigningPayload(), a.Signature) {
		return ErrRunnerAssertionForged
	}

	return nil
}

func ParseRunnerPublicKey(encoded string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, ErrRunnerKeyMalformed
	}

	return raw, nil
}

func EncodeRunnerPublicKey(key ed25519.PublicKey) string {
	return base64.StdEncoding.EncodeToString(key)
}

func NewRunnerSecret(prefix string) (string, []byte, error) {
	raw := make([]byte, RunnerSecretBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate runner secret: %w", err)
	}

	secret := prefix + base64.RawURLEncoding.EncodeToString(raw)

	return secret, HashRunnerSecret(secret), nil
}

func HashRunnerSecret(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))

	return sum[:]
}

func LooksLikeRunnerToken(token string) bool {
	return strings.HasPrefix(token, RunnerAccessPrefix)
}

func runnerText(field, value string, max int) FieldError {
	trimmed := strings.TrimSpace(value)

	switch {
	case trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case utf8.RuneCountInString(trimmed) > max:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func ValidateRunnerName(field, name string) FieldError {
	return runnerText(field, name, RunnerNameMaxLen)
}

func ValidateRunnerHost(host RunnerHost) []FieldError {
	return []FieldError{
		runnerText("host.hostname", host.Hostname, RunnerHostnameMaxLen),
		runnerText("host.os", host.OS, RunnerPlatformMaxLen),
		runnerText("host.arch", host.Arch, RunnerPlatformMaxLen),
		runnerText("host.version", host.Version, RunnerVersionMaxLen),
	}
}

func ValidateRunnerNonce(field, nonce string) FieldError {
	trimmed := strings.TrimSpace(nonce)

	switch {
	case trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case len(trimmed) < RunnerNonceMinLen:
		return FieldError{Field: field, Code: ValidationCodeTooShort}
	case len(trimmed) > RunnerNonceMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func ValidateRunnerAudience(field, audience string) FieldError {
	if strings.TrimSpace(audience) != RunnerAssertionAudience {
		return FieldError{Field: field, Code: ValidationCodeUnsupportedValue}
	}

	return FieldError{}
}
