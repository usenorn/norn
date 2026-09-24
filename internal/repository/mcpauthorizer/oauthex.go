package mcpauthorizer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"golang.org/x/oauth2"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/outbound"
	"github.com/usenorn/norn/internal/pkg/toolingclient"
	"github.com/usenorn/norn/internal/repository"
)

const (
	protectedResourcePath   = "/.well-known/oauth-protected-resource"
	authorizationServerPath = "/.well-known/oauth-authorization-server"
	openIDConfigurationPath = "/.well-known/openid-configuration"
	legacyAuthorizePath     = "/authorize"
	legacyTokenPath         = "/token"
	legacyRegisterPath      = "/register"
	resourceParameter       = "resource"
	resourceMetadataParam   = "resource_metadata"
	scopeParam              = "scope"
	bearerScheme            = "bearer"
	authenticateHeader      = "WWW-Authenticate"
	probeAccept             = "application/json, text/event-stream"
	clientName              = "Norn"
	publicClientAuthMethod  = "none"
	authorizationCodeGrant  = "authorization_code"
	refreshTokenGrant       = "refresh_token"
	codeResponseType        = "code"
)

type authorizer struct {
	client *toolingclient.Client
}

func New(client *toolingclient.Client) repository.MCPAuthorizer {
	return &authorizer{client: client}
}

func origin(parsed *url.URL) string {
	return parsed.Scheme + "://" + parsed.Host
}

