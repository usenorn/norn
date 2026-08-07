package mcpconnection

import (
	"context"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

const usageStampInterval = time.Minute

type mcpConnectionsService struct {
	clients     repository.MCPClient
	connections repository.MCPConnection
	tokens      repository.MCPToken
	authState   repository.MCPAuthState
	accounts    repository.Account
	workspaces  repository.Workspace
	memberships repository.Membership
	authorizer  service.Authorizer
	audit       service.Audit
	transactor  repository.Transactor
	app         config.App
	cfg         config.MCP
}

func New(
	clients repository.MCPClient,
	connections repository.MCPConnection,
	tokens repository.MCPToken,
	authState repository.MCPAuthState,
	accounts repository.Account,
	workspaces repository.Workspace,
	memberships repository.Membership,
	authorizer service.Authorizer,
	audit service.Audit,
	transactor repository.Transactor,
	app config.App,
	cfg config.MCP,
) service.MCPConnections {
	return &mcpConnectionsService{
		clients:     clients,
		connections: connections,
		tokens:      tokens,
		authState:   authState,
		accounts:    accounts,
		workspaces:  workspaces,
		memberships: memberships,
		authorizer:  authorizer,
		audit:       audit,
		transactor:  transactor,
		app:         app,
		cfg:         cfg,
	}
}

func (s *mcpConnectionsService) RegisterClient(
	ctx context.Context,
	input service.RegisterMCPClientInput,
) (entity.MCPClient, error) {
	if err := entity.NewValidationError(
		entity.ValidateMCPClientName("client_name", input.Name),
	); err != nil {
		return entity.MCPClient{}, err
	}

	if len(input.RedirectURIs) == 0 {
		return entity.MCPClient{}, entity.NewValidationError(entity.FieldError{
			Field: "redirect_uris", Code: entity.ValidationCodeRequired,
		})
	}

	if len(input.RedirectURIs) > entity.MCPClientMaxRedirect {
		return entity.MCPClient{}, entity.NewValidationError(entity.FieldError{
			Field: "redirect_uris", Code: entity.ValidationCodeOutOfRange,
		})
	}

	for _, uri := range input.RedirectURIs {
		if !entity.ValidMCPRedirectURI(uri) {
			return entity.MCPClient{}, entity.NewValidationError(entity.FieldError{
				Field: "redirect_uris", Code: entity.ValidationCodeMalformed,
			})
		}
	}

	return s.clients.Create(ctx, entity.MCPClient{
		Name:         strings.TrimSpace(input.Name),
		RedirectURIs: input.RedirectURIs,
	})
}

func (s *mcpConnectionsService) BeginAuthorization(
	ctx context.Context,
	input service.BeginMCPAuthorizationInput,
) (string, error) {
	clientID, err := uuid.Parse(input.ClientID)
	if err != nil {
		return "", entity.ErrMCPClientNotFound
	}

	client, err := s.clients.GetByID(ctx, clientID)
	if err != nil {
		return "", err
	}

	if !client.PermitsRedirect(input.RedirectURI) {
		return "", entity.ErrMCPRedirectInvalid
	}

	if input.ResponseType != "code" {
		return "", entity.ErrMCPRequestInvalid
	}

	if input.CodeChallenge == "" || input.CodeChallengeMethod != "S256" {
		return "", entity.ErrMCPRequestInvalid
	}

	capability, err := entity.ParseMCPScopes(input.Scope)
	if err != nil {
		return "", err
	}

	if input.Resource != "" && strings.TrimRight(input.Resource, "/") != s.resource() {
		return "", entity.ErrMCPResourceInvalid
	}

	requestID := uuid.NewString()

	if err := s.authState.PutRequest(ctx, requestID, entity.MCPAuthRequest{
		ClientID:      client.ID,
		ClientName:    client.Name,
		RedirectURI:   input.RedirectURI,
		Capability:    capability,
		State:         input.State,
		CodeChallenge: input.CodeChallenge,
		Resource:      input.Resource,
		CreatedAt:     time.Now().UTC(),
	}); err != nil {
		return "", err
	}

	return requestID, nil
}

func (s *mcpConnectionsService) DescribeAuthorization(
	ctx context.Context,
	requestID string,
) (service.MCPAuthorizationView, error) {
	actor, err := s.self(ctx)
	if err != nil {
		return service.MCPAuthorizationView{}, err
	}

	request, err := s.authState.GetRequest(ctx, requestID)
	if err != nil {
		return service.MCPAuthorizationView{}, err
	}

	reachable, err := s.workspaces.ListByAccountID(ctx, actor.AccountID)
	if err != nil {
		return service.MCPAuthorizationView{}, err
	}

	return service.MCPAuthorizationView{
		ClientName: request.ClientName,
		Capability: request.Capability,
		Workspaces: reachable,
	}, nil
}

func (s *mcpConnectionsService) Approve(
	ctx context.Context,
	requestID string,
	input service.ApproveMCPAuthorizationInput,
) (service.MCPAuthorizationDecision, error) {
	actor, err := s.self(ctx)
	if err != nil {
		return service.MCPAuthorizationDecision{}, err
	}

	grants, err := s.chosenGrants(ctx, actor.AccountID, input)
	if err != nil {
		return service.MCPAuthorizationDecision{}, err
	}

	request, err := s.authState.TakeRequest(ctx, requestID)
	if err != nil {
		return service.MCPAuthorizationDecision{}, err
	}

	code, err := entity.NewMCPAuthCode()
	if err != nil {
		return service.MCPAuthorizationDecision{}, err
	}

	if err := s.authState.PutCode(ctx, code, entity.MCPAuthCode{
		ClientID:      request.ClientID,
		AccountID:     actor.AccountID,
		RedirectURI:   request.RedirectURI,
		Capability:    request.Capability,
		CodeChallenge: request.CodeChallenge,
		Grants:        grants,
	}); err != nil {
		return service.MCPAuthorizationDecision{}, err
	}

	redirectTo, err := redirectWith(request.RedirectURI, map[string]string{
		"code":  code,
		"state": request.State,
	})
	if err != nil {
		return service.MCPAuthorizationDecision{}, err
	}

	return service.MCPAuthorizationDecision{RedirectTo: redirectTo}, nil
}

func (s *mcpConnectionsService) chosenGrants(
	ctx context.Context,
	accountID uuid.UUID,
	input service.ApproveMCPAuthorizationInput,
) (entity.APITokenGrants, error) {
	if input.AllWorkspaces {
		return nil, nil
	}

	reachable, err := s.workspaces.ListByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}

	return entity.MCPGrantsFor(input.WorkspaceIDs, reachable)
}

