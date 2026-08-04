package oidcprovider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
)

type Client struct {
	http *http.Client
}

func New(cfg config.OIDC) *Client {
	return &Client{
		http: &http.Client{
			Timeout:   cfg.RequestTimeout,
			Transport: &cappedTransport{inner: http.DefaultTransport, limit: cfg.MaxResponseSize},
		},
	}
}

type cappedTransport struct {
	inner http.RoundTripper
	limit int64
}

func (t *cappedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.inner.RoundTrip(request)
	if err != nil {
		return nil, err
	}

	response.Body = struct {
		io.Reader
		io.Closer
	}{io.LimitReader(response.Body, t.limit), response.Body}

	return response, nil
}

func (c *Client) context(ctx context.Context) context.Context {
	return oidc.ClientContext(context.WithValue(ctx, oauth2.HTTPClient, c.http), c.http)
}

func (c *Client) Discover(ctx context.Context, issuer string) (entity.OIDCEndpoints, error) {
	provider, err := oidc.NewProvider(c.context(ctx), strings.TrimRight(strings.TrimSpace(issuer), "/"))
	if err != nil {
		return entity.OIDCEndpoints{}, entity.SSOFailure(
			entity.SSOStageDiscovery,
			"Norn could not read the discovery document at that issuer.",
			err,
		)
	}

	var document struct {
		Issuer           string `json:"issuer"`
		JWKSURI          string `json:"jwks_uri"`
		UserinfoEndpoint string `json:"userinfo_endpoint"`
	}

	if err := provider.Claims(&document); err != nil {
		return entity.OIDCEndpoints{}, entity.SSOFailure(
			entity.SSOStageDiscovery,
			"The discovery document could not be read.",
			err,
		)
	}

	endpoints := entity.OIDCEndpoints{
		Issuer:                document.Issuer,
		AuthorizationEndpoint: provider.Endpoint().AuthURL,
		TokenEndpoint:         provider.Endpoint().TokenURL,
		JWKSURI:               document.JWKSURI,
		UserinfoEndpoint:      document.UserinfoEndpoint,
	}

	if err := endpoints.Validate(); err != nil {
		return entity.OIDCEndpoints{}, err
	}

	return endpoints, nil
}

type Session struct {
	client   *Client
	config   oauth2.Config
	issuer   string
	verifier *oidc.IDTokenVerifier
}

func (c *Client) For(ctx context.Context, connection entity.OIDCConnection, redirectURI string) *Session {
	keys := oidc.NewRemoteKeySet(c.context(ctx), connection.Endpoints.JWKSURI)

	return &Session{
		client: c,
		issuer: connection.Endpoints.Issuer,
		config: oauth2.Config{
			ClientID:     connection.ClientID,
			ClientSecret: connection.ClientSecret,
			RedirectURL:  redirectURI,
			Scopes:       connection.Scopes,
			Endpoint: oauth2.Endpoint{
				AuthURL:  connection.Endpoints.AuthorizationEndpoint,
				TokenURL: connection.Endpoints.TokenEndpoint,
			},
		},
		verifier: oidc.NewVerifier(
			connection.Endpoints.Issuer,
			keys,
			&oidc.Config{ClientID: connection.ClientID},
		),
	}
}

func (s *Session) AuthCodeURL(state, nonce, verifier string) string {
	return s.config.AuthCodeURL(
		state,
		oidc.Nonce(nonce),
		oauth2.AccessTypeOffline,
		oauth2.S256ChallengeOption(verifier),
	)
}

func (s *Session) Exchange(ctx context.Context, code, verifier, nonce string) (entity.OIDCClaims, error) {
	token, err := s.config.Exchange(s.client.context(ctx), code, oauth2.VerifierOption(verifier))
	if err != nil {
		return entity.OIDCClaims{}, entity.SSOFailure(
			entity.SSOStageTokenExchange,
			"The provider refused to exchange the sign-in code for a token.",
			providerMessage(err),
		)
	}

	raw, present := token.Extra("id_token").(string)
	if !present || raw == "" {
		return entity.OIDCClaims{}, entity.NewSSOError(
			entity.SSOStageIDToken,
			"The provider returned no ID token. Check that the openid scope is allowed for this client.",
		)
	}

	verified, err := s.verifier.Verify(s.client.context(ctx), raw)
	if err != nil {
		return entity.OIDCClaims{}, entity.SSOFailure(
			entity.SSOStageIDToken,
			"The ID token from the provider could not be verified.",
			err,
		)
	}

	if verified.Nonce != nonce {
		return entity.OIDCClaims{}, entity.NewSSOError(
			entity.SSOStageIDToken,
			"The ID token does not belong to this sign-in attempt.",
		)
	}

	return claimsOf(verified)
}

func claimsOf(token *oidc.IDToken) (entity.OIDCClaims, error) {
	var payload struct {
		Email             string   `json:"email"`
		EmailVerified     *bool    `json:"email_verified"`
		Name              string   `json:"name"`
		PreferredUsername string   `json:"preferred_username"`
		Groups            []string `json:"groups"`
	}

	if err := token.Claims(&payload); err != nil {
		return entity.OIDCClaims{}, entity.SSOFailure(
			entity.SSOStageClaims,
			"The claims in the ID token could not be read.",
			err,
		)
	}

	name := payload.Name
	if name == "" {
		name = payload.PreferredUsername
	}

	return entity.OIDCClaims{
		Subject:       token.Subject,
		Email:         payload.Email,
		EmailVerified: payload.EmailVerified,
		Name:          name,
		Groups:        payload.Groups,
	}, nil
}

func providerMessage(err error) error {
	var retrieved *oauth2.RetrieveError
	if !errors.As(err, &retrieved) {
		return err
	}

	if retrieved.ErrorCode == "" {
		return fmt.Errorf("%s: %s", retrieved.Response.Status, strings.TrimSpace(string(retrieved.Body)))
	}

	if retrieved.ErrorDescription == "" {
		return errors.New(retrieved.ErrorCode)
	}

	return fmt.Errorf("%s: %s", retrieved.ErrorCode, retrieved.ErrorDescription)
}
