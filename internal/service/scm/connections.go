package scm

import (
	"context"
	"errors"
	"fmt"
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
	connections repository.SCMConnection
	links       repository.CodeLink
	mirrors     repository.IssueMirror
	settings    repository.SCMTeamSetting
	states      repository.WorkflowState
	accounts    repository.Account
	issues      repository.Issue
	activity    repository.Activity
	forges      service.Forges
	authorizer  service.Authorizer
	audit       service.Audit
	transactor  repository.Transactor
	app         config.App
}

func NewConnections(
	connectionRepository repository.SCMConnection,
	links repository.CodeLink,
	mirrors repository.IssueMirror,
	settings repository.SCMTeamSetting,
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
		connections: connectionRepository,
		links:       links,
		mirrors:     mirrors,
		settings:    settings,
		states:      states,
		accounts:    accounts,
		issues:      issues,
		activity:    activity,
		forges:      forges,
		authorizer:  authorizer,
		audit:       audit,
		transactor:  transactor,
		app:         app,
	}
}

func (s *connections) administers(
	ctx context.Context,
	workspaceID uuid.UUID,
) (entity.Decision, error) {
	// Scoped, because every caller of this goes on to ask whether a team is within reach.
	// Without it the decision carries an empty scope, which covers no team at all and turns
	// "attach this to Engineering" into "there is no such team".
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

func (s *connections) List(
	ctx context.Context,
	workspaceID uuid.UUID,
) ([]entity.SCMConnection, error) {
	if _, err := s.administers(ctx, workspaceID); err != nil {
		return nil, err
	}

	return s.connections.ListByWorkspace(ctx, workspaceID)
}

func (s *connections) Get(
	ctx context.Context,
	workspaceID, connectionID uuid.UUID,
) (entity.SCMConnection, error) {
	if _, err := s.administers(ctx, workspaceID); err != nil {
		return entity.SCMConnection{}, err
	}

	return s.connections.GetByID(ctx, workspaceID, connectionID)
}

func validateConnect(input service.ConnectSourceControlInput) error {
	fields := make([]entity.FieldError, 0, 4)

	for _, field := range []entity.FieldError{
		entity.ValidateSCMRepository("repository", input.Repository),
		entity.ValidateSCMBaseURL("baseUrl", input.BaseURL),
		entity.ValidateSCMToken("token", input.Token),
		entity.ValidateSCMMirrorLabel("mirrorLabel", input.MirrorLabel),
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

	if len(fields) > 0 {
		return entity.ValidationError{Fields: fields}
	}

	return nil
}

// Connect proves the token before anything is stored. A connection that exists but was never
// reachable is indistinguishable on the screen from one whose credentials expired, and the
// person who just pasted a token is the only one who can fix it while they are still here.
func (s *connections) Connect(
	ctx context.Context,
	input service.ConnectSourceControlInput,
) (service.ConnectedSourceControl, error) {
	decision, err := s.administers(ctx, input.WorkspaceID)
	if err != nil {
		return service.ConnectedSourceControl{}, err
	}

	if input.MirrorLabel == "" {
		input.MirrorLabel = defaultMirrorLabel
	}

	if err := validateConnect(input); err != nil {
		return service.ConnectedSourceControl{}, err
	}

	if input.TeamID != uuid.Nil && !decision.Scope.Covers(input.TeamID) {
		return service.ConnectedSourceControl{}, entity.ErrTeamNotFound
	}

	forge, err := s.forges.Lookup(input.Provider)
	if err != nil {
		return service.ConnectedSourceControl{}, err
	}

	repositoryName := entity.NormalizeSCMRepository(input.Repository)
	target := entity.SCMTarget{
		Provider:   input.Provider,
		BaseURL:    strings.TrimSpace(input.BaseURL),
		Repository: repositoryName,
		Token:      strings.TrimSpace(input.Token),
	}

	found, err := forge.Repository(ctx, target)
	if err != nil {
		return service.ConnectedSourceControl{}, err
	}

	login, err := forge.Identity(ctx, target)
	if err != nil {
		return service.ConnectedSourceControl{}, err
	}

	secret, err := entity.NewSCMWebhookSecret()
	if err != nil {
		return service.ConnectedSourceControl{}, fmt.Errorf("mint a webhook secret: %w", err)
	}

	var created entity.SCMConnection

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		account, err := s.accounts.Create(ctx, entity.Account{
			Status:      entity.AccountStatusActive,
			Kind:        entity.AccountKindIntegration,
			DisplayName: entity.IntegrationAccountName(input.Provider, repositoryName),
			Timezone:    decision.Workspace.Timezone,
		})
		if err != nil {
			return err
		}

		stored, err := s.connections.Create(ctx, repository.SCMConnectionInput{
			Connection: entity.SCMConnection{
				WorkspaceID:          input.WorkspaceID,
				TeamID:               input.TeamID,
				Provider:             input.Provider,
				BaseURL:              target.BaseURL,
				Repository:           repositoryName,
				ExternalRepositoryID: found.ExternalID,
				TokenHint:            entity.SCMTokenHint(target.Token),
				IdentityLogin:        login,
				IntegrationAccountID: account.ID,
				OwnerAccountID:       decision.Actor.Authority(),
				OwnerActorKind:       entity.ActorKindToken,
				OwnerAuthMethod:      decision.Actor.AuthMethod,
				MirrorLabel:          strings.TrimSpace(input.MirrorLabel),
			},
			Token:         target.Token,
			WebhookSecret: secret,
		})
		if err != nil {
			return err
		}

		created = stored

		return nil
	}); err != nil {
		return service.ConnectedSourceControl{}, err
	}

	callback := s.callbackURL(created)

	// The hook is installed after the row exists rather than before. Installing first and
	// failing to store leaves a hook on the forge pointing at a connection that never
	// existed, delivering into nothing with no record able to remove it.
	hookID, err := forge.InstallHook(ctx, service.ForgeHookRequest{
		Target:      target,
		CallbackURL: callback,
		Secret:      secret,
	})
	if err != nil {
		logging.From(ctx).WarnContext(
			ctx,
			"installing the source control hook failed; the reconcile sweep will retry it",
			"connection_id", created.ID.String(),
			"error", err.Error(),
		)
	} else if err := s.connections.RecordHook(ctx, created.ID, hookID); err != nil {
		return service.ConnectedSourceControl{}, err
	} else {
		created.ExternalHookID = hookID
	}

	now := time.Now().UTC()

	if err := s.connections.MarkVerified(ctx, created.ID, found, login, now); err != nil {
		return service.ConnectedSourceControl{}, err
	}

	created.VerifiedAt = &now

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  input.WorkspaceID,
		Action:       entity.AuditSourceControlConnected,
		ResourceKind: "scm_connection",
		ResourceID:   created.ID,
		ResourceName: created.Repository,
		Detail: map[string]string{
			"provider":   string(created.Provider),
			"repository": created.Repository,
		},
	})

	return service.ConnectedSourceControl{
		Connection:    created,
		WebhookURL:    callback,
		WebhookSecret: secret,
	}, nil
}

