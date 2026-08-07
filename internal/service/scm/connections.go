package scm

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type connections struct {
	connections  repository.SCMConnection
	repositories repository.SCMRepository
	routes       repository.SCMRoute
	rules        repository.SCMTransitionRule
	teamSettings repository.SCMTeamSetting
	identities   repository.SCMIdentity
	conflicts    repository.MirrorConflict
	memberships  repository.Membership
	deliveries   repository.SCMDelivery
	links        repository.CodeLink
	mirrors      repository.IssueMirror
	states       repository.WorkflowState
	accounts     repository.Account
	issues       repository.Issue
	activity     repository.Activity
	forges       service.Forges
	authorizer   service.Authorizer
	audit        service.Audit
	transactor   repository.Transactor
	app          config.App
}

func NewConnections(
	connectionRepository repository.SCMConnection,
	repositories repository.SCMRepository,
	routes repository.SCMRoute,
	rules repository.SCMTransitionRule,
	teamSettings repository.SCMTeamSetting,
	identities repository.SCMIdentity,
	conflicts repository.MirrorConflict,
	memberships repository.Membership,
	deliveries repository.SCMDelivery,
	links repository.CodeLink,
	mirrors repository.IssueMirror,
	states repository.WorkflowState,
	accounts repository.Account,
	issues repository.Issue,
	activity repository.Activity,
	forges service.Forges,
	authorizer service.Authorizer,
	audit service.Audit,
	transactor repository.Transactor,
	app config.App,
) service.SourceControl {
	return &connections{
		connections:  connectionRepository,
		repositories: repositories,
		routes:       routes,
		rules:        rules,
		teamSettings: teamSettings,
		identities:   identities,
		conflicts:    conflicts,
		memberships:  memberships,
		deliveries:   deliveries,
		links:        links,
		mirrors:      mirrors,
		states:       states,
		accounts:     accounts,
		issues:       issues,
		activity:     activity,
		forges:       forges,
		authorizer:   authorizer,
		audit:        audit,
		transactor:   transactor,
		app:          app,
	}
}