func (a *authorizer) challenge(
	ctx context.Context,
	serverURL string,
	allowPrivate bool,
) (string, []string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, serverURL, nil)
	if err != nil {
		return "", nil, entity.ErrAgentMCPOAuthUnsupported
	}

	request.Header.Set("Accept", probeAccept)

	response, err := a.client.HTTP(allowPrivate).Do(request)
	if err != nil {
		if errors.Is(err, outbound.ErrDestinationRefused) {
			return "", nil, entity.ErrAgentMCPDestinationRefused
		}

		return "", nil, fmt.Errorf("%w: %v", entity.ErrAgentMCPServerUnreachable, err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusUnauthorized {
		return "", nil, nil
	}

	challenges, err := oauthex.ParseWWWAuthenticate(response.Header.Values(authenticateHeader))
	if err != nil {
		return "", nil, nil
	}

	for _, challenge := range challenges {
		if challenge.Scheme != bearerScheme {
			continue
		}

		var scopes []string
		if scope := challenge.Params[scopeParam]; scope != "" {
			scopes = strings.Fields(scope)
		}

		return challenge.Params[resourceMetadataParam], scopes, nil
	}

	return "", nil, nil
}

func (a *authorizer) protectedResource(
	ctx context.Context,
	serverURL *url.URL,
	advertised string,
	allowPrivate bool,
) (*oauthex.ProtectedResourceMetadata, error) {
	candidates := []string{}
	if advertised != "" {
		candidates = append(candidates, advertised)
	}

	if path := strings.TrimRight(serverURL.Path, "/"); path != "" {
		candidates = append(candidates, origin(serverURL)+protectedResourcePath+path)
	}

	candidates = append(candidates, origin(serverURL)+protectedResourcePath)

	for _, candidate := range candidates {
		body, err := a.client.Get(ctx, candidate, http.Header{"Accept": {"application/json"}}, allowPrivate)
		if err != nil {
			if errors.Is(err, toolingclient.ErrNotFound) || errors.Is(err, toolingclient.ErrUnexpected) ||
				errors.Is(err, toolingclient.ErrUnauthorized) {
				continue
			}

			return nil, translate(err)
		}

		var metadata oauthex.ProtectedResourceMetadata
		if err := json.Unmarshal(body, &metadata); err != nil || len(metadata.AuthorizationServers) == 0 {
			continue
		}

		if !covers(metadata.Resource, serverURL.String()) {
			continue
		}

		return &metadata, nil
	}

	return nil, nil
}

func covers(resource, serverURL string) bool {
	if resource == "" {
		return false
	}

	trimmed := strings.TrimRight(resource, "/")

	return serverURL == resource || strings.TrimRight(serverURL, "/") == trimmed ||
		strings.HasPrefix(serverURL, trimmed+"/")
}

func metadataCandidates(issuer *url.URL) []string {
	path := strings.TrimRight(issuer.Path, "/")
	if path == "" {
		return []string{
			origin(issuer) + authorizationServerPath,
			origin(issuer) + openIDConfigurationPath,
		}
	}

	return []string{
		origin(issuer) + authorizationServerPath + path,
		origin(issuer) + openIDConfigurationPath + path,
		origin(issuer) + path + openIDConfigurationPath,
	}
}

func (a *authorizer) Discover(
	ctx context.Context,
	serverURL string,
	allowPrivate bool,
) (entity.AgentMCPAuthServer, error) {
	parsed, err := url.Parse(serverURL)
	if err != nil || parsed.Host == "" {
		return entity.AgentMCPAuthServer{}, entity.ErrAgentMCPOAuthUnsupported
	}

	advertised, scopes, err := a.challenge(ctx, serverURL, allowPrivate)
	if err != nil {
		return entity.AgentMCPAuthServer{}, err
	}

	resource, err := a.protectedResource(ctx, parsed, advertised, allowPrivate)
	if err != nil {
		return entity.AgentMCPAuthServer{}, err
	}

	issuer := origin(parsed)
	server := entity.AgentMCPAuthServer{Resource: serverURL, Scopes: scopes}

	if resource != nil {
		issuer = resource.AuthorizationServers[0]
		server.Resource = resource.Resource

		if len(server.Scopes) == 0 {
			server.Scopes = resource.ScopesSupported
		}
	}

	issuerURL, err := url.Parse(issuer)
	if err != nil || issuerURL.Host == "" {
		return entity.AgentMCPAuthServer{}, entity.ErrAgentMCPOAuthUnsupported
	}

	for _, candidate := range metadataCandidates(issuerURL) {
		metadata, err := oauthex.GetAuthServerMeta(ctx, candidate, issuer, a.client.HTTP(allowPrivate))
		if err != nil {
			return entity.AgentMCPAuthServer{}, fmt.Errorf("%w: %v", entity.ErrAgentMCPOAuthUnsupported, err)
		}

		if metadata == nil {
			continue
		}

		server.Issuer = metadata.Issuer
		server.AuthorizationEndpoint = metadata.AuthorizationEndpoint
		server.TokenEndpoint = metadata.TokenEndpoint
		server.RegistrationEndpoint = metadata.RegistrationEndpoint

		if len(server.Scopes) == 0 {
			server.Scopes = metadata.ScopesSupported
		}

		return server, nil
	}

	if resource != nil {
		return entity.AgentMCPAuthServer{}, entity.ErrAgentMCPOAuthUnsupported
	}

	server.Issuer = issuer
	server.AuthorizationEndpoint = issuer + legacyAuthorizePath
	server.TokenEndpoint = issuer + legacyTokenPath
	server.RegistrationEndpoint = issuer + legacyRegisterPath

	return server, nil
}

func (a *authorizer) Register(
	ctx context.Context,
	server entity.AgentMCPAuthServer,
	redirectURI string,
	allowPrivate bool,
) (entity.AgentMCPClient, error) {
	if server.RegistrationEndpoint == "" {
		return entity.AgentMCPClient{}, entity.ErrAgentMCPOAuthUnsupported
	}

	registered, err := oauthex.RegisterClient(ctx, server.RegistrationEndpoint, &oauthex.ClientRegistrationMetadata{
		RedirectURIs:            []string{redirectURI},
		TokenEndpointAuthMethod: publicClientAuthMethod,
		GrantTypes:              []string{authorizationCodeGrant, refreshTokenGrant},
		ResponseTypes:           []string{codeResponseType},
		ClientName:              clientName,
		Scope:                   strings.Join(server.Scopes, " "),
	}, a.client.HTTP(allowPrivate))
	if err != nil {
		return entity.AgentMCPClient{}, fmt.Errorf("%w: %v", entity.ErrAgentMCPOAuthUnsupported, err)
	}

	return entity.AgentMCPClient{ID: registered.ClientID, Secret: registered.ClientSecret}, nil
}

func config(
	client entity.AgentMCPClient,
	authorizationEndpoint, tokenEndpoint, redirectURI string,
	scopes []string,
) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     client.ID,
		ClientSecret: client.Secret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  authorizationEndpoint,
			TokenURL: tokenEndpoint,
		},
		RedirectURL: redirectURI,
		Scopes:      scopes,
	}
}

