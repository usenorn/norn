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

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/samlprovider"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type connectionsService struct {
	connections repository.SSOConnection
	policies    repository.WorkspaceAuthPolicy
	identities  repository.SSOIdentity
	states      repository.OIDCState
	requests    repository.SAMLRequest
	replays     repository.SAMLReplay
	provider    repository.OIDCProvider
	saml        *samlprovider.Client
	app         config.App
	workspaces  repository.Workspace
	accounts    repository.Account
	memberships repository.Membership
	mailer      repository.Mailer
	sessions    service.Sessions
	authorizer  service.Authorizer
	transactor  repository.Transactor
	audit       service.Audit
}

func New(
	connections repository.SSOConnection,
	policies repository.WorkspaceAuthPolicy,
	identities repository.SSOIdentity,
	states repository.OIDCState,
	requests repository.SAMLRequest,
	replays repository.SAMLReplay,
	workspaces repository.Workspace,
	accounts repository.Account,
	memberships repository.Membership,
	provider repository.OIDCProvider,
	saml *samlprovider.Client,
	mailer repository.Mailer,
	sessions service.Sessions,
	app config.App,
	authorizer service.Authorizer,
	transactor repository.Transactor,
	audit service.Audit,
) service.SSOConnections {
	return &connectionsService{
		connections: connections,
		policies:    policies,
		identities:  identities,
		states:      states,
		requests:    requests,
		replays:     replays,
		workspaces:  workspaces,
		accounts:    accounts,
		memberships: memberships,
		provider:    provider,
		saml:        saml,
		mailer:      mailer,
		sessions:    sessions,
		app:         app,
		authorizer:  authorizer,
		transactor:  transactor,
		audit:       audit,
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

	return s.connections.GetOIDC(ctx, workspaceID)
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

	saved, err := s.connections.SaveOIDC(ctx, connection)
	if err != nil {
		return entity.OIDCConnection{}, err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  input.WorkspaceID,
		Action:       entity.AuditSSOConnectionSaved,
		ResourceKind: string(entity.ResourceSSOConnection),
		ResourceID:   input.WorkspaceID,
		Detail:       map[string]string{"issuer": saved.Endpoints.Issuer},
	})

	return saved, nil
}

