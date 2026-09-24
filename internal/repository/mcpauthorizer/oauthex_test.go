package mcpauthorizer_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/toolingclient"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/repository/mcpauthorizer"
)

const redirectURI = "https://norn.test/v1/agent-mcp/oauth/callback"

type provider struct {
	server     *httptest.Server
	registered []string
	exchanged  url.Values
	refused    bool
}

func answer(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func newProvider(t *testing.T) *provider {
	t.Helper()

	p := &provider{}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /mcp", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("WWW-Authenticate",
			`Bearer resource_metadata="`+p.server.URL+`/.well-known/oauth-protected-resource/mcp", scope="issues:read"`)
		w.WriteHeader(http.StatusUnauthorized)
	})
	mux.HandleFunc("GET /.well-known/oauth-protected-resource/mcp", func(w http.ResponseWriter, _ *http.Request) {
		answer(w, http.StatusOK, map[string]any{
			"resource":              p.server.URL + "/mcp",
			"authorization_servers": []string{p.server.URL + "/auth"},
		})
	})
	mux.HandleFunc("GET /.well-known/oauth-authorization-server/auth", func(w http.ResponseWriter, _ *http.Request) {
		answer(w, http.StatusOK, map[string]any{
			"issuer":                           p.server.URL + "/auth",
			"authorization_endpoint":           p.server.URL + "/auth/authorize",
			"token_endpoint":                   p.server.URL + "/auth/token",
			"registration_endpoint":            p.server.URL + "/auth/register",
			"response_types_supported":         []string{"code"},
			"code_challenge_methods_supported": []string{"S256"},
		})
	})
	mux.HandleFunc("POST /auth/register", func(w http.ResponseWriter, r *http.Request) {
		var metadata struct {
			RedirectURIs []string `json:"redirect_uris"`
		}
		_ = json.NewDecoder(r.Body).Decode(&metadata)
		p.registered = metadata.RedirectURIs

		answer(w, http.StatusCreated, map[string]any{"client_id": "client-norn", "redirect_uris": metadata.RedirectURIs})
	})
	mux.HandleFunc("POST /auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		p.exchanged = r.PostForm

		if p.refused {
			answer(w, http.StatusBadRequest, map[string]string{"error": "invalid_grant"})

			return
		}

		answer(w, http.StatusOK, map[string]any{
			"access_token": "at-1", "refresh_token": "rt-1", "token_type": "Bearer", "expires_in": 3600,
		})
	})

	p.server = httptest.NewServer(mux)
	t.Cleanup(p.server.Close)

	return p
}

func authorizer(t *testing.T) repository.MCPAuthorizer {
	t.Helper()

	client, err := toolingclient.New(config.AgentTooling{
		GitHubEndpoint:      "https://api.github.test",
		GitHubRawEndpoint:   "https://raw.github.test",
		RegistryEndpoint:    "https://registry.test",
		RequestTimeout:      5 * time.Second,
		DialTimeout:         time.Second,
		MaxResponseSize:     8 << 20,
		AllowedDestinations: []string{"127.0.0.1/32"},
	})
	if err != nil {
		t.Fatalf("toolingclient.New: %v", err)
	}

	return mcpauthorizer.New(client)
}

func TestSigningInFollowsTheServersOwnDirections(t *testing.T) {
	p := newProvider(t)
	oauth := authorizer(t)

	discovered, err := oauth.Discover(context.Background(), p.server.URL+"/mcp", false)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if discovered.Issuer != p.server.URL+"/auth" || discovered.Resource != p.server.URL+"/mcp" ||
		!slices.Equal(discovered.Scopes, []string{"issues:read"}) {
		t.Fatalf("discovered = %+v", discovered)
	}

	client, err := oauth.Register(context.Background(), discovered, redirectURI, false)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if client.ID != "client-norn" || !slices.Equal(p.registered, []string{redirectURI}) {
		t.Errorf("client %+v registered for %v", client, p.registered)
	}

	address, err := url.Parse(oauth.AuthorizationURL(discovered, client, redirectURI, "state-1", "verifier-that-is-long-enough-for-pkce-0123456789"))
	if err != nil {
		t.Fatalf("parse authorization url: %v", err)
	}

	query := address.Query()
	if query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") == "" ||
		query.Get("resource") != p.server.URL+"/mcp" || query.Get("state") != "state-1" {
		t.Errorf("authorization url = %s; PKCE and the resource indicator are what the MCP spec requires", address)
	}

	tokens, err := oauth.Exchange(context.Background(), entity.AgentMCPOAuthState{
		Verifier:      "verifier-that-is-long-enough-for-pkce-0123456789",
		TokenEndpoint: discovered.TokenEndpoint,
		Resource:      discovered.Resource,
		Client:        client,
	}, "code-1", redirectURI, false)
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}

	if tokens.AccessToken != "at-1" || tokens.RefreshToken != "rt-1" || tokens.ExpiresAt == nil {
		t.Errorf("tokens = %+v", tokens)
	}

	if p.exchanged.Get("code_verifier") == "" || p.exchanged.Get("resource") != p.server.URL+"/mcp" {
		t.Errorf("token request = %v", p.exchanged)
	}
}

func TestARevokedRefreshTokenIsARefusalNotAnOutage(t *testing.T) {
	p := newProvider(t)
	p.refused = true

	_, err := authorizer(t).Refresh(context.Background(), entity.AgentMCPConnection{
		TokenEndpoint: p.server.URL + "/auth/token",
		ClientID:      "client-norn",
	}, entity.AgentMCPTokens{RefreshToken: "rt-revoked"}, false)
	if !errors.Is(err, entity.ErrAgentMCPOAuthRefused) {
		t.Fatalf("err = %v, want ErrAgentMCPOAuthRefused so the connection is marked for a new sign-in", err)
	}
}

func TestAServerOnAPrivateAddressIsRefusedUnlessAllowed(t *testing.T) {
	client, err := toolingclient.New(config.AgentTooling{
		GitHubEndpoint:    "https://api.github.test",
		GitHubRawEndpoint: "https://raw.github.test",
		RegistryEndpoint:  "https://registry.test",
		RequestTimeout:    5 * time.Second,
		DialTimeout:       time.Second,
		MaxResponseSize:   8 << 20,
	})
	if err != nil {
		t.Fatalf("toolingclient.New: %v", err)
	}

	p := newProvider(t)

	_, err = mcpauthorizer.New(client).Discover(context.Background(), p.server.URL+"/mcp", false)
	if !errors.Is(err, entity.ErrAgentMCPDestinationRefused) {
		t.Fatalf("err = %v; a cloud workspace must not be able to make Norn call into its own network", err)
	}
}
