package scm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

const (
	defaultMirrorLabel = "norn"
	deliveryLogSize    = 50
	minPollInterval    = time.Minute
	maxPollInterval    = 24 * time.Hour
)

func (s *connections) ListRepositories(
	ctx context.Context,
	workspaceID, connectionID uuid.UUID,
) ([]entity.SCMRepository, error) {
	if _, err := s.administers(ctx, workspaceID); err != nil {
		return nil, err
	}

	if connectionID == uuid.Nil {
		return s.repositories.ListByWorkspace(ctx, workspaceID)
	}

	if _, err := s.connections.GetByID(ctx, workspaceID, connectionID); err != nil {
		return nil, err
	}

	return s.repositories.ListByConnection(ctx, connectionID)
}

func validateRepository(input service.AddRepositoryInput) error {
	fields := make([]entity.FieldError, 0, 2)

	for _, field := range []entity.FieldError{
		entity.ValidateSCMRepository("fullName", input.FullName),
		entity.ValidateSCMMirrorLabel("mirrorLabel", input.MirrorLabel),
	} {
		if field.Field != "" {
			fields = append(fields, field)
		}
	}

	if input.PollInterval != 0 &&
		(input.PollInterval < minPollInterval || input.PollInterval > maxPollInterval) {
		fields = append(fields, entity.FieldError{
			Field: "pollInterval",
			Code:  entity.ValidationCodeOutOfRange,
		})
	}

	if len(fields) > 0 {
		return entity.ValidationError{Fields: fields}
	}

	return nil
}

func (s *connections) AddRepository(
	ctx context.Context,
	workspaceID uuid.UUID,
	input service.AddRepositoryInput,
) (service.ConnectedRepository, error) {
	if _, err := s.administers(ctx, workspaceID); err != nil {
		return service.ConnectedRepository{}, err
	}

	if input.MirrorLabel == "" {
		input.MirrorLabel = defaultMirrorLabel
	}

	if err := validateRepository(input); err != nil {
		return service.ConnectedRepository{}, err
	}

	connection, err := s.connections.GetByID(ctx, workspaceID, input.ConnectionID)
	if err != nil {
		return service.ConnectedRepository{}, err
	}

	token, err := s.credentials.refresh(ctx, connection)
	if err != nil {
		return service.ConnectedRepository{}, err
	}

	forge, err := s.forges.Lookup(connection.Provider)
	if err != nil {
		return service.ConnectedRepository{}, err
	}

	fullName := entity.NormalizeSCMRepository(input.FullName)
	target := connection.Target(fullName, token)

	found, err := forge.Repository(ctx, target)
	if err != nil {
		// A repository this connection was never granted says nothing about the credential, so
		// only a refused one breaks the connection. Reading it the other way let one name the
		// installation cannot reach stop every repository already connected through it.
		var rejected entity.SCMCredentialsRejectedError
		if errors.As(err, &rejected) {
			s.breakOn(ctx, connection, err)
		}

		return service.ConnectedRepository{}, err
	}

	central := connection.DeliversCentrally()

	var secret string

	if !central {
		if secret, err = entity.NewSCMWebhookSecret(); err != nil {
			return service.ConnectedRepository{}, fmt.Errorf("mint a webhook secret: %w", err)
		}
	}

	created, err := s.repositories.Create(ctx, repository.SCMRepositoryInput{
		Repository: entity.SCMRepository{
			ConnectionID:  connection.ID,
			WorkspaceID:   workspaceID,
			Provider:      connection.Provider,
			FullName:      fullName,
			ExternalID:    found.ExternalID,
			DefaultBranch: found.DefaultBranch,
			URL:           found.URL,
			MirrorLabel:   strings.TrimSpace(input.MirrorLabel),
			PollInterval:  input.PollInterval,
		},
		WebhookSecret: secret,
	})
	if err != nil {
		return service.ConnectedRepository{}, err
	}

	callback := s.callbackURL(created)

	if !central {
		hookID, err := forge.InstallHook(ctx, service.ForgeHookRequest{
			Target:      target,
			CallbackURL: callback,
			Secret:      secret,
		})

		switch {
		case err != nil:
			logging.From(ctx).WarnContext(
				ctx,
				"installing the source control hook failed; the reconcile sweep will retry it",
				"repository_id", created.ID.String(),
				"error", err.Error(),
			)
		default:
			if err := s.repositories.RecordHook(ctx, created.ID, hookID); err != nil {
				return service.ConnectedRepository{}, err
			}

			created.ExternalHookID = hookID
		}
	}

	if err := s.jobs.EnqueueSCMBackfill(ctx, entity.SCMBackfillPayload{
		RepositoryID: created.ID,
	}); err != nil {
		logWarn(ctx, "queueing the first read of a repository failed", created.ID, err)
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  workspaceID,
		Action:       entity.AuditSourceControlConnected,
		ResourceKind: "scm_repository",
		ResourceID:   created.ID,
		ResourceName: created.FullName,
		Detail: map[string]string{
			"provider":   string(created.Provider),
			"repository": created.FullName,
		},
	})

	return service.ConnectedRepository{
		Repository:    created,
		WebhookURL:    callback,
		WebhookSecret: secret,
		HookInstalled: created.HookInstalled(),
	}, nil
}