func (s *connections) administers(
	ctx context.Context,
	workspaceID uuid.UUID,
) (entity.Decision, error) {
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

func (s *connections) ListConnections(
	ctx context.Context,
	workspaceID uuid.UUID,
) ([]entity.SCMConnection, error) {
	if _, err := s.administers(ctx, workspaceID); err != nil {
		return nil, err
	}

	return s.connections.ListByWorkspace(ctx, workspaceID)
}

func (s *connections) GetConnection(
	ctx context.Context,
	workspaceID, connectionID uuid.UUID,
) (entity.SCMConnection, error) {
	if _, err := s.administers(ctx, workspaceID); err != nil {
		return entity.SCMConnection{}, err
	}

	return s.connections.GetByID(ctx, workspaceID, connectionID)
}

func validateConnect(input service.ConnectSourceControlInput) error {
	fields := make([]entity.FieldError, 0, 3)

	for _, field := range []entity.FieldError{
		entity.ValidateSCMBaseURL("baseUrl", input.BaseURL),
		entity.ValidateSCMToken("token", input.Token),
		entity.ValidateSCMLabel("label", input.Label),
		entity.ValidateSCMCertificate("caCertificate", input.CACertificate),
	} {
		if field.Field != "" {
			fields = append(fields, field)
		}
	}

	if !input.Provider.Valid() {
		fields = append(fields, entity.FieldError{
			Field: "provider",
			Code:  entity.ValidationCodeUnsupportedValue,
		})
	}

	if input.Provider == entity.SCMProviderGitea && strings.TrimSpace(input.BaseURL) == "" {
		fields = append(fields, entity.FieldError{
			Field: "baseUrl",
			Code:  entity.ValidationCodeRequired,
		})
	}

	if len(fields) > 0 {
		return entity.ValidationError{Fields: fields}
	}

	return nil
}

func (s *connections) Connect(
	ctx context.Context,
	input service.ConnectSourceControlInput,
) (entity.SCMConnection, error) {
	decision, err := s.administers(ctx, input.WorkspaceID)
	if err != nil {
		return entity.SCMConnection{}, err
	}

	if err := validateConnect(input); err != nil {
		return entity.SCMConnection{}, err
	}

	forge, err := s.forges.Lookup(input.Provider)
	if err != nil {
		return entity.SCMConnection{}, err
	}

	trust := entity.SCMTrust{
		AllowPrivateAddress: input.AllowPrivateAddress,
		CACertificate:       strings.TrimSpace(input.CACertificate),
	}

	target := entity.SCMTarget{
		Provider: input.Provider,
		BaseURL:  strings.TrimSpace(input.BaseURL),
		Token:    strings.TrimSpace(input.Token),
		Trust:    trust,
	}

	login, err := forge.Identity(ctx, target)
	if err != nil {
		return entity.SCMConnection{}, err
	}

	label := strings.TrimSpace(input.Label)
	if label == "" {
		label = login
	}

	var created entity.SCMConnection

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		account, err := s.accounts.Create(ctx, entity.Account{
			Status:      entity.AccountStatusActive,
			Kind:        entity.AccountKindIntegration,
			DisplayName: entity.IntegrationAccountName(input.Provider, label),
			Timezone:    decision.Workspace.Timezone,
		})
		if err != nil {
			return err
		}

		stored, err := s.connections.Create(ctx, repository.SCMConnectionInput{
			Connection: entity.SCMConnection{
				WorkspaceID:          input.WorkspaceID,
				Provider:             input.Provider,
				BaseURL:              target.BaseURL,
				Label:                label,
				TokenHint:            entity.SCMTokenHint(target.Token),
				IdentityLogin:        login,
				IntegrationAccountID: account.ID,
				OwnerAccountID:       decision.Actor.Authority(),
				OwnerActorKind:       entity.ActorKindToken,
				OwnerAuthMethod:      decision.Actor.AuthMethod,
				Trust:                trust,
				Capabilities:         forge.Capabilities(),
			},
			Token: target.Token,
		})
		if err != nil {
			return err
		}

		created = stored

		return nil
	}); err != nil {
		return entity.SCMConnection{}, err
	}

	now := time.Now().UTC()

	if err := s.connections.MarkVerified(
		ctx, created.ID, login, forge.Capabilities(), now,
	); err != nil {
		return entity.SCMConnection{}, err
	}

	created.VerifiedAt = &now

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  input.WorkspaceID,
		Action:       entity.AuditSourceControlConnected,
		ResourceKind: "scm_connection",
		ResourceID:   created.ID,
		ResourceName: created.DisplayName(),
		Detail: map[string]string{
			"provider": string(created.Provider),
			"identity": login,
		},
	})

	return created, nil
}

func (s *connections) UpdateConnection(
	ctx context.Context,
	workspaceID, connectionID uuid.UUID,
	input service.UpdateConnectionInput,
) (entity.SCMConnection, error) {
	if _, err := s.administers(ctx, workspaceID); err != nil {
		return entity.SCMConnection{}, err
	}

	if _, err := s.connections.GetByID(ctx, workspaceID, connectionID); err != nil {
		return entity.SCMConnection{}, err
	}

	if field := entity.ValidateSCMLabel("label", input.Label); field.Field != "" {
		return entity.SCMConnection{}, entity.ValidationError{Fields: []entity.FieldError{field}}
	}

	return s.connections.UpdateLabel(ctx, connectionID, strings.TrimSpace(input.Label))
}

func (s *connections) ReplaceToken(
	ctx context.Context,
	workspaceID, connectionID uuid.UUID,
	token string,
) (entity.SCMConnection, error) {
	if _, err := s.administers(ctx, workspaceID); err != nil {
		return entity.SCMConnection{}, err
	}

	if field := entity.ValidateSCMToken("token", token); field.Field != "" {
		return entity.SCMConnection{}, entity.ValidationError{Fields: []entity.FieldError{field}}
	}

	connection, err := s.connections.GetByID(ctx, workspaceID, connectionID)
	if err != nil {
		return entity.SCMConnection{}, err
	}

	forge, err := s.forges.Lookup(connection.Provider)
	if err != nil {
		return entity.SCMConnection{}, err
	}

	target := connection.Target("", strings.TrimSpace(token))

	login, err := forge.Identity(ctx, target)
	if err != nil {
		return entity.SCMConnection{}, err
	}

	if err := s.connections.ReplaceToken(
		ctx,
		connectionID,
		target.Token,
		entity.SCMTokenHint(target.Token),
		login,
		time.Now().UTC(),
	); err != nil {
		return entity.SCMConnection{}, err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  workspaceID,
		Action:       entity.AuditSourceControlTokenReplaced,
		ResourceKind: "scm_connection",
		ResourceID:   connectionID,
		ResourceName: connection.DisplayName(),
	})

	return s.connections.GetByID(ctx, workspaceID, connectionID)
}