func (s *mcpConnectionsService) Deny(
	ctx context.Context,
	requestID string,
) (service.MCPAuthorizationDecision, error) {
	if _, err := s.self(ctx); err != nil {
		return service.MCPAuthorizationDecision{}, err
	}

	request, err := s.authState.TakeRequest(ctx, requestID)
	if err != nil {
		return service.MCPAuthorizationDecision{}, err
	}

	redirectTo, err := redirectWith(request.RedirectURI, map[string]string{
		"error": "access_denied",
		"state": request.State,
	})
	if err != nil {
		return service.MCPAuthorizationDecision{}, err
	}

	return service.MCPAuthorizationDecision{RedirectTo: redirectTo}, nil
}

func (s *mcpConnectionsService) Exchange(
	ctx context.Context,
	input service.ExchangeMCPCodeInput,
) (service.MCPTokenPair, error) {
	grant, err := s.authState.TakeCode(ctx, input.Code)
	if err != nil {
		return service.MCPTokenPair{}, err
	}

	clientID, err := uuid.Parse(input.ClientID)
	if err != nil || clientID != grant.ClientID {
		return service.MCPTokenPair{}, entity.ErrMCPCodeInvalid
	}

	if input.RedirectURI != grant.RedirectURI {
		return service.MCPTokenPair{}, entity.ErrMCPCodeInvalid
	}

	if !entity.VerifyPKCE(grant.CodeChallenge, input.CodeVerifier) {
		return service.MCPTokenPair{}, entity.ErrMCPCodeInvalid
	}

	client, err := s.clients.GetByID(ctx, grant.ClientID)
	if err != nil {
		return service.MCPTokenPair{}, err
	}

	pair, mint, err := s.mintPair(grant.Capability)
	if err != nil {
		return service.MCPTokenPair{}, err
	}

	var connection entity.MCPConnection

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		created, err := s.connections.Create(ctx, entity.MCPConnection{
			AccountID:  grant.AccountID,
			ClientID:   grant.ClientID,
			ClientName: client.Name,
			Scopes:     entity.MCPScopesFor(grant.Capability),
			Grants:     grant.Grants,
		})
		if err != nil {
			return err
		}

		connection = created

		return mint(ctx, connection.ID)
	}); err != nil {
		return service.MCPTokenPair{}, err
	}

	s.recordConnection(ctx, entity.AuditConnectionAuthorized, connection, uuid.Nil)

	return pair, nil
}