func (s *connections) callbackURL(stored entity.SCMRepository) string {
	return strings.TrimRight(s.app.BaseURL, "/") +
		"/v1/source-control/" + string(stored.Provider) + "/" + stored.ID.String()
}

func (s *connections) UpdateRepository(
	ctx context.Context,
	workspaceID, repositoryID uuid.UUID,
	input service.UpdateRepositoryInput,
) (entity.SCMRepository, error) {
	if _, err := s.administers(ctx, workspaceID); err != nil {
		return entity.SCMRepository{}, err
	}

	stored, err := s.repositories.GetByID(ctx, workspaceID, repositoryID)
	if err != nil {
		return entity.SCMRepository{}, err
	}

	label := stored.MirrorLabel
	if input.MirrorLabel != "" {
		if field := entity.ValidateSCMMirrorLabel(
			"mirrorLabel", input.MirrorLabel,
		); field.Field != "" {
			return entity.SCMRepository{}, entity.ValidationError{
				Fields: []entity.FieldError{field},
			}
		}

		label = strings.TrimSpace(input.MirrorLabel)
	}

	interval := stored.PollInterval

	if input.PollInterval != 0 {
		if input.PollInterval < minPollInterval || input.PollInterval > maxPollInterval {
			return entity.SCMRepository{}, entity.ValidationError{
				Fields: []entity.FieldError{{
					Field: "pollInterval",
					Code:  entity.ValidationCodeOutOfRange,
				}},
			}
		}

		interval = input.PollInterval
	}

	direction := stored.Direction()
	if input.SyncDirection.Valid() && input.SyncDirection != "" {
		direction = input.SyncDirection
	}

	polling := stored.WebhooksDisabled
	if input.WebhooksDisabled != nil {
		polling = *input.WebhooksDisabled
	}

	return s.repositories.UpdateSettings(ctx, repositoryID, repository.SCMRepositorySettings{
		MirrorLabel:      label,
		SyncDirection:    direction,
		WebhooksDisabled: polling,
		PollInterval:     interval,
	})
}

func (s *connections) RemoveRepository(
	ctx context.Context,
	workspaceID, repositoryID uuid.UUID,
) error {
	if _, err := s.administers(ctx, workspaceID); err != nil {
		return err
	}

	stored, err := s.repositories.GetByID(ctx, workspaceID, repositoryID)
	if err != nil {
		return err
	}

	connection, err := s.connections.GetByID(ctx, workspaceID, stored.ConnectionID)
	if err != nil {
		return err
	}

	if err := s.removeRepository(ctx, connection, stored); err != nil {
		return err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  workspaceID,
		Action:       entity.AuditSourceControlDisconnected,
		ResourceKind: "scm_repository",
		ResourceID:   repositoryID,
		ResourceName: stored.FullName,
	})

	return nil
}

func (s *connections) removeRepository(
	ctx context.Context,
	connection entity.SCMConnection,
	stored entity.SCMRepository,
) error {
	if stored.HookInstalled() {
		s.removeHook(ctx, connection, stored)
	}

	return s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.links.DetachRepository(ctx, stored.ID); err != nil {
			return err
		}

		if err := s.mirrors.DetachRepository(ctx, stored.ID); err != nil {
			return err
		}

		return s.repositories.Delete(ctx, stored.WorkspaceID, stored.ID)
	})
}

func (s *connections) removeHook(
	ctx context.Context,
	connection entity.SCMConnection,
	stored entity.SCMRepository,
) {
	token, err := s.credentials.token(ctx, connection)
	if err != nil {
		logWarn(ctx, "reading the token to remove a hook failed", stored.ID, err)

		return
	}

	forge, err := s.forges.Lookup(connection.Provider)
	if err != nil {
		logWarn(ctx, "looking up the forge to remove a hook failed", stored.ID, err)

		return
	}

	if err := forge.RemoveHook(
		ctx,
		connection.Target(stored.FullName, token),
		stored.ExternalHookID,
	); err != nil {
		logging.From(ctx).WarnContext(
			ctx,
			"removing the source control hook failed; it will keep delivering until somebody "+
				"removes it on the forge",
			"repository_id", stored.ID.String(),
			"error", err.Error(),
		)
	}
}

func (s *connections) Deliveries(
	ctx context.Context,
	workspaceID, repositoryID uuid.UUID,
) ([]entity.SCMDelivery, error) {
	if _, err := s.administers(ctx, workspaceID); err != nil {
		return nil, err
	}

	if _, err := s.repositories.GetByID(ctx, workspaceID, repositoryID); err != nil {
		return nil, err
	}

	return s.deliveries.ListByRepository(ctx, repositoryID, deliveryLogSize)
}
