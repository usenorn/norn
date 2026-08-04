package ssoconnection

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type connectionsService struct {
	connections repository.OIDCConnection
	states      repository.OIDCState
	workspaces  repository.Workspace
	accounts    repository.Account
	memberships repository.Membership
	provider    repository.OIDCProvider
	sessions    service.Sessions
	authorizer  service.Authorizer
	transactor  repository.Transactor
}

func New(
	connections repository.OIDCConnection,
	states repository.OIDCState,
	workspaces repository.Workspace,
	accounts repository.Account,
	memberships repository.Membership,
	provider repository.OIDCProvider,
	sessions service.Sessions,
	authorizer service.Authorizer,
	transactor repository.Transactor,
) service.SSOConnections {
	return &connectionsService{
		connections: connections,
		states:      states,
		workspaces:  workspaces,
		accounts:    accounts,
		memberships: memberships,
		provider:    provider,
		sessions:    sessions,
		authorizer:  authorizer,
		transactor:  transactor,
	}
}

func (s *connectionsService) authorize(
	ctx context.Context,
	workspaceID uuid.UUID,
	action entity.Action,
) error {
	_, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceSSOConnection,
		Action:      action,
		WorkspaceID: workspaceID,
	})

	return err
}

func (s *connectionsService) Get(
	ctx context.Context,
	workspaceID uuid.UUID,
) (entity.OIDCConnection, error) {
	if err := s.authorize(ctx, workspaceID, entity.ActionRead); err != nil {
		return entity.OIDCConnection{}, err
	}

	return s.connections.Get(ctx, workspaceID)
}

func (s *connectionsService) Discover(
	ctx context.Context,
	workspaceID uuid.UUID,
	issuer string,
) (entity.OIDCEndpoints, error) {
	if err := s.authorize(ctx, workspaceID, entity.ActionUpdate); err != nil {
		return entity.OIDCEndpoints{}, err
	}

	if err := entity.ValidateOIDCIssuer(issuer); err != nil {
		return entity.OIDCEndpoints{}, err
	}

	return s.provider.Discover(ctx, issuer)
}

func (s *connectionsService) Save(
	ctx context.Context,
	input service.SaveOIDCConnectionInput,
) (entity.OIDCConnection, error) {
	if err := s.authorize(ctx, input.WorkspaceID, entity.ActionUpdate); err != nil {
		return entity.OIDCConnection{}, err
	}

	if err := entity.ValidateOIDCIssuer(input.Issuer); err != nil {
		return entity.OIDCConnection{}, err
	}

	secret, err := s.secretFor(ctx, input)
	if err != nil {
		return entity.OIDCConnection{}, err
	}

	connection := entity.OIDCConnection{
		WorkspaceID:  input.WorkspaceID,
		ClientID:     strings.TrimSpace(input.ClientID),
		ClientSecret: secret,
		Scopes:       entity.NormalizeOIDCScopes(input.Scopes),
		GroupsClaim:  strings.TrimSpace(input.GroupsClaim),
		Provisioning: input.Provisioning,
	}

	if input.Endpoints == nil {
		discovered, err := s.provider.Discover(ctx, input.Issuer)
		if err != nil {
			return entity.OIDCConnection{}, err
		}

		connection.Endpoints = discovered
		connection.Discovered = true
	} else {
		connection.Endpoints = *input.Endpoints
		connection.Endpoints.Issuer = strings.TrimSpace(input.Issuer)
		connection.Discovered = false
	}

	if err := connection.Validate(); err != nil {
		return entity.OIDCConnection{}, err
	}

	return s.connections.Save(ctx, connection)
}

func (s *connectionsService) secretFor(
	ctx context.Context,
	input service.SaveOIDCConnectionInput,
) (string, error) {
	if strings.TrimSpace(input.ClientSecret) != "" {
		return input.ClientSecret, nil
	}

	existing, err := s.connections.Get(ctx, input.WorkspaceID)
	if err != nil {
		if errors.Is(err, entity.ErrOIDCConnectionNotFound) {
			return "", entity.NewOIDCError(
				entity.OIDCStageEndpoints,
				"Enter the client secret your provider issued.",
			)
		}

		return "", err
	}

	return existing.ClientSecret, nil
}

func (s *connectionsService) Remove(ctx context.Context, workspaceID uuid.UUID) error {
	if err := s.authorize(ctx, workspaceID, entity.ActionDelete); err != nil {
		return err
	}

	return s.connections.Delete(ctx, workspaceID)
}

func (s *connectionsService) BeginTest(ctx context.Context, workspaceID uuid.UUID) (string, error) {
	if err := s.authorize(ctx, workspaceID, entity.ActionUpdate); err != nil {
		return "", err
	}

	connection, err := s.connections.Get(ctx, workspaceID)
	if err != nil {
		return "", err
	}

	return s.begin(ctx, connection, entity.OIDCPurposeTest, "")
}

func (s *connectionsService) BeginLogin(
	ctx context.Context,
	input service.BeginOIDCLoginInput,
) (string, error) {
	workspace, err := s.workspaces.GetBySlug(ctx, input.WorkspaceSlug)
	if err != nil {
		if errors.Is(err, entity.ErrWorkspaceNotFound) {
			return "", entity.ErrOIDCConnectionNotFound
		}

		return "", err
	}

	connection, err := s.connections.Get(ctx, workspace.ID)
	if err != nil {
		return "", err
	}

	return s.begin(ctx, connection, entity.OIDCPurposeLogin, input.ReturnTo)
}

