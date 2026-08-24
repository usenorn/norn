package scm

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type apps struct {
	apps          repository.SCMApp
	connections   repository.SCMConnection
	states        repository.SCMAppState
	forges        service.Forges
	authorizer    service.Authorizer
	sourceControl config.SourceControl
}

func NewApps(
	appRepository repository.SCMApp,
	connections repository.SCMConnection,
	states repository.SCMAppState,
	forges service.Forges,
	authorizer service.Authorizer,
	sourceControl config.SourceControl,
) service.SourceControlApps {
	return &apps{
		apps:          appRepository,
		connections:   connections,
		states:        states,
		forges:        forges,
		authorizer:    authorizer,
		sourceControl: sourceControl,
	}
}

func (s *apps) administers(ctx context.Context, workspaceID uuid.UUID) (entity.Decision, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceWorkspace,
		Action:      entity.ActionUpdate,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return entity.Decision{}, err
	}

	if decision.Role != entity.MembershipRoleAdmin {
		return entity.Decision{}, entity.ErrAccountForbidden
	}

	return decision, nil
}

// Application answers only for the forge application this instance holds, never for where it
// happens to be installed. That question is answered per account by the sign-in below: the
// forge's own list spans every workspace on the instance, so reading it here named one
// tenant's organisations on another tenant's screen and ticked a step they had not taken.
func (s *apps) Application(
	ctx context.Context,
	workspaceID uuid.UUID,
	provider entity.SCMProvider,
) (entity.SCMApp, error) {
	if _, err := s.administers(ctx, workspaceID); err != nil {
		return entity.SCMApp{}, err
	}

	return s.application(ctx, provider)
}

func (s *apps) application(
	ctx context.Context,
	provider entity.SCMProvider,
) (entity.SCMApp, error) {
	if !entity.SupportsApp(provider) {
		return entity.SCMApp{}, entity.ErrSCMAppUnsupported
	}

	endpoint := s.endpoint(provider)

	found, err := s.apps.Get(ctx, provider, endpoint)
	if err == nil {
		return found, nil
	}

	if !errors.Is(err, entity.ErrSCMAppNotFound) || !s.sourceControl.GitHubAppConfigured() {
		return entity.SCMApp{}, err
	}

	return s.apps.Upsert(ctx, repository.SCMAppInput{
		App: entity.SCMApp{
			Provider:      provider,
			BaseURL:       endpoint,
			Slug:          s.sourceControl.GitHubAppSlug,
			ExternalAppID: s.sourceControl.GitHubAppID,
			ClientID:      s.sourceControl.GitHubAppClientID,
		},
		PrivateKey:    s.sourceControl.GitHubAppPrivateKey,
		WebhookSecret: s.sourceControl.GitHubAppWebhookSecret,
		ClientSecret:  s.sourceControl.GitHubAppClientSecret,
	})
}

func (s *apps) endpoint(provider entity.SCMProvider) string {
	forge, err := s.forges.Lookup(provider)
	if err != nil {
		return ""
	}

	return forge.Endpoint()
}

// registrable refuses to register over an application anybody is connected through. One row per
// provider and base URL serves the whole instance, so replacing it swaps the private key, the
// client secret and the webhook secret every other workspace's connections already depend on —
// breaking theirs, and handing whoever registered a secret that verifies deliveries into any of
// them. A row nothing is connected through is nobody's, so a registration that went wrong can
// still be started again. The callback that finishes a registration carries no session, so this
// is asked there too rather than only where the exchange begins.
func (s *apps) registrable(ctx context.Context, provider entity.SCMProvider) error {
	if s.sourceControl.GitHubAppConfigured() {
		return entity.ErrSCMAppExists
	}

	held, err := s.apps.Get(ctx, provider, s.endpoint(provider))

	if errors.Is(err, entity.ErrSCMAppNotFound) {
		return nil
	}

	if err != nil {
		return err
	}

	connected, err := s.connections.CountByApp(ctx, held.ID)
	if err != nil {
		return err
	}

	if connected > 0 {
		return entity.ErrSCMAppExists
	}

	return nil
}

func (s *apps) Registration(
	ctx context.Context,
	input service.RegisterSCMAppInput,
) (entity.SCMAppRegistration, error) {
	decision, err := s.administers(ctx, input.WorkspaceID)
	if err != nil {
		return entity.SCMAppRegistration{}, err
	}

	if err := s.registrable(ctx, entity.SCMProviderGitHub); err != nil {
		return entity.SCMAppRegistration{}, err
	}

	forgeApp, err := s.forges.App(entity.SCMProviderGitHub)
	if err != nil {
		return entity.SCMAppRegistration{}, err
	}

	state, err := opaque()
	if err != nil {
		return entity.SCMAppRegistration{}, err
	}

	err = s.states.Put(ctx, state, entity.SCMAppState{
		Purpose:       entity.SCMAppRegister,
		Provider:      entity.SCMProviderGitHub,
		WorkspaceID:   input.WorkspaceID,
		WorkspaceSlug: decision.Workspace.Slug,
		AccountID:     decision.Actor.AccountID,
		Organization:  input.Organization,
		Trust: entity.SCMTrust{
			AllowPrivateAddress: input.AllowPrivateAddress,
			CACertificate:       strings.TrimSpace(input.CACertificate),
		},
		CreatedAt: time.Now(),
	})
	if err != nil {
		return entity.SCMAppRegistration{}, err
	}

	return entity.SCMAppRegistration{
		Target: forgeApp.ManifestTarget(s.endpoint(entity.SCMProviderGitHub), input.Organization),
		State:  state,
		Manifest: entity.SCMAppManifest{
			Name:           input.InstanceName,
			URL:            input.InstanceURL,
			HookAttributes: map[string]string{"url": input.HookURL},
			RedirectURL:    input.RedirectURL,
			CallbackURLs:   []string{input.CallbackURL},
			Public:         false,
			DefaultEvents:  entity.SCMAppEvents(),
			DefaultPermMap: entity.SCMAppPermissions(),
		},
	}, nil
}

