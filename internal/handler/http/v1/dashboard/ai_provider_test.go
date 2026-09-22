package dashboard_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	aiprovidersvc "github.com/usenorn/norn/internal/service/aiprovider"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

type aiProviderHarness struct {
	providers *aiprovidersvc.MockAIProviders
	routes    http.Handler
	workspace uuid.UUID
}

func newAIProviderHarness(t *testing.T) *aiProviderHarness {
	t.Helper()

	ctrl := gomock.NewController(t)
	h := &aiProviderHarness{providers: aiprovidersvc.NewMockAIProviders(ctrl), workspace: uuid.New()}
	h.routes = newEdge(ctrl, edgeServices{aiProviders: h.providers})

	return h
}

func (h *aiProviderHarness) call(t *testing.T, method, suffix string, payload any) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			t.Fatalf("encode request: %v", err)
		}
	}

	request := httptest.NewRequest(method, "/workspaces/"+h.workspace.String()+"/ai-provider"+suffix, &body)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	h.routes.ServeHTTP(recorder, request)

	return recorder
}

func TestTheStoredKeyNeverLeavesTheBackend(t *testing.T) {
	h := newAIProviderHarness(t)
	verified := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)

	h.providers.EXPECT().Get(gomock.Any(), h.workspace).Return(entity.AIProviderConnection{
		WorkspaceID:  h.workspace,
		Provider:     entity.AIProviderOpenAI,
		KeyHint:      "1234",
		DefaultModel: "gpt-6-luna",
		VerifiedAt:   &verified,
	}, nil)

	recorder := h.call(t, http.MethodGet, "", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", recorder.Code, recorder.Body)
	}

	got := decodeBody[api.WorkspaceAiProvider](t, recorder)
	if got.Status != api.AiProviderStatusVerified || got.KeyHint != "1234" || got.DefaultModel != "gpt-6-luna" {
		t.Errorf("provider = %+v, want verified with hint 1234 and model gpt-4o", got)
	}

	if strings.Contains(strings.ToLower(recorder.Body.String()), "apikey") {
		t.Errorf("the response carries a key field: %s", recorder.Body)
	}
}

func TestAWorkspaceWithoutAProviderReadsAsNotFound(t *testing.T) {
	h := newAIProviderHarness(t)
	h.providers.EXPECT().Get(gomock.Any(), h.workspace).Return(entity.AIProviderConnection{}, entity.ErrAIProviderNotConfigured)

	if recorder := h.call(t, http.MethodGet, "", nil); recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 so the screen can offer a first save", recorder.Code)
	}
}

func TestAProviderRefusalCarriesACodeTheScreenCanBranchOn(t *testing.T) {
	cases := []struct {
		err  error
		want api.AiProviderFailure
	}{
		{fmt.Errorf("%w: Incorrect API key provided", entity.ErrAIProviderKeyRejected), api.AiProviderKeyRejected},
		{entity.ErrAIProviderModelUnavailable, api.AiProviderModelUnavailable},
		{entity.ErrAIProviderQuotaExceeded, api.AiProviderQuotaExceeded},
		{entity.ErrAIProviderUnreachable, api.AiProviderUnreachable},
	}

	for _, tc := range cases {
		t.Run(string(tc.want), func(t *testing.T) {
			h := newAIProviderHarness(t)
			h.providers.EXPECT().Test(gomock.Any(), h.workspace).Return(entity.AIProviderConnection{}, tc.err)

			recorder := h.call(t, http.MethodPost, "/test", nil)
			if recorder.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422", recorder.Code)
			}

			problem := decodeBody[api.AiProviderRefusedProblem](t, recorder)
			if problem.Code == nil || *problem.Code != tc.want {
				t.Fatalf("code = %v, want %q", problem.Code, tc.want)
			}
		})
	}
}

func TestSavingPassesTheFormThroughAndLeavesAnOmittedKeyEmpty(t *testing.T) {
	h := newAIProviderHarness(t)

	h.providers.EXPECT().
		Configure(gomock.Any(), h.workspace, entity.AIProviderInput{
			Provider:     entity.AIProviderOpenAI,
			DefaultModel: "gpt-6-sol",
		}).
		Return(entity.AIProviderConnection{WorkspaceID: h.workspace, Provider: entity.AIProviderOpenAI}, nil)

	recorder := h.call(t, http.MethodPut, "", map[string]string{"provider": "openai", "defaultModel": "gpt-6-sol"})
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", recorder.Code, recorder.Body)
	}
}

func TestAWorkspaceEndpointReachesTheServiceAsSent(t *testing.T) {
	h := newAIProviderHarness(t)

	h.providers.EXPECT().
		Configure(gomock.Any(), h.workspace, entity.AIProviderInput{
			Provider: entity.AIProviderOpenAI,
			Endpoint: entity.AIProviderEndpoint{BaseURL: "http://10.0.4.12:8000/v1", AllowPrivateAddress: true},
			APIKey:   "sk-gateway",
		}).
		Return(entity.AIProviderConnection{
			WorkspaceID: h.workspace,
			Provider:    entity.AIProviderOpenAI,
			Endpoint:    entity.AIProviderEndpoint{BaseURL: "http://10.0.4.12:8000/v1", AllowPrivateAddress: true},
		}, nil)

	recorder := h.call(t, http.MethodPut, "", map[string]any{
		"provider":            "openai",
		"baseUrl":             "http://10.0.4.12:8000/v1",
		"allowPrivateAddress": true,
		"apiKey":              "sk-gateway",
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", recorder.Code, recorder.Body)
	}

	got := decodeBody[api.WorkspaceAiProvider](t, recorder)
	if got.BaseUrl != "http://10.0.4.12:8000/v1" || !got.AllowPrivateAddress {
		t.Errorf("provider = %+v, want the endpoint read back", got)
	}
}

func TestAMissingEncryptionKeyIsAnInstanceProblemNotAWorkspaceOne(t *testing.T) {
	h := newAIProviderHarness(t)
	h.providers.EXPECT().
		Configure(gomock.Any(), h.workspace, gomock.Any()).
		Return(entity.AIProviderConnection{}, entity.ErrAIProviderEncryptionKeyMissing)

	recorder := h.call(t, http.MethodPut, "", map[string]string{"provider": "openai", "apiKey": "sk-test"})
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", recorder.Code)
	}

	problem := decodeBody[api.AiProviderSealingUnavailableProblem](t, recorder)
	if problem.Code != api.AiProviderSealingUnavailableProblemCodeAiProviderSealingUnavailable {
		t.Fatalf("code = %q, want ai_provider_sealing_unavailable", problem.Code)
	}
}