const defaultMirrorLabel = "norn"

func (s *connections) callbackURL(connection entity.SCMConnection) string {
	return strings.TrimRight(s.app.BaseURL, "/") +
		"/v1/source-control/" + string(connection.Provider) + "/" + connection.ID.String()
}

func (s *connections) Update(
	ctx context.Context,
	workspaceID, connectionID uuid.UUID,
	input service.UpdateSourceControlInput,
) (entity.SCMConnection, error) {
	decision, err := s.administers(ctx, workspaceID)
	if err != nil {
		return entity.SCMConnection{}, err
	}

	connection, err := s.connections.GetByID(ctx, workspaceID, connectionID)
	if err != nil {
		return entity.SCMConnection{}, err
	}

	teamID := connection.TeamID

	switch {
	case input.ClearTeam:
		teamID = uuid.Nil
	case input.TeamID != uuid.Nil:
		if !decision.Scope.Covers(input.TeamID) {
			return entity.SCMConnection{}, entity.ErrTeamNotFound
		}

		teamID = input.TeamID
	}

	label := connection.MirrorLabel
	if input.MirrorLabel != "" {
		if field := entity.ValidateSCMMirrorLabel("mirrorLabel", input.MirrorLabel); field.Field != "" {
			return entity.SCMConnection{}, entity.ValidationError{Fields: []entity.FieldError{field}}
		}

		label = strings.TrimSpace(input.MirrorLabel)
	}

	return s.connections.UpdateSettings(ctx, connectionID, teamID, label)
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

	target := connection.Target(strings.TrimSpace(token))

	// The replacement is proved before it is stored, and a failure leaves the connection
	// broken. A repair that reports success without working is worse than no repair: the
	// screen stops saying anything is wrong and nobody looks again.
	if _, err := forge.Repository(ctx, target); err != nil {
		return entity.SCMConnection{}, err
	}

	login, err := forge.Identity(ctx, target)
	if err != nil {
		return entity.SCMConnection{}, err
	}

	now := time.Now().UTC()

	if err := s.connections.ReplaceToken(
		ctx,
		connectionID,
		target.Token,
		entity.SCMTokenHint(target.Token),
		login,
		now,
	); err != nil {
		return entity.SCMConnection{}, err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  workspaceID,
		Action:       entity.AuditSourceControlTokenReplaced,
		ResourceKind: "scm_connection",
		ResourceID:   connectionID,
		ResourceName: connection.Repository,
	})

	return s.connections.GetByID(ctx, workspaceID, connectionID)
}

