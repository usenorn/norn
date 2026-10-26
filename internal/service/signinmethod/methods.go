package signinmethod

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type methodsService struct {
	workspaces  repository.Workspace
	policies    repository.WorkspaceAuthPolicy
	connections repository.SSOConnection
	domains     repository.WorkspaceDomain
	throttle    repository.SignInThrottle
	instance    config.Instance
	now         func() time.Time
	waitUntil   func(ctx context.Context, deadline time.Time) error
}

func New(
	workspaces repository.Workspace,
	policies repository.WorkspaceAuthPolicy,
	connections repository.SSOConnection,
	domains repository.WorkspaceDomain,
	throttle repository.SignInThrottle,
	instance config.Instance,
) service.SignInMethods {
	return &methodsService{
		workspaces:  workspaces,
		policies:    policies,
		connections: connections,
		domains:     domains,
		throttle:    throttle,
		instance:    instance,
		now:         time.Now,
		waitUntil:   waitUntil,
	}
}

func (s *methodsService) ForWorkspace(ctx context.Context, slug string) (entity.WorkspaceSignIn, error) {
	workspace, err := s.workspaces.GetBySlug(ctx, slug)
	if err != nil {
		return entity.WorkspaceSignIn{}, err
	}

	policy, err := s.policyFor(ctx, workspace)
	if err != nil {
		return entity.WorkspaceSignIn{}, err
	}

	return entity.WorkspaceSignIn{
		Slug:          workspace.Slug,
		Name:          workspace.Name,
		SignInOptions: entity.DeriveSignInOptions(s.instance.PasswordAuth, []entity.WorkspaceSignInPolicy{policy}),
	}, nil
}

func (s *methodsService) Discover(
	ctx context.Context,
	input service.DiscoverSignInInput,
) (entity.SignInOptions, error) {
	deadline := s.now().Add(entity.SignInDiscoveryFloor)

	options, err := s.discover(ctx, input)

	if waitErr := s.waitUntil(ctx, deadline); waitErr != nil {
		return entity.SignInOptions{}, waitErr
	}

	return options, err
}

func (s *methodsService) discover(
	ctx context.Context,
	input service.DiscoverSignInInput,
) (entity.SignInOptions, error) {
	attempts, err := s.throttle.RecordAddressAttempt(ctx, input.Client.IP)
	if err != nil {
		return entity.SignInOptions{}, err
	}

	if attempts > entity.SignInAddressMaxAttempts {
		return entity.SignInOptions{}, entity.ErrSignInRateLimited
	}

	email := entity.NormalizeEmail(input.Email)
	if err := entity.NewValidationError(entity.ValidateEmail("email", email)); err != nil {
		return entity.SignInOptions{}, err
	}

	domain, _ := entity.EmailDomain(email)

	workspaceIDs, err := s.domains.VerifiedWorkspaceIDs(ctx, domain)
	if err != nil {
		return entity.SignInOptions{}, err
	}

	policies := make([]entity.WorkspaceSignInPolicy, 0, len(workspaceIDs))

	for _, workspaceID := range workspaceIDs {
		workspace, err := s.workspaces.GetByID(ctx, workspaceID)
		if err != nil {
			return entity.SignInOptions{}, err
		}

		if workspace.Deleted() {
			continue
		}

		policy, err := s.policyFor(ctx, workspace)
		if err != nil {
			return entity.SignInOptions{}, err
		}

		policies = append(policies, policy)
	}

	return entity.DeriveSignInOptions(s.instance.PasswordAuth, policies), nil
}

func (s *methodsService) policyFor(
	ctx context.Context,
	workspace entity.Workspace,
) (entity.WorkspaceSignInPolicy, error) {
	authPolicy, err := s.policies.Get(ctx, workspace.ID)
	if err != nil {
		return entity.WorkspaceSignInPolicy{}, err
	}

	policy := entity.WorkspaceSignInPolicy{
		Workspace:   entity.SignInWorkspace{Slug: workspace.Slug, Name: workspace.Name},
		Enforcement: authPolicy.Enforcement,
	}

	protocol, err := s.connections.Protocol(ctx, workspace.ID)
	if err != nil {
		if errors.Is(err, entity.ErrSSOConnectionNotFound) {
			return policy, nil
		}

		return entity.WorkspaceSignInPolicy{}, err
	}

	address, verified, err := s.provider(ctx, workspace, protocol)
	if err != nil {
		return entity.WorkspaceSignInPolicy{}, err
	}

	policy.Workspace.Protocol = protocol
	policy.Workspace.Host = hostOf(address)
	policy.SSOReady = verified

	return policy, nil
}

func (s *methodsService) provider(
	ctx context.Context,
	workspace entity.Workspace,
	protocol entity.SSOProtocol,
) (string, bool, error) {
	if protocol == entity.SSOProtocolSAML {
		connection, err := s.connections.GetSAML(ctx, workspace.ID)
		if err != nil {
			return "", false, err
		}

		return connection.Descriptor.SSOURL, connection.Verified(), nil
	}

	connection, err := s.connections.GetOIDC(ctx, workspace.ID)
	if err != nil {
		return "", false, err
	}

	return connection.Endpoints.Issuer, connection.Verified(), nil
}

func hostOf(address string) string {
	parsed, err := url.Parse(address)
	if err != nil {
		return ""
	}

	return parsed.Host
}

func waitUntil(ctx context.Context, deadline time.Time) error {
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return nil
	}

	timer := time.NewTimer(remaining)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