func (s *mcpConnectionsService) Refresh(
	ctx context.Context,
	input service.RefreshMCPTokenInput,
) (service.MCPTokenPair, error) {
	token, err := s.tokens.GetByHash(ctx, entity.HashMCPToken(input.RefreshToken))
	if err != nil {
		return service.MCPTokenPair{}, entity.ErrMCPCodeInvalid
	}

	now := time.Now().UTC()

	if token.Kind != entity.MCPTokenKindRefresh || token.ExpiredAt(now) {
		return service.MCPTokenPair{}, entity.ErrMCPCodeInvalid
	}

	if token.Consumed() {
		s.revokeFamily(ctx, token.ConnectionID, "a consumed mcp refresh token was replayed")

		return service.MCPTokenPair{}, entity.ErrMCPCodeInvalid
	}

	connection, err := s.connections.GetByID(ctx, token.ConnectionID)
	if err != nil {
		return service.MCPTokenPair{}, entity.ErrMCPCodeInvalid
	}

	if !connection.Usable() {
		return service.MCPTokenPair{}, entity.ErrMCPCodeInvalid
	}

	clientID, err := uuid.Parse(input.ClientID)
	if err != nil || clientID != connection.ClientID {
		return service.MCPTokenPair{}, entity.ErrMCPCodeInvalid
	}

	pair, mint, err := s.mintPair(connection.Capability())
	if err != nil {
		return service.MCPTokenPair{}, err
	}

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.tokens.Consume(ctx, token.ID, now); err != nil {
			return err
		}

		if err := s.tokens.PruneExpired(ctx, connection.ID, now); err != nil {
			return err
		}

		return mint(ctx, connection.ID)
	}); err != nil {
		s.revokeFamily(ctx, token.ConnectionID, "an mcp refresh token raced its own rotation")

		return service.MCPTokenPair{}, entity.ErrMCPCodeInvalid
	}

	return pair, nil
}

func (s *mcpConnectionsService) RevokeByValue(ctx context.Context, value, clientID string) error {
	token, err := s.tokens.GetByHash(ctx, entity.HashMCPToken(value))
	if err != nil {
		return nil
	}

	connection, err := s.connections.GetByID(ctx, token.ConnectionID)
	if err != nil {
		return err
	}

	claimed, err := uuid.Parse(clientID)
	if err != nil || claimed != connection.ClientID {
		return nil
	}

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.connections.Revoke(ctx, token.ConnectionID, time.Now().UTC()); err != nil {
			return err
		}

		return s.tokens.DeleteForConnection(ctx, token.ConnectionID)
	}); err != nil {
		return err
	}

	s.recordConnection(ctx, entity.AuditConnectionRevoked, connection, uuid.Nil)

	return nil
}