func (s *connections) Verify(
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

	credentials, err := s.connections.Credentials(ctx, connectionID)
	if err != nil {
		return entity.SCMConnection{}, err
	}

	forge, err := s.forges.Lookup(connection.Provider)
	if err != nil {
		return entity.SCMConnection{}, err
	}

	target := connection.Target(credentials.Token)

	found, err := forge.Repository(ctx, target)
	if err != nil {
		s.breakOn(ctx, connection, err)

		return entity.SCMConnection{}, err
	}

	login, err := forge.Identity(ctx, target)
	if err != nil {
		s.breakOn(ctx, connection, err)

		return entity.SCMConnection{}, err
	}

	if err := s.connections.MarkVerified(
		ctx,
		connectionID,
		found,
		login,
		time.Now().UTC(),
	); err != nil {
		return entity.SCMConnection{}, err
	}

	return s.connections.GetByID(ctx, workspaceID, connectionID)
}

// breakOn records only the failures that mean a person has to act. An outage is not a broken
// connection, and marking one would ask somebody to repair something that was never broken.
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
		ResourceName: connection.Repository,
		Detail:       map[string]string{"reason": string(reason)},
	})
}

// Disconnect cuts every link and mirror loose before deleting the connection, so what a
// person already saw on an issue stays exactly as readable and the sealed credential this
// instance was asked to stop using is destroyed rather than kept.
func (s *connections) Disconnect(ctx context.Context, workspaceID, connectionID uuid.UUID) error {
	if _, err := s.administers(ctx, workspaceID); err != nil {
		return err
	}

	connection, err := s.connections.GetByID(ctx, workspaceID, connectionID)
	if err != nil {
		return err
	}

	if credentials, err := s.connections.Credentials(ctx, connectionID); err == nil &&
		connection.ExternalHookID != "" {
		if forge, err := s.forges.Lookup(connection.Provider); err == nil {
			if err := forge.RemoveHook(
				ctx,
				connection.Target(credentials.Token),
				connection.ExternalHookID,
			); err != nil {
				logging.From(ctx).WarnContext(
					ctx,
					"removing the source control hook failed; it will keep delivering until "+
						"somebody removes it on the forge",
					"connection_id", connectionID.String(),
					"error", err.Error(),
				)
			}
		}
	}

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.links.DetachConnection(ctx, connectionID); err != nil {
			return err
		}

		if err := s.mirrors.DetachConnection(ctx, connectionID); err != nil {
			return err
		}

		if err := s.mirrors.DetachConnectionComments(ctx, connectionID); err != nil {
			return err
		}

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
		ResourceName: connection.Repository,
	})

	return nil
}