func (s *connectionsService) begin(
	ctx context.Context,
	connection entity.OIDCConnection,
	purpose entity.OIDCPurpose,
	returnTo string,
) (string, error) {
	state, err := opaque()
	if err != nil {
		return "", err
	}

	nonce, err := opaque()
	if err != nil {
		return "", err
	}

	verifier, err := opaque()
	if err != nil {
		return "", err
	}

	if err := s.states.Put(ctx, state, entity.OIDCState{
		Purpose:     purpose,
		WorkspaceID: connection.WorkspaceID,
		Nonce:       nonce,
		Verifier:    verifier,
		ReturnTo:    safeReturnTo(returnTo),
		CreatedAt:   time.Now().UTC(),
	}); err != nil {
		return "", err
	}

	return s.provider.AuthCodeURL(ctx, connection, entity.OIDCAuthorization{
		State:    state,
		Nonce:    nonce,
		Verifier: verifier,
	}), nil
}

func (s *connectionsService) Complete(
	ctx context.Context,
	input service.CompleteOIDCInput,
) (entity.OIDCExchange, error) {
	attempt, err := s.states.Take(ctx, input.State)
	if err != nil {
		return entity.OIDCExchange{}, err
	}

	if !attempt.Purpose.Valid() {
		return entity.OIDCExchange{}, entity.ErrOIDCStateNotFound
	}

	workspace, err := s.workspaces.GetByID(ctx, attempt.WorkspaceID)
	if err != nil {
		return entity.OIDCExchange{}, err
	}

	exchange := entity.OIDCExchange{
		Purpose:       attempt.Purpose,
		WorkspaceID:   attempt.WorkspaceID,
		WorkspaceSlug: workspace.Slug,
		ReturnTo:      attempt.ReturnTo,
	}

	connection, err := s.connections.Get(ctx, attempt.WorkspaceID)
	if err != nil {
		return exchange, err
	}

	claims, err := s.provider.Exchange(ctx, connection, entity.OIDCRedemption{
		Code:     input.Code,
		Nonce:    attempt.Nonce,
		Verifier: attempt.Verifier,
	})
	if err != nil {
		return exchange, err
	}

	if err := entity.ValidateClaims(claims); err != nil {
		return exchange, err
	}

	exchange.Claims = claims

	if attempt.Purpose == entity.OIDCPurposeTest {
		if err := s.connections.MarkVerified(ctx, attempt.WorkspaceID, time.Now().UTC()); err != nil {
			return exchange, err
		}

		return exchange, nil
	}

	account, provisioned, err := s.admit(ctx, connection, claims)
	if err != nil {
		return exchange, err
	}

	issued, err := s.sessions.Start(ctx, service.StartSessionInput{
		AccountID:  account.ID,
		AuthMethod: entity.SessionAuthMethodSSO,
		Client:     input.Client,
	})
	if err != nil {
		return exchange, err
	}

	exchange.Session = issued.Session
	exchange.Token = issued.Token
	exchange.Provisioned = provisioned

	return exchange, nil
}

func (s *connectionsService) admit(
	ctx context.Context,
	connection entity.OIDCConnection,
	claims entity.OIDCClaims,
) (entity.Account, bool, error) {
	email := entity.NormalizeEmail(claims.Email)

	account, err := s.accounts.GetByEmail(ctx, email)
	if err != nil && !errors.Is(err, entity.ErrAccountNotFound) {
		return entity.Account{}, false, err
	}

	exists := err == nil
	member := false

	if exists {
		if account.Status != entity.AccountStatusActive {
			return entity.Account{}, false, entity.NewOIDCError(
				entity.OIDCStageMatching,
				"The Norn account for "+email+" is not active.",
			)
		}

		if _, err := s.memberships.Get(ctx, connection.WorkspaceID, account.ID); err != nil {
			if !errors.Is(err, entity.ErrMembershipNotFound) {
				return entity.Account{}, false, err
			}
		} else {
			member = true
		}
	}

	outcome := entity.ResolveMatch(exists, member, connection.Provisioning)
	if !outcome.Admits() {
		return entity.Account{}, false, outcome.Refusal(email)
	}

	if outcome == entity.MatchOutcomeSignIn {
		return account, false, nil
	}

	provisioned, err := s.provision(ctx, connection.WorkspaceID, email, claims.Name)
	if err != nil {
		return entity.Account{}, false, err
	}

	return provisioned, true, nil
}

func (s *connectionsService) provision(
	ctx context.Context,
	workspaceID uuid.UUID,
	email, name string,
) (entity.Account, error) {
	var created entity.Account

	err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		account, err := s.accounts.Create(ctx, entity.Account{
			Status:      entity.AccountStatusActive,
			Kind:        entity.AccountKindPerson,
			Email:       email,
			DisplayName: displayName(name, email),
			Timezone:    entity.DefaultTimezone,
		})
		if err != nil {
			return err
		}

		if _, err := s.memberships.Create(ctx, entity.Membership{
			WorkspaceID: workspaceID,
			AccountID:   account.ID,
			Role:        entity.MembershipRoleMember,
			Source:      entity.MembershipSourceDirectory,
		}); err != nil {
			return err
		}

		created = account

		return nil
	})
	if err != nil {
		return entity.Account{}, entity.OIDCFailure(
			entity.OIDCStageProvisioning,
			"Norn could not create an account for "+email+".",
			err,
		)
	}

	return created, nil
}

func displayName(name, email string) string {
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		return trimmed
	}

	local, _, _ := strings.Cut(email, "@")

	return local
}

func safeReturnTo(target string) string {
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		return ""
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/") {
		return ""
	}

	return parsed.String()
}

func opaque() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate oidc state: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