func (s *mcpConnectionsService) Authenticate(
	ctx context.Context,
	value string,
) (entity.Actor, entity.MCPConnection, error) {
	token, err := s.tokens.GetByHash(ctx, entity.HashMCPToken(value))
	if err != nil {
		return entity.Actor{}, entity.MCPConnection{}, err
	}

	now := time.Now().UTC()

	if token.Kind != entity.MCPTokenKindAccess || token.ExpiredAt(now) {
		return entity.Actor{}, entity.MCPConnection{}, entity.ErrMCPTokenNotFound
	}

	connection, err := s.connections.GetByID(ctx, token.ConnectionID)
	if err != nil {
		return entity.Actor{}, entity.MCPConnection{}, err
	}

	if !connection.Usable() {
		return entity.Actor{}, entity.MCPConnection{}, entity.ErrMCPConnectionRevoked
	}

	account, err := s.accounts.GetByID(ctx, connection.AccountID)
	if err != nil {
		return entity.Actor{}, entity.MCPConnection{}, err
	}

	if account.Status != entity.AccountStatusActive {
		return entity.Actor{}, entity.MCPConnection{}, entity.ErrMCPTokenNotFound
	}

	if connection.NeedsUsageStamp(now, usageStampInterval) {
		if err := s.connections.RecordUsage(ctx, connection.ID, now); err != nil {
			logging.From(ctx).WarnContext(
				ctx,
				"recording mcp connection usage failed",
				"connection_id", connection.ID.String(),
				"error", err.Error(),
			)
		}
	}

	connectionID := connection.ID

	return entity.Actor{
		Kind:           entity.ActorKindToken,
		AccountID:      connection.AccountID,
		ConnectionID:   &connectionID,
		ConnectionName: connection.ClientName,
		Grants:         connection.Grants,
		Scopes:         connection.Scopes,
	}, connection, nil
}

func (s *mcpConnectionsService) List(ctx context.Context) ([]entity.MCPConnection, error) {
	actor, err := s.self(ctx)
	if err != nil {
		return nil, err
	}

	return s.connections.ListByAccount(ctx, actor.AccountID)
}

func (s *mcpConnectionsService) Revoke(ctx context.Context, connectionID uuid.UUID) error {
	actor, err := s.self(ctx)
	if err != nil {
		return err
	}

	connection, err := s.connections.GetByID(ctx, connectionID)
	if err != nil {
		return err
	}

	if connection.AccountID != actor.AccountID {
		return entity.ErrMCPConnectionNotFound
	}

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.connections.Revoke(ctx, connectionID, time.Now().UTC()); err != nil {
			return err
		}

		return s.tokens.DeleteForConnection(ctx, connectionID)
	}); err != nil {
		return err
	}

	s.recordConnection(ctx, entity.AuditConnectionRevoked, connection, uuid.Nil)

	return nil
}

func (s *mcpConnectionsService) Narrow(
	ctx context.Context,
	connectionID uuid.UUID,
	input service.NarrowMCPConnectionInput,
) (entity.MCPConnection, error) {
	actor, err := s.self(ctx)
	if err != nil {
		return entity.MCPConnection{}, err
	}

	connection, err := s.connections.GetByID(ctx, connectionID)
	if err != nil {
		return entity.MCPConnection{}, err
	}

	if connection.AccountID != actor.AccountID || connection.Revoked() {
		return entity.MCPConnection{}, entity.ErrMCPConnectionNotFound
	}

	var scopes entity.APIScopeSet

	if input.Capability != nil {
		if !input.Capability.Valid() {
			return entity.MCPConnection{}, entity.ErrMCPScopeInvalid
		}

		narrowed := entity.MCPScopesFor(*input.Capability)
		if !narrowed.SubsetOf(connection.Scopes) {
			return entity.MCPConnection{}, entity.ErrMCPGrantInvalid
		}

		scopes = narrowed
	}

	var grants entity.APITokenGrants

	if input.Grants != nil {
		grants = *input.Grants

		if err := s.checkNarrowing(ctx, connection, grants); err != nil {
			return entity.MCPConnection{}, err
		}
	}

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if scopes != nil {
			if err := s.connections.SetScopes(ctx, connectionID, scopes); err != nil {
				return err
			}
		}

		if grants != nil {
			if err := s.connections.ReplaceGrants(ctx, connectionID, grants); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return entity.MCPConnection{}, err
	}

	s.recordConnection(ctx, entity.AuditConnectionNarrowed, connection, uuid.Nil)

	return s.connections.GetByID(ctx, connectionID)
}