func (a *authorizer) AuthorizationURL(
	server entity.AgentMCPAuthServer,
	client entity.AgentMCPClient,
	redirectURI, state, verifier string,
) string {
	return config(client, server.AuthorizationEndpoint, server.TokenEndpoint, redirectURI, server.Scopes).
		AuthCodeURL(
			state,
			oauth2.S256ChallengeOption(verifier),
			oauth2.SetAuthURLParam(resourceParameter, server.Resource),
		)
}

func (a *authorizer) Exchange(
	ctx context.Context,
	attempt entity.AgentMCPOAuthState,
	code, redirectURI string,
	allowPrivate bool,
) (entity.AgentMCPTokens, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, a.client.HTTP(allowPrivate))

	token, err := config(attempt.Client, "", attempt.TokenEndpoint, redirectURI, attempt.Scopes).Exchange(
		ctx,
		code,
		oauth2.VerifierOption(attempt.Verifier),
		oauth2.SetAuthURLParam(resourceParameter, attempt.Resource),
	)
	if err != nil {
		return entity.AgentMCPTokens{}, tokenFailure(err)
	}

	return tokensOf(token, attempt.Client.Secret, ""), nil
}

func (a *authorizer) Refresh(
	ctx context.Context,
	connection entity.AgentMCPConnection,
	tokens entity.AgentMCPTokens,
	allowPrivate bool,
) (entity.AgentMCPTokens, error) {
	if tokens.RefreshToken == "" {
		return entity.AgentMCPTokens{}, entity.ErrAgentMCPOAuthRefused
	}

	ctx = context.WithValue(ctx, oauth2.HTTPClient, a.client.HTTP(allowPrivate))

	client := entity.AgentMCPClient{ID: connection.ClientID, Secret: tokens.ClientSecret}

	token, err := config(client, "", connection.TokenEndpoint, "", connection.Scopes).
		TokenSource(ctx, &oauth2.Token{RefreshToken: tokens.RefreshToken, Expiry: time.Unix(1, 0)}).
		Token()
	if err != nil {
		return entity.AgentMCPTokens{}, tokenFailure(err)
	}

	return tokensOf(token, tokens.ClientSecret, tokens.RefreshToken), nil
}

func tokensOf(token *oauth2.Token, clientSecret, previousRefresh string) entity.AgentMCPTokens {
	tokens := entity.AgentMCPTokens{
		ClientSecret: clientSecret,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
	}

	if tokens.RefreshToken == "" {
		tokens.RefreshToken = previousRefresh
	}

	if !token.Expiry.IsZero() {
		expiry := token.Expiry.UTC()
		tokens.ExpiresAt = &expiry
	}

	return tokens
}

func tokenFailure(err error) error {
	var retrieve *oauth2.RetrieveError
	if errors.As(err, &retrieve) {
		return fmt.Errorf("%w: %s", entity.ErrAgentMCPOAuthRefused, retrieve.ErrorCode)
	}

	return fmt.Errorf("%w: %v", entity.ErrAgentMCPServerUnreachable, err)
}

func translate(err error) error {
	if errors.Is(err, toolingclient.ErrRefused) {
		return entity.ErrAgentMCPDestinationRefused
	}

	return fmt.Errorf("%w: %w", entity.ErrAgentMCPServerUnreachable, err)
}