func (s *apps) CompleteRegistration(
	ctx context.Context,
	code, state string,
) (entity.SCMAppState, error) {
	attempt, err := s.states.Take(ctx, state)
	if err != nil {
		return entity.SCMAppState{}, err
	}

	if attempt.Purpose != entity.SCMAppRegister {
		return entity.SCMAppState{}, entity.ErrSCMAppStateNotFound
	}

	if err := s.registrable(ctx, attempt.Provider); err != nil {
		return attempt, err
	}

	forgeApp, err := s.forges.App(attempt.Provider)
	if err != nil {
		return attempt, err
	}

	endpoint := s.endpoint(attempt.Provider)

	registered, err := forgeApp.ConvertManifest(ctx, entity.SCMApp{
		BaseURL: endpoint,
		Trust:   attempt.Trust,
	}, code)
	if err != nil {
		return attempt, err
	}

	if _, err := s.apps.Upsert(ctx, repository.SCMAppInput{
		App:           registered,
		PrivateKey:    registered.PrivateKey,
		WebhookSecret: registered.WebhookSecret,
		ClientSecret:  registered.ClientSecret,
	}); err != nil {
		return attempt, err
	}

	return attempt, nil
}

func (s *apps) Authorization(
	ctx context.Context,
	workspaceID uuid.UUID,
	callbackURL string,
) (string, error) {
	decision, err := s.administers(ctx, workspaceID)
	if err != nil {
		return "", err
	}

	registered, err := s.application(ctx, entity.SCMProviderGitHub)
	if err != nil {
		return "", err
	}

	if registered.ClientID == "" {
		return "", entity.ErrSCMAppNotFound
	}

	forgeApp, err := s.forges.App(entity.SCMProviderGitHub)
	if err != nil {
		return "", err
	}

	state, err := opaque()
	if err != nil {
		return "", err
	}

	err = s.states.Put(ctx, state, entity.SCMAppState{
		Purpose:       entity.SCMAppConnect,
		Provider:      entity.SCMProviderGitHub,
		WorkspaceID:   workspaceID,
		WorkspaceSlug: decision.Workspace.Slug,
		AccountID:     decision.Actor.AccountID,
		CreatedAt:     time.Now(),
	})
	if err != nil {
		return "", err
	}

	return forgeApp.AuthorizeURL(registered, state, callbackURL), nil
}

func (s *apps) CompleteAuthorization(
	ctx context.Context,
	code, state, callbackURL string,
) (service.SCMAppChoice, error) {
	attempt, err := s.states.Take(ctx, state)
	if err != nil {
		return service.SCMAppChoice{}, err
	}

	if attempt.Purpose != entity.SCMAppConnect {
		return service.SCMAppChoice{}, entity.ErrSCMAppStateNotFound
	}

	choice := service.SCMAppChoice{
		WorkspaceID:   attempt.WorkspaceID,
		WorkspaceSlug: attempt.WorkspaceSlug,
	}

	registered, err := s.application(ctx, attempt.Provider)
	if err != nil {
		return choice, err
	}

	secrets, err := s.apps.Secrets(ctx, registered.ID)
	if err != nil {
		return choice, err
	}

	forgeApp, err := s.forges.App(attempt.Provider)
	if err != nil {
		return choice, err
	}

	token, err := forgeApp.ExchangeCode(ctx, secrets, code, callbackURL)
	if err != nil {
		return choice, err
	}

	installations, err := forgeApp.Installations(ctx, secrets, token)
	if err != nil {
		return choice, err
	}

	handle, err := opaque()
	if err != nil {
		return choice, err
	}

	err = s.states.Put(ctx, handle, entity.SCMAppState{
		Purpose:       entity.SCMAppChosen,
		Provider:      attempt.Provider,
		WorkspaceID:   attempt.WorkspaceID,
		WorkspaceSlug: attempt.WorkspaceSlug,
		AccountID:     attempt.AccountID,
		Installations: installations,
		CreatedAt:     time.Now(),
	})
	if err != nil {
		return choice, err
	}

	choice.Handle = handle
	choice.Installations = installations

	return choice, nil
}

func (s *apps) Choice(
	ctx context.Context,
	workspaceID uuid.UUID,
	handle string,
) (service.SCMAppChoice, error) {
	decision, err := s.administers(ctx, workspaceID)
	if err != nil {
		return service.SCMAppChoice{}, err
	}

	held, err := s.states.Read(ctx, handle)
	if err != nil {
		return service.SCMAppChoice{}, err
	}

	// What is stashed is one person's own forge installations, not the workspace's, so sharing
	// an administrator's workspace is not enough to read it.
	if held.Purpose != entity.SCMAppChosen ||
		held.WorkspaceID != workspaceID ||
		held.AccountID != decision.Actor.AccountID {
		return service.SCMAppChoice{}, entity.ErrSCMAppStateNotFound
	}

	return service.SCMAppChoice{
		Handle:        handle,
		WorkspaceID:   held.WorkspaceID,
		WorkspaceSlug: held.WorkspaceSlug,
		Installations: held.Installations,
	}, nil
}

func opaque() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate source control application state: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