func (s *mcpConnectionsService) checkNarrowing(
	ctx context.Context,
	connection entity.MCPConnection,
	grants entity.APITokenGrants,
) error {
	if len(grants) == 0 {
		return entity.ErrMCPGrantInvalid
	}

	seen := make(map[uuid.UUID]struct{}, len(grants))

	for _, grant := range grants {
		if _, duplicate := seen[grant.WorkspaceID]; duplicate {
			return entity.ErrMCPGrantInvalid
		}

		seen[grant.WorkspaceID] = struct{}{}

		if grant.AllTeams && len(grant.TeamIDs) > 0 {
			return entity.ErrMCPGrantInvalid
		}

		if !grant.AllTeams && len(grant.TeamIDs) == 0 {
			return entity.ErrMCPGrantInvalid
		}

		if !connection.FollowsMembership() {
			current, ok := connection.Grants.For(grant.WorkspaceID)
			if !ok {
				return entity.ErrMCPGrantInvalid
			}

			if !current.AllTeams {
				if grant.AllTeams {
					return entity.ErrMCPGrantInvalid
				}

				for _, teamID := range grant.TeamIDs {
					if !slices.Contains(current.TeamIDs, teamID) {
						return entity.ErrMCPGrantInvalid
					}
				}
			}
		}

		decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
			Resource:    entity.ResourceIssue,
			Action:      entity.ActionRead,
			WorkspaceID: grant.WorkspaceID,
			Scoped:      true,
		})
		if err != nil {
			return entity.ErrMCPGrantInvalid
		}

		for _, teamID := range grant.TeamIDs {
			if !decision.Scope.Covers(teamID) {
				return entity.ErrMCPGrantInvalid
			}
		}
	}

	return nil
}

func (s *mcpConnectionsService) ListForWorkspace(
	ctx context.Context,
	workspaceID uuid.UUID,
) ([]service.OwnedMCPConnection, error) {
	if _, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceMCPConnection,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
	}); err != nil {
		return nil, err
	}

	connections, err := s.connections.ListReachingWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	owned := make([]service.OwnedMCPConnection, 0, len(connections))

	for _, connection := range connections {
		account, err := s.accounts.GetByID(ctx, connection.AccountID)
		if err != nil {
			return nil, err
		}

		owned = append(owned, service.OwnedMCPConnection{
			Connection: connection,
			OwnerName:  account.DisplayName,
			OwnerEmail: account.Email,
		})
	}

	return owned, nil
}

func (s *mcpConnectionsService) RevokeInWorkspace(
	ctx context.Context,
	workspaceID, connectionID uuid.UUID,
) error {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceMCPConnection,
		Action:      entity.ActionManage,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return err
	}

	if decision.Actor.Kind != entity.ActorKindUser {
		return entity.ErrMCPManageForbidden
	}

	connection, err := s.connections.GetByID(ctx, connectionID)
	if err != nil {
		return err
	}

	if connection.Revoked() {
		return entity.ErrMCPConnectionNotFound
	}

	if connection.FollowsMembership() {
		if _, err := s.memberships.Get(ctx, workspaceID, connection.AccountID); err != nil {
			return entity.ErrMCPConnectionNotFound
		}
	} else if !connection.Grants.Covers(workspaceID) {
		return entity.ErrMCPConnectionNotFound
	}

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.connections.Revoke(ctx, connectionID, time.Now().UTC()); err != nil {
			return err
		}

		return s.tokens.DeleteForConnection(ctx, connectionID)
	}); err != nil {
		return err
	}

	s.recordConnection(ctx, entity.AuditConnectionRevoked, connection, workspaceID)

	return nil
}

