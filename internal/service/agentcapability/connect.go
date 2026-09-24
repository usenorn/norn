package agentcapability

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

const randomTokenBytes = 32

func random() (string, error) {
	buffer := make([]byte, randomTokenBytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func (s *capabilities) BeginMCPConnect(ctx context.Context, input service.BeginMCPConnectInput) (string, error) {
	server, decision, err := s.editableServer(ctx, input.WorkspaceID, input.ServerID)
	if err != nil {
		return "", err
	}

	if server.Auth != entity.AgentMCPAuthOAuth {
		return "", entity.ErrAgentMCPOAuthNotConfigured
	}

	if !entity.ValidAgentMCPReturnTo(input.ReturnTo) {
		return "", entity.NewValidationError(
			entity.FieldError{Field: "returnTo", Code: entity.ValidationCodeMalformed},
		)
	}

	allowPrivate := s.instance.SelfHosted

	discovered, err := s.oauth.Discover(ctx, server.URL, allowPrivate)
	if err != nil {
		return "", err
	}

	client := entity.AgentMCPClient{ID: server.OAuthClientID}

	if client.ID != "" {
		secrets, err := s.servers.Secrets(ctx, server.WorkspaceID, server.ID)
		if err != nil {
			return "", err
		}

		client.Secret = secrets.OAuthClientSecret
	} else {
		client, err = s.oauth.Register(ctx, discovered, input.RedirectURI, allowPrivate)
		if err != nil {
			return "", err
		}
	}

	state, err := random()
	if err != nil {
		return "", err
	}

	verifier, err := random()
	if err != nil {
		return "", err
	}

	if err := s.states.Put(ctx, state, entity.AgentMCPOAuthState{
		WorkspaceID:   server.WorkspaceID,
		ServerID:      server.ID,
		AccountID:     decision.Actor.AccountID,
		Verifier:      verifier,
		Issuer:        discovered.Issuer,
		TokenEndpoint: discovered.TokenEndpoint,
		Resource:      discovered.Resource,
		Scopes:        discovered.Scopes,
		Client:        client,
		ReturnTo:      input.ReturnTo,
		CreatedAt:     time.Now().UTC(),
	}); err != nil {
		return "", err
	}

	return s.oauth.AuthorizationURL(discovered, client, input.RedirectURI, state, verifier), nil
}

func (s *capabilities) CompleteMCPConnect(
	ctx context.Context,
	state, code, redirectURI string,
) (service.CompletedMCPConnect, error) {
	attempt, err := s.states.Take(ctx, state)
	if err != nil {
		return service.CompletedMCPConnect{}, err
	}

	completed := service.CompletedMCPConnect{
		WorkspaceID: attempt.WorkspaceID,
		ServerID:    attempt.ServerID,
		ReturnTo:    attempt.ReturnTo,
	}

	if code == "" {
		return completed, entity.ErrAgentMCPOAuthRefused
	}

	server, err := s.servers.Get(ctx, attempt.WorkspaceID, attempt.ServerID)
	if err != nil {
		return completed, err
	}

	tokens, err := s.oauth.Exchange(ctx, attempt, code, redirectURI, s.instance.SelfHosted)
	if err != nil {
		return completed, err
	}

	if tokens.AccessToken == "" {
		return completed, entity.ErrAgentMCPOAuthRefused
	}

	tokens.ClientSecret = attempt.Client.Secret

	if _, err := s.connections.Save(ctx, entity.AgentMCPConnection{
		ServerID:      server.ID,
		WorkspaceID:   server.WorkspaceID,
		Issuer:        attempt.Issuer,
		TokenEndpoint: attempt.TokenEndpoint,
		ClientID:      attempt.Client.ID,
		Scopes:        attempt.Scopes,
		ConnectedBy:   attempt.AccountID,
		ConnectedAt:   time.Now().UTC(),
	}, tokens); err != nil {
		return completed, err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  server.WorkspaceID,
		Action:       entity.AuditAgentMCPConnected,
		Actor:        entity.AuditActor{AccountID: attempt.AccountID, Kind: entity.ActorKindUser},
		ResourceKind: auditServerKind,
		ResourceID:   server.ID,
		ResourceName: server.Name,
		Detail:       map[string]string{"issuer": attempt.Issuer},
	})

	return completed, nil
}

func (s *capabilities) DisconnectMCP(ctx context.Context, workspaceID, serverID uuid.UUID) error {
	server, _, err := s.editableServer(ctx, workspaceID, serverID)
	if err != nil {
		return err
	}

	if err := s.connections.Delete(ctx, workspaceID, serverID); err != nil {
		return err
	}

	s.record(ctx, workspaceID, entity.AuditAgentMCPDisconnected, auditServerKind, server.ID, server.Name, server.AgentID)

	return nil
}
