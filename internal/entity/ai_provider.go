package entity

import (
	"errors"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	AIProviderKeyMaxLen     = 512
	AIProviderKeyHintLen    = 4
	AIProviderModelMaxLen   = 128
	AIProviderBaseURLMaxLen = 2048
)

var (
	ErrAIProviderNotConfigured        = errors.New("this workspace has no AI provider configured")
	ErrAIProviderKeyRejected          = errors.New("the provider rejected this API key")
	ErrAIProviderModelUnavailable     = errors.New("this API key cannot use that model")
	ErrAIProviderQuotaExceeded        = errors.New("the provider account behind this API key is out of quota")
	ErrAIProviderRateLimited          = errors.New("the provider is rate limiting this API key")
	ErrAIProviderUnreachable          = errors.New("the provider could not be reached")
	ErrAIProviderDestinationRefused   = errors.New("this instance will not open a connection to that endpoint")
	ErrAIProviderEncryptionKeyMissing = errors.New("this instance has no encryption key, so an API key cannot be stored")
)

type AIProviderKind string

const AIProviderOpenAI AIProviderKind = "openai"

func (k AIProviderKind) Valid() bool {
	return k == AIProviderOpenAI
}

func (k AIProviderKind) Label() string {
	if k == AIProviderOpenAI {
		return "OpenAI"
	}

	return string(k)
}

type AIProviderFailure string

const (
	AIProviderFailureKeyRejected        AIProviderFailure = "key_rejected"
	AIProviderFailureModelUnavailable   AIProviderFailure = "model_unavailable"
	AIProviderFailureQuotaExceeded      AIProviderFailure = "quota_exceeded"
	AIProviderFailureRateLimited        AIProviderFailure = "rate_limited"
	AIProviderFailureUnreachable        AIProviderFailure = "unreachable"
	AIProviderFailureDestinationRefused AIProviderFailure = "destination_refused"
)

var aiProviderFailures = []struct {
	failure AIProviderFailure
	err     error
}{
	{AIProviderFailureKeyRejected, ErrAIProviderKeyRejected},
	{AIProviderFailureModelUnavailable, ErrAIProviderModelUnavailable},
	{AIProviderFailureQuotaExceeded, ErrAIProviderQuotaExceeded},
	{AIProviderFailureRateLimited, ErrAIProviderRateLimited},
	{AIProviderFailureUnreachable, ErrAIProviderUnreachable},
	{AIProviderFailureDestinationRefused, ErrAIProviderDestinationRefused},
}

func AIProviderFailureOf(err error) (AIProviderFailure, bool) {
	for _, known := range aiProviderFailures {
		if errors.Is(err, known.err) {
			return known.failure, true
		}
	}

	return "", false
}

type AIProviderStatus string

const (
	AIProviderUnverified AIProviderStatus = "unverified"
	AIProviderVerified   AIProviderStatus = "verified"
	AIProviderFailed     AIProviderStatus = "failed"
)

type AIProviderEndpoint struct {
	BaseURL             string
	AllowPrivateAddress bool
}

type AIProviderConnection struct {
	WorkspaceID  uuid.UUID
	Provider     AIProviderKind
	Endpoint     AIProviderEndpoint
	KeyHint      string
	DefaultModel string
	VerifiedAt   *time.Time
	FailedAt     *time.Time
	Failure      AIProviderFailure
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (c AIProviderConnection) Status() AIProviderStatus {
	switch {
	case c.FailedAt != nil:
		return AIProviderFailed
	case c.VerifiedAt != nil:
		return AIProviderVerified
	default:
		return AIProviderUnverified
	}
}

type AIProviderInput struct {
	Provider     AIProviderKind
	Endpoint     AIProviderEndpoint
	APIKey       string
	DefaultModel string
}

func (in AIProviderInput) Normalized() AIProviderInput {
	return AIProviderInput{
		Provider: AIProviderKind(strings.TrimSpace(string(in.Provider))),
		Endpoint: AIProviderEndpoint{
			BaseURL:             strings.TrimRight(strings.TrimSpace(in.Endpoint.BaseURL), "/"),
			AllowPrivateAddress: in.Endpoint.AllowPrivateAddress,
		},
		APIKey:       strings.TrimSpace(in.APIKey),
		DefaultModel: strings.TrimSpace(in.DefaultModel),
	}
}

func (in AIProviderInput) Validate(configured bool) error {
	var fields []FieldError

	if !in.Provider.Valid() {
		fields = append(fields, FieldError{Field: "provider", Code: ValidationCodeUnsupportedValue})
	}

	switch {
	case in.APIKey == "" && !configured:
		fields = append(fields, FieldError{Field: "apiKey", Code: ValidationCodeRequired})
	case len(in.APIKey) > AIProviderKeyMaxLen:
		fields = append(fields, FieldError{Field: "apiKey", Code: ValidationCodeTooLong})
	}

	if utf8.RuneCountInString(in.DefaultModel) > AIProviderModelMaxLen {
		fields = append(fields, FieldError{Field: "defaultModel", Code: ValidationCodeTooLong})
	}

	if field := validateAIProviderBaseURL(in.Endpoint.BaseURL); field.Field != "" {
		fields = append(fields, field)
	}

	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}

	return nil
}

func validateAIProviderBaseURL(raw string) FieldError {
	if raw == "" {
		return FieldError{}
	}

	if len(raw) > AIProviderBaseURLMaxLen {
		return FieldError{Field: "baseUrl", Code: ValidationCodeTooLong}
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") ||
		parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return FieldError{Field: "baseUrl", Code: ValidationCodeMalformed}
	}

	return FieldError{}
}

func AIProviderPrivateAddressRefused() error {
	return ValidationError{
		Fields: []FieldError{{Field: "allowPrivateAddress", Code: ValidationCodeUnsupportedValue}},
	}
}

func AIProviderKeyRequired() error {
	return ValidationError{Fields: []FieldError{{Field: "apiKey", Code: ValidationCodeRequired}}}
}

func AIProviderModelRequired() error {
	return ValidationError{Fields: []FieldError{{Field: "defaultModel", Code: ValidationCodeRequired}}}
}

func AIProviderKeyHint(key string) string {
	trimmed := strings.TrimSpace(key)

	if utf8.RuneCountInString(trimmed) <= AIProviderKeyHintLen {
		return ""
	}

	runes := []rune(trimmed)

	return string(runes[len(runes)-AIProviderKeyHintLen:])
}