func (s *mcpConnectionsService) mintPair(
	capability entity.MCPCapability,
) (service.MCPTokenPair, func(ctx context.Context, connectionID uuid.UUID) error, error) {
	accessValue, accessHash, err := entity.NewMCPTokenValue()
	if err != nil {
		return service.MCPTokenPair{}, nil, err
	}

	refreshValue, refreshHash, err := entity.NewMCPTokenValue()
	if err != nil {
		return service.MCPTokenPair{}, nil, err
	}

	pair := service.MCPTokenPair{
		AccessToken:  accessValue,
		RefreshToken: refreshValue,
		ExpiresIn:    int(s.cfg.AccessTokenTTL.Seconds()),
		Capability:   capability,
	}

	mint := func(ctx context.Context, connectionID uuid.UUID) error {
		now := time.Now().UTC()

		if _, err := s.tokens.Create(ctx, entity.MCPToken{
			ConnectionID: connectionID,
			Kind:         entity.MCPTokenKindAccess,
			TokenHash:    accessHash,
			ExpiresAt:    now.Add(s.cfg.AccessTokenTTL),
		}); err != nil {
			return err
		}

		_, err := s.tokens.Create(ctx, entity.MCPToken{
			ConnectionID: connectionID,
			Kind:         entity.MCPTokenKindRefresh,
			TokenHash:    refreshHash,
			ExpiresAt:    now.Add(s.cfg.RefreshTokenTTL),
		})

		return err
	}

	return pair, mint, nil
}

func (s *mcpConnectionsService) revokeFamily(ctx context.Context, connectionID uuid.UUID, reason string) {
	logging.From(ctx).WarnContext(
		ctx,
		reason,
		"connection_id", connectionID.String(),
	)

	connection, connectionErr := s.connections.GetByID(ctx, connectionID)

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.connections.Revoke(ctx, connectionID, time.Now().UTC()); err != nil {
			return err
		}

		return s.tokens.DeleteForConnection(ctx, connectionID)
	}); err != nil {
		logging.From(ctx).ErrorContext(
			ctx,
			"revoking a compromised mcp connection failed",
			"connection_id", connectionID.String(),
			"error", err.Error(),
		)

		return
	}

	if connectionErr == nil {
		s.recordConnection(ctx, entity.AuditConnectionRevoked, connection, uuid.Nil)
	}
}

func (s *mcpConnectionsService) recordConnection(
	ctx context.Context,
	action entity.AuditAction,
	connection entity.MCPConnection,
	workspaceID uuid.UUID,
) {
	entry := entity.AuditEntry{
		WorkspaceID:  workspaceID,
		Action:       action,
		ResourceKind: string(entity.ResourceMCPConnection),
		ResourceID:   connection.ID,
		ResourceName: connection.ClientName,
	}

	if _, ok := identity.Actor(ctx); !ok {
		entry.Actor = entity.AuditActor{
			Kind:      entity.ActorKindUser,
			AccountID: connection.AccountID,
		}
	}

	s.audit.Record(ctx, entry)
}

func (s *mcpConnectionsService) self(ctx context.Context) (entity.Actor, error) {
	actor, ok := identity.Actor(ctx)
	if !ok {
		return entity.Actor{}, entity.ErrAccountForbidden
	}

	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource: entity.ResourceMCPConnection,
		Action:   entity.ActionManage,
		Subject:  actor.AccountID,
	})
	if err != nil {
		return entity.Actor{}, err
	}

	if decision.Actor.Kind != entity.ActorKindUser {
		return entity.Actor{}, entity.ErrMCPManageForbidden
	}

	return decision.Actor, nil
}

func (s *mcpConnectionsService) resource() string {
	return strings.TrimRight(s.app.BaseURL, "/") + "/mcp"
}

func redirectWith(redirectURI string, params map[string]string) (string, error) {
	parsed, err := url.Parse(redirectURI)
	if err != nil {
		return "", entity.ErrMCPRedirectInvalid
	}

	query := parsed.Query()

	for key, value := range params {
		if value != "" {
			query.Set(key, value)
		}
	}

	parsed.RawQuery = query.Encode()

	return parsed.String(), nil
}