// retireIntegrationAccount deactivates rather than deletes. Every comment the connection
// authored still names it, so removing the account would leave that content with no author.
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

func (s *connections) TeamSettings(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
) (service.TeamSourceControlSettings, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceTeam,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return service.TeamSourceControlSettings{}, err
	}

	if !decision.Scope.Covers(teamID) {
		return service.TeamSourceControlSettings{}, entity.ErrTeamNotFound
	}

	settings, err := s.settings.Settings(ctx, workspaceID, teamID)
	if errors.Is(err, entity.ErrSCMTeamSettingsNotFound) {
		return service.TeamSourceControlSettings{
			Settings: entity.SCMTeamSettings{WorkspaceID: workspaceID, TeamID: teamID},
		}, nil
	}

	if err != nil {
		return service.TeamSourceControlSettings{}, err
	}

	return s.resolve(ctx, settings)
}

// resolve reports whether the state a team chose still exists. A team that pointed merged
// work at a state somebody later deleted would otherwise look configured and quietly do
// nothing, which is the failure this whole feature is meant not to have.
func (s *connections) resolve(
	ctx context.Context,
	settings entity.SCMTeamSettings,
) (service.TeamSourceControlSettings, error) {
	answer := service.TeamSourceControlSettings{Settings: settings}

	if !settings.AdvanceOnMerge {
		return answer, nil
	}

	states, err := s.states.ListByTeamID(ctx, settings.TeamID)
	if err != nil {
		return service.TeamSourceControlSettings{}, err
	}

	if target, found := settings.TargetState(states); found {
		answer.TargetResolved = true
		answer.TargetName = target.Name
	}

	return answer, nil
}

func (s *connections) SetTeamSettings(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
	input service.SetTeamSourceControlInput,
) (service.TeamSourceControlSettings, error) {
	decision, err := s.administers(ctx, workspaceID)
	if err != nil {
		return service.TeamSourceControlSettings{}, err
	}

	if !decision.Scope.Covers(teamID) {
		return service.TeamSourceControlSettings{}, entity.ErrTeamNotFound
	}

	stateID := input.MergedStateID
	if input.ClearState {
		stateID = uuid.Nil
	}

	if stateID != uuid.Nil {
		states, err := s.states.ListByTeamID(ctx, teamID)
		if err != nil {
			return service.TeamSourceControlSettings{}, err
		}

		known := false

		for _, state := range states {
			if state.ID == stateID {
				known = true

				break
			}
		}

		if !known {
			return service.TeamSourceControlSettings{}, entity.ValidationError{
				Fields: []entity.FieldError{{
					Field: "mergedStateId",
					Code:  entity.ValidationCodeUnsupportedValue,
				}},
			}
		}
	}

	stored, err := s.settings.Upsert(ctx, entity.SCMTeamSettings{
		TeamID:         teamID,
		WorkspaceID:    workspaceID,
		AdvanceOnMerge: input.AdvanceOnMerge,
		MergedStateID:  stateID,
	})
	if err != nil {
		return service.TeamSourceControlSettings{}, err
	}

	return s.resolve(ctx, stored)
}

func logWarn(ctx context.Context, message string, id uuid.UUID, err error) {
	logging.From(ctx).WarnContext(ctx, message, "id", id.String(), "error", err.Error())
}
