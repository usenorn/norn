package entity_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

func TestAIProviderInputNamesEveryFieldItRefuses(t *testing.T) {
	cases := []struct {
		name       string
		input      entity.AIProviderInput
		configured bool
		want       []entity.FieldError
	}{
		{
			name:  "a first save must carry a key",
			input: entity.AIProviderInput{Provider: entity.AIProviderOpenAI},
			want:  []entity.FieldError{{Field: "apiKey", Code: entity.ValidationCodeRequired}},
		},
		{
			name:       "a later save may keep the stored key",
			input:      entity.AIProviderInput{Provider: entity.AIProviderOpenAI, DefaultModel: "gpt-6-luna"},
			configured: true,
		},
		{
			name:  "an unknown provider is refused",
			input: entity.AIProviderInput{Provider: "mistral", APIKey: "sk-test"},
			want:  []entity.FieldError{{Field: "provider", Code: entity.ValidationCodeUnsupportedValue}},
		},
		{
			name: "a key past the limit is refused",
			input: entity.AIProviderInput{
				Provider: entity.AIProviderOpenAI,
				APIKey:   strings.Repeat("k", entity.AIProviderKeyMaxLen+1),
			},
			want: []entity.FieldError{{Field: "apiKey", Code: entity.ValidationCodeTooLong}},
		},
		{
			name: "an endpoint that is not an http address is refused",
			input: entity.AIProviderInput{
				Provider: entity.AIProviderOpenAI,
				Endpoint: entity.AIProviderEndpoint{BaseURL: "file:///etc/passwd"},
				APIKey:   "sk-test",
			},
			want: []entity.FieldError{{Field: "baseUrl", Code: entity.ValidationCodeMalformed}},
		},
		{
			name: "an endpoint carrying credentials is refused",
			input: entity.AIProviderInput{
				Provider: entity.AIProviderOpenAI,
				Endpoint: entity.AIProviderEndpoint{BaseURL: "https://user:pass@gateway.example.com/v1"},
				APIKey:   "sk-test",
			},
			want: []entity.FieldError{{Field: "baseUrl", Code: entity.ValidationCodeMalformed}},
		},
		{
			name: "a model past the limit is refused",
			input: entity.AIProviderInput{
				Provider:     entity.AIProviderOpenAI,
				APIKey:       "sk-test",
				DefaultModel: strings.Repeat("m", entity.AIProviderModelMaxLen+1),
			},
			want: []entity.FieldError{{Field: "defaultModel", Code: entity.ValidationCodeTooLong}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.Validate(tc.configured)

			if len(tc.want) == 0 {
				if err != nil {
					t.Fatalf("expected the input to pass, got %v", err)
				}

				return
			}

			var validation entity.ValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("expected a validation error, got %v", err)
			}

			if fmt.Sprint(validation.Fields) != fmt.Sprint(tc.want) {
				t.Fatalf("expected fields %v, got %v", tc.want, validation.Fields)
			}
		})
	}
}

func TestAIProviderKeyHintKeepsOnlyTheLastCharacters(t *testing.T) {
	cases := map[string]string{
		"  sk-proj-abcdef1234  ": "1234",
		"sk12":                   "",
		"":                       "",
	}

	for key, want := range cases {
		if got := entity.AIProviderKeyHint(key); got != want {
			t.Errorf("AIProviderKeyHint(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestAIProviderStatusPrefersTheLatestFailure(t *testing.T) {
	now := time.Now()

	cases := []struct {
		name       string
		connection entity.AIProviderConnection
		want       entity.AIProviderStatus
	}{
		{"never tested", entity.AIProviderConnection{}, entity.AIProviderUnverified},
		{"tested and passed", entity.AIProviderConnection{VerifiedAt: &now}, entity.AIProviderVerified},
		{"passed once, failed since", entity.AIProviderConnection{VerifiedAt: &now, FailedAt: &now}, entity.AIProviderFailed},
	}

	for _, tc := range cases {
		if got := tc.connection.Status(); got != tc.want {
			t.Errorf("%s: status %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestAIProviderFailureOfRecognisesWrappedProviderErrors(t *testing.T) {
	failure, ok := entity.AIProviderFailureOf(fmt.Errorf("probe: %w", entity.ErrAIProviderModelUnavailable))
	if !ok || failure != entity.AIProviderFailureModelUnavailable {
		t.Fatalf("expected model_unavailable, got %q (recognised %v)", failure, ok)
	}

	if _, ok := entity.AIProviderFailureOf(errors.New("connection reset")); ok {
		t.Fatal("an error that is not a provider verdict must not be recorded as one")
	}
}