func (s *connectionsService) secretFor(
	ctx context.Context,
	input service.SaveOIDCConnectionInput,
) (string, error) {
	if strings.TrimSpace(input.ClientSecret) != "" {
		return input.ClientSecret, nil
	}

	existing, err := s.connections.GetOIDC(ctx, input.WorkspaceID)
	if err != nil {
		if errors.Is(err, entity.ErrSSOConnectionNotFound) {
			return "", entity.NewSSOError(
				entity.SSOStageEndpoints,
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

	if err := s.connections.Delete(ctx, workspaceID); err != nil {
		return err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  workspaceID,
		Action:       entity.AuditSSOConnectionRemoved,
		ResourceKind: string(entity.ResourceSSOConnection),
		ResourceID:   workspaceID,
	})

	return nil
}

func (s *connectionsService) BeginTest(ctx context.Context, workspaceID uuid.UUID) (string, error) {
	if err := s.authorize(ctx, workspaceID, entity.ActionUpdate); err != nil {
		return "", err
	}

	connection, err := s.connections.GetOIDC(ctx, workspaceID)
	if err != nil {
		return "", err
	}

	return s.begin(ctx, connection, entity.SSOPurposeTest, "")
}

func (s *connectionsService) BeginLogin(
	ctx context.Context,
	input service.BeginOIDCLoginInput,
) (string, error) {
	workspace, err := s.workspaces.GetBySlug(ctx, input.WorkspaceSlug)
	if err != nil {
		if errors.Is(err, entity.ErrWorkspaceNotFound) {
			return "", entity.ErrSSOConnectionNotFound
		}

		return "", err
	}

	connection, err := s.connections.GetOIDC(ctx, workspace.ID)
	if err != nil {
		return "", err
	}

	return s.begin(ctx, connection, entity.SSOPurposeLogin, input.ReturnTo)
}

func (s *connectionsService) SignIn(
	ctx context.Context,
	slug string,
) (entity.WorkspaceSignIn, error) {
	workspace, err := s.workspaces.GetBySlug(ctx, slug)
	if err != nil {
		return entity.WorkspaceSignIn{}, err
	}

	policy, err := s.policies.Get(ctx, workspace.ID)
	if err != nil {
		return entity.WorkspaceSignIn{}, err
	}

	signIn := entity.WorkspaceSignIn{
		Slug:     workspace.Slug,
		Name:     workspace.Name,
		Password: policy.Enforcement.Permits(entity.SessionAuthMethodPassword),
	}

	protocol, err := s.connections.Protocol(ctx, workspace.ID)
	if err != nil {
		if errors.Is(err, entity.ErrSSOConnectionNotFound) {
			return signIn, nil
		}

		return entity.WorkspaceSignIn{}, err
	}

	signIn.Protocol = protocol

	host, verified, err := s.providerHost(ctx, workspace.ID, protocol)
	if err != nil {
		return entity.WorkspaceSignIn{}, err
	}

	signIn.SSO = verified
	signIn.Host = host

	return signIn, nil
}

func (s *connectionsService) providerHost(
	ctx context.Context,
	workspaceID uuid.UUID,
	protocol entity.SSOProtocol,
) (string, bool, error) {
	if protocol == entity.SSOProtocolSAML {
		connection, err := s.connections.GetSAML(ctx, workspaceID)
		if err != nil {
			return "", false, err
		}

		return hostOf(connection.Descriptor.SSOURL), connection.Verified(), nil
	}

	connection, err := s.connections.GetOIDC(ctx, workspaceID)
	if err != nil {
		return "", false, err
	}

	return hostOf(connection.Endpoints.Issuer), connection.Verified(), nil
}

func hostOf(address string) string {
	parsed, err := url.Parse(address)
	if err != nil {
		return ""
	}

	return parsed.Host
}

func (s *connectionsService) begin(
	ctx context.Context,
	connection entity.OIDCConnection,
	purpose entity.SSOPurpose,
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
) (entity.SSOExchange, error) {
	attempt, err := s.states.Take(ctx, input.State)
	if err != nil {
		return entity.SSOExchange{}, err
	}

	if !attempt.Purpose.Valid() {
		return entity.SSOExchange{}, entity.ErrSSOStateNotFound
	}

	workspace, err := s.workspaces.GetByID(ctx, attempt.WorkspaceID)
	if err != nil {
		return entity.SSOExchange{}, err
	}

	exchange := entity.SSOExchange{
		Protocol:      entity.SSOProtocolOIDC,
		Purpose:       attempt.Purpose,
		WorkspaceID:   attempt.WorkspaceID,
		WorkspaceSlug: workspace.Slug,
		ReturnTo:      attempt.ReturnTo,
	}

	connection, err := s.connections.GetOIDC(ctx, attempt.WorkspaceID)
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

	exchange.Email = entity.NormalizeEmail(claims.Email)

	if attempt.Purpose == entity.SSOPurposeTest {
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

type admission struct {
	WorkspaceID  uuid.UUID
	Provisioning bool
	Issuer       string
	Subject      string
	Email        string
	Name         string
}

func (s *connectionsService) admit(
	ctx context.Context,
	connection entity.OIDCConnection,
	claims entity.OIDCClaims,
) (entity.Account, bool, error) {
	return s.admitIdentity(ctx, admission{
		WorkspaceID:  connection.WorkspaceID,
		Provisioning: connection.Provisioning,
		Issuer:       connection.Endpoints.Issuer,
		Subject:      claims.Subject,
		Email:        entity.NormalizeEmail(claims.Email),
		Name:         claims.Name,
	})
}

func (s *connectionsService) admitIdentity(
	ctx context.Context,
	request admission,
) (entity.Account, bool, error) {
	linked, err := s.linkedAccount(ctx, request)
	if err != nil {
		return entity.Account{}, false, err
	}

	if linked != nil {
		return *linked, false, nil
	}

	return s.bootstrap(ctx, request)
}

func (s *connectionsService) linkedAccount(
	ctx context.Context,
	request admission,
) (*entity.Account, error) {
	issuer, subject := trimmedPair(request.Issuer, request.Subject)
	if issuer == "" || subject == "" {
		return nil, nil
	}

	identity, err := s.identities.GetBySubject(ctx, request.WorkspaceID, issuer, subject)
	if err != nil {
		if errors.Is(err, entity.ErrSSOIdentityNotFound) {
			return nil, nil
		}

		return nil, err
	}

	account, err := s.accounts.GetByID(ctx, identity.AccountID)
	if err != nil {
		return nil, err
	}

	if account.Status != entity.AccountStatusActive {
		return nil, entity.NewSSOError(
			entity.SSOStageMatching,
			"The Norn account for "+account.Email+" is not active.",
		)
	}

	if _, err := s.memberships.Get(ctx, request.WorkspaceID, account.ID); err != nil {
		if errors.Is(err, entity.ErrMembershipNotFound) {
			return nil, entity.MatchOutcomeNotMember.Refusal(account.Email)
		}

		return nil, err
	}

	return &account, nil
}

func (s *connectionsService) bootstrap(
	ctx context.Context,
	request admission,
) (entity.Account, bool, error) {
	account, err := s.accounts.GetByEmail(ctx, request.Email)
	if err != nil && !errors.Is(err, entity.ErrAccountNotFound) {
		return entity.Account{}, false, err
	}

	exists := err == nil
	member := false

	if exists {
		if account.Status != entity.AccountStatusActive {
			return entity.Account{}, false, entity.NewSSOError(
				entity.SSOStageMatching,
				"The Norn account for "+request.Email+" is not active.",
			)
		}

		if _, err := s.memberships.Get(ctx, request.WorkspaceID, account.ID); err != nil {
			if !errors.Is(err, entity.ErrMembershipNotFound) {
				return entity.Account{}, false, err
			}
		} else {
			member = true
		}
	}

	outcome := entity.ResolveMatch(exists, member, request.Provisioning)
	if !outcome.Admits() {
		return entity.Account{}, false, outcome.Refusal(request.Email)
	}

	if outcome == entity.MatchOutcomeSignIn {
		if err := s.link(ctx, request, account.ID); err != nil {
			return entity.Account{}, false, err
		}

		return account, false, nil
	}

	provisioned, err := s.provision(ctx, request.WorkspaceID, request.Email, request.Name)
	if err != nil {
		return entity.Account{}, false, err
	}

	if err := s.link(ctx, request, provisioned.ID); err != nil {
		return entity.Account{}, false, err
	}

	return provisioned, true, nil
}

func (s *connectionsService) link(
	ctx context.Context,
	request admission,
	accountID uuid.UUID,
) error {
	issuer, subject := trimmedPair(request.Issuer, request.Subject)
	if issuer == "" || subject == "" {
		return nil
	}

	existing, err := s.identities.Get(ctx, request.WorkspaceID, accountID)
	if err != nil && !errors.Is(err, entity.ErrSSOIdentityNotFound) {
		return err
	}

	var linked *entity.SSOIdentity
	if err == nil {
		linked = &existing
	}

	if err := entity.MatchLink(linked, issuer, subject, request.Email); err != nil {
		return err
	}

	if linked != nil && linked.Issuer == issuer && linked.Subject == subject {
		return nil
	}

	return s.identities.Link(ctx, entity.SSOIdentity{
		WorkspaceID: request.WorkspaceID,
		AccountID:   accountID,
		Issuer:      issuer,
		Subject:     subject,
	})
}

func trimmedPair(issuer, subject string) (string, string) {
	return strings.TrimSpace(issuer), strings.TrimSpace(subject)
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
			Source:      entity.MembershipSourceManual,
		}); err != nil {
			return err
		}

		created = account

		return nil
	})
	if err != nil {
		return entity.Account{}, entity.SSOFailure(
			entity.SSOStageProvisioning,
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
