package openai_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/openai"
)

const loopback = "127.0.0.1/32"

func build(t *testing.T, endpoint string, allowed ...string) *openai.Client {
	t.Helper()

	client, err := openai.New(config.OpenAI{
		Endpoint:            endpoint,
		RequestTimeout:      5 * time.Second,
		DialTimeout:         time.Second,
		MaxResponseSize:     1 << 20,
		AllowedDestinations: allowed,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	return client
}

func answering(t *testing.T, handler http.HandlerFunc) *openai.Client {
	t.Helper()

	provider := httptest.NewServer(handler)
	t.Cleanup(provider.Close)

	return build(t, provider.URL+"/", loopback)
}

func TestTextModelsKeepsEveryModelThatAnswersText(t *testing.T) {
	var offered, path string

	client := answering(t, func(writer http.ResponseWriter, request *http.Request) {
		offered = request.Header.Get("Authorization")
		path = request.URL.Path

		_, _ = writer.Write([]byte(`{"data":[
			{"id":"gpt-6-astra"},{"id":"gpt-6-luna"},{"id":"gpt-6-sol"},{"id":"gpt-oss-120b"},
			{"id":"text-embedding-3-small"},{"id":"whisper-1"},{"id":"gpt-realtime"},
			{"id":"gpt-image-2.5"},{"id":"omni-moderation-latest"},{"id":"gpt-4o-mini-tts"}
		]}`))
	})

	models, err := client.TextModels(context.Background(), entity.AIProviderEndpoint{}, "sk-test")
	if err != nil {
		t.Fatalf("TextModels: %v", err)
	}

	want := []string{"gpt-6-astra", "gpt-6-luna", "gpt-6-sol", "gpt-oss-120b"}
	if !slices.Equal(models, want) {
		t.Errorf(
			"models = %v, want %v: a new text model must appear without a code change, and a "+
				"model that cannot answer a text request must not be offered as the default",
			models, want,
		)
	}

	if offered != "Bearer sk-test" {
		t.Errorf("authorization = %q, want the key as a bearer token", offered)
	}

	if path != "/models" {
		t.Errorf("path = %q, want /models under the configured endpoint without a doubled slash", path)
	}
}

func TestProbeSendsOneSmallResponsesRequestForTheChosenModel(t *testing.T) {
	var sent struct {
		Model           string `json:"model"`
		Input           string `json:"input"`
		MaxOutputTokens int    `json:"max_output_tokens"`
	}

	client := answering(t, func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/responses" {
			t.Errorf("request = %s %s, want POST /responses", request.Method, request.URL.Path)
		}

		if err := json.NewDecoder(request.Body).Decode(&sent); err != nil {
			t.Errorf("decode probe: %v", err)
		}

		_, _ = writer.Write([]byte(`{"id":"resp_1","status":"completed"}`))
	})

	if err := client.Probe(context.Background(), entity.AIProviderEndpoint{}, "sk-test", "gpt-6-luna"); err != nil {
		t.Fatalf("Probe: %v", err)
	}

	if sent.Model != "gpt-6-luna" || sent.Input == "" {
		t.Errorf("sent model %q input %q, want the workspace's default model and a prompt", sent.Model, sent.Input)
	}

	if sent.MaxOutputTokens < 16 || sent.MaxOutputTokens > 64 {
		t.Errorf(
			"max_output_tokens = %d; the Responses API refuses anything under 16, and a test must "+
				"cost next to nothing on the workspace's own bill",
			sent.MaxOutputTokens,
		)
	}
}

func TestAWorkspaceEndpointReplacesTheDefaultOne(t *testing.T) {
	var reached bool

	gateway := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		reached = request.URL.Path == "/openai/v1/models"

		_, _ = writer.Write([]byte(`{"data":[{"id":"llama-4-scout"}]}`))
	}))
	t.Cleanup(gateway.Close)

	client := build(t, "https://api.openai.invalid/v1", loopback)

	models, err := client.TextModels(
		context.Background(),
		entity.AIProviderEndpoint{BaseURL: gateway.URL + "/openai/v1"},
		"sk-test",
	)
	if err != nil {
		t.Fatalf("TextModels: %v", err)
	}

	if !reached || !slices.Equal(models, []string{"llama-4-scout"}) {
		t.Fatalf("reached %v, models %v; the workspace's own endpoint must be the one asked", reached, models)
	}
}

func TestAnEndpointOnARefusedAddressIsNeverDialled(t *testing.T) {
	var reached bool

	internal := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		reached = true
	}))
	t.Cleanup(internal.Close)

	client := build(t, "https://api.openai.invalid/v1")

	err := client.Probe(
		context.Background(),
		entity.AIProviderEndpoint{BaseURL: internal.URL, AllowPrivateAddress: true},
		"sk-test",
		"gpt-6-luna",
	)
	if !errors.Is(err, entity.ErrAIProviderDestinationRefused) {
		t.Fatalf("err = %v, want ErrAIProviderDestinationRefused", err)
	}

	if reached {
		t.Fatal("the key was sent to loopback, which a workspace setting alone must never reach")
	}
}

func TestProviderRefusalsBecomeTheVerdictAnAdministratorCanActOn(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"a wrong key", http.StatusUnauthorized, `{"error":{"message":"Incorrect API key provided","code":"invalid_api_key"}}`, entity.ErrAIProviderKeyRejected},
		{"a key without permission", http.StatusForbidden, `{"error":{"message":"Missing scopes: model.request"}}`, entity.ErrAIProviderKeyRejected},
		{"a model the key cannot use", http.StatusNotFound, `{"error":{"message":"The model does not exist","code":"model_not_found"}}`, entity.ErrAIProviderModelUnavailable},
		{"an account out of credit", http.StatusTooManyRequests, `{"error":{"message":"You exceeded your current quota","type":"insufficient_quota","code":"insufficient_quota"}}`, entity.ErrAIProviderQuotaExceeded},
		{"a rate limit", http.StatusTooManyRequests, `{"error":{"message":"Rate limit reached","code":"rate_limit_exceeded"}}`, entity.ErrAIProviderRateLimited},
		{"a provider outage", http.StatusServiceUnavailable, `upstream connect error`, entity.ErrAIProviderUnreachable},
		{"a redirect", http.StatusFound, ``, entity.ErrAIProviderUnreachable},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := answering(t, func(writer http.ResponseWriter, _ *http.Request) {
				if tc.status == http.StatusFound {
					writer.Header().Set("Location", "http://169.254.169.254/latest/meta-data")
				}

				writer.WriteHeader(tc.status)
				_, _ = writer.Write([]byte(tc.body))
			})

			err := client.Probe(context.Background(), entity.AIProviderEndpoint{}, "sk-test", "gpt-6-luna")
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestAnOversizedAnswerIsRefusedRatherThanTruncated(t *testing.T) {
	client := answering(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"data":[{"id":"` + strings.Repeat("x", 2<<20) + `"}]}`))
	})

	_, err := client.TextModels(context.Background(), entity.AIProviderEndpoint{}, "sk-test")
	if !errors.Is(err, openai.ErrResponseTooLarge) || !errors.Is(err, entity.ErrAIProviderUnreachable) {
		t.Fatalf("err = %v, want a named size refusal reported as the provider being unusable", err)
	}
}