func (s *connections) VerifyConnection(
	ctx context.Context,
	workspaceID, connectionID uuid.UUID,
) (entity.SCMConnection, error) {
	if _, err := s.administers(ctx, workspaceID); err != nil {
		return entity.SCMConnection{}, err
	}

	connection, err := s.connections.GetByID(ctx, workspaceID, connectionID)
	if err != nil {
		return entity.SCMConnection{}, err
	}

	token, err := s.connections.Token(ctx, connectionID)
	if err != nil {
		return entity.SCMConnection{}, err
	}

	forge, err := s.forges.Lookup(connection.Provider)
	if err != nil {
		return entity.SCMConnection{}, err
	}

	login, err := forge.Identity(ctx, connection.Target("", token))
	if err != nil {
		s.breakOn(ctx, connection, err)

		return entity.SCMConnection{}, err
	}

	stored, err := s.repositories.ListByConnection(ctx, connectionID)
	if err != nil {
		return entity.SCMConnection{}, err
	}

	for _, one := range stored {
		if _, err := forge.Repository(ctx, connection.Target(one.FullName, token)); err != nil {
			s.breakOn(ctx, connection, err)

			return entity.SCMConnection{}, err
		}
	}

	if err := s.connections.MarkVerified(
		ctx, connectionID, login, forge.Capabilities(), time.Now().UTC(),
	); err != nil {
		return entity.SCMConnection{}, err
	}

	return s.connections.GetByID(ctx, workspaceID, connectionID)
}

func (s *connections) breakOn(ctx context.Context, connection entity.SCMConnection, cause error) {
	reason, detail, actionable := entity.SCMBrokenBy(cause)
	if !actionable {
		return
	}

	if err := s.connections.MarkBroken(
		ctx,
		connection.ID,
		reason,
		detail,
		time.Now().UTC(),
	); err != nil {
		logging.From(ctx).ErrorContext(
			ctx,
			"recording a broken source control connection failed",
			"connection_id", connection.ID.String(),
			"error", err.Error(),
		)

		return
	}

	if connection.Broken() {
		return
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  connection.WorkspaceID,
		Action:       entity.AuditSourceControlBroken,
		ResourceKind: "scm_connection",
		ResourceID:   connection.ID,
		ResourceName: connection.DisplayName(),
		Detail:       map[string]string{"reason": string(reason)},
	})
}

func (s *connections) Disconnect(ctx context.Context, workspaceID, connectionID uuid.UUID) error {
	if _, err := s.administers(ctx, workspaceID); err != nil {
		return err
	}

	connection, err := s.connections.GetByID(ctx, workspaceID, connectionID)
	if err != nil {
		return err
	}

	stored, err := s.repositories.ListByConnection(ctx, connectionID)
	if err != nil {
		return err
	}

	for _, one := range stored {
		if err := s.removeRepository(ctx, connection, one); err != nil {
			return err
		}
	}

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.retireIntegrationAccount(ctx, connection.IntegrationAccountID); err != nil {
			return err
		}

		return s.connections.Delete(ctx, connectionID)
	}); err != nil {
		return err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  workspaceID,
		Action:       entity.AuditSourceControlDisconnected,
		ResourceKind: "scm_connection",
		ResourceID:   connectionID,
		ResourceName: connection.DisplayName(),
	})

	return nil
}

func (s *connections) retireIntegrationAccount(ctx context.Context, accountID uuid.UUID) error {
	account, err := s.accounts.GetByID(ctx, accountID)
	if err != nil {
		return err
	}

	if account.Status == entity.AccountStatusDeactivated {
		return nil
	}

	now := time.Now().UTC()
	account.Status = entity.AccountStatusDeactivated
	account.DeactivatedAt = &now

	_, err = s.accounts.Update(ctx, account)

	return err
}

func logWarn(ctx context.Context, message string, id uuid.UUID, err error) {
	logging.From(ctx).WarnContext(ctx, message, "id", id.String(), "error", err.Error())
}
