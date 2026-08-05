package directory

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (s *directoriesService) Settings(
	ctx context.Context,
	workspaceID uuid.UUID,
) (service.DirectorySettings, error) {
	if err := s.authorize(ctx, workspaceID, entity.ActionRead); err != nil {
		return service.DirectorySettings{}, err
	}

	connection, err := s.directories.GetConnection(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, entity.ErrDirectoryNotConnected) {
			return service.DirectorySettings{}, nil
		}

		return service.DirectorySettings{}, err
	}

	return service.DirectorySettings{Connection: connection, Connected: true}, nil
}

func (s *directoriesService) Connect(
	ctx context.Context,
	workspaceID uuid.UUID,
) (service.DirectorySettings, error) {
	if err := s.licensed(); err != nil {
		return service.DirectorySettings{}, err
	}

	if err := s.authorize(ctx, workspaceID, entity.ActionManage); err != nil {
		return service.DirectorySettings{}, err
	}

	if _, err := s.directories.GetConnection(ctx, workspaceID); err == nil {
		return s.RotateToken(ctx, workspaceID)
	} else if !errors.Is(err, entity.ErrDirectoryNotConnected) {
		return service.DirectorySettings{}, err
	}

	token, hashed, err := entity.NewDirectoryToken()
	if err != nil {
		return service.DirectorySettings{}, err
	}

	connection, err := s.directories.SaveConnection(ctx, entity.DirectoryConnection{
		WorkspaceID: workspaceID,
		TokenHash:   hashed,
		Enabled:     true,
		OnUnknown:   entity.DefaultDirectoryUnknownPolicy,
		OnAbsent:    entity.DefaultDirectoryAbsentPolicy,
	})
	if err != nil {
		return service.DirectorySettings{}, err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  workspaceID,
		Action:       entity.AuditDirectoryConnected,
		ResourceKind: string(entity.ResourceWorkspace),
		ResourceID:   workspaceID,
	})

	return service.DirectorySettings{Connection: connection, Connected: true, Token: token}, nil
}

func (s *directoriesService) RotateToken(
	ctx context.Context,
	workspaceID uuid.UUID,
) (service.DirectorySettings, error) {
	if err := s.licensed(); err != nil {
		return service.DirectorySettings{}, err
	}

	if err := s.authorize(ctx, workspaceID, entity.ActionManage); err != nil {
		return service.DirectorySettings{}, err
	}

	current, err := s.directories.GetConnection(ctx, workspaceID)
	if err != nil {
		return service.DirectorySettings{}, err
	}

	token, hashed, err := entity.NewDirectoryToken()
	if err != nil {
		return service.DirectorySettings{}, err
	}

	current.TokenHash = hashed

	connection, err := s.directories.SaveConnection(ctx, current)
	if err != nil {
		return service.DirectorySettings{}, err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  workspaceID,
		Action:       entity.AuditDirectoryTokenRotated,
		ResourceKind: string(entity.ResourceWorkspace),
		ResourceID:   workspaceID,
	})

	return service.DirectorySettings{Connection: connection, Connected: true, Token: token}, nil
}

func (s *directoriesService) Configure(
	ctx context.Context,
	workspaceID uuid.UUID,
	wanted entity.DirectoryConnection,
) (service.DirectorySettings, error) {
	if err := s.licensed(); err != nil {
		return service.DirectorySettings{}, err
	}

	if err := s.authorize(ctx, workspaceID, entity.ActionManage); err != nil {
		return service.DirectorySettings{}, err
	}

	if !wanted.OnUnknown.Valid() {
		return service.DirectorySettings{}, entity.NewValidationError(entity.FieldError{
			Field: "onUnknown",
			Code:  entity.ValidationCodeUnsupportedValue,
		})
	}

	if !wanted.OnAbsent.Valid() {
		return service.DirectorySettings{}, entity.NewValidationError(entity.FieldError{
			Field: "onAbsent",
			Code:  entity.ValidationCodeUnsupportedValue,
		})
	}

	current, err := s.directories.GetConnection(ctx, workspaceID)
	if err != nil {
		return service.DirectorySettings{}, err
	}

	current.Enabled = wanted.Enabled
	current.OnUnknown = wanted.OnUnknown
	current.OnAbsent = wanted.OnAbsent
	current.AdminGroup = wanted.AdminGroup

	connection, err := s.directories.SaveConnection(ctx, current)
	if err != nil {
		return service.DirectorySettings{}, err
	}

	return service.DirectorySettings{Connection: connection, Connected: true}, nil
}

func (s *directoriesService) Disconnect(ctx context.Context, workspaceID uuid.UUID) error {
	if err := s.authorize(ctx, workspaceID, entity.ActionManage); err != nil {
		return err
	}

	if err := s.directories.DeleteConnection(ctx, workspaceID); err != nil {
		return err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  workspaceID,
		Action:       entity.AuditDirectoryDisconnected,
		ResourceKind: string(entity.ResourceWorkspace),
		ResourceID:   workspaceID,
	})

	return nil
}

func (s *directoriesService) Runs(
	ctx context.Context,
	workspaceID uuid.UUID,
	page entity.DirectoryRunPage,
) ([]entity.DirectorySyncRun, error) {
	if err := s.licensed(); err != nil {
		return nil, err
	}

	if err := s.authorize(ctx, workspaceID, entity.ActionRead); err != nil {
		return nil, err
	}

	return s.runs.ListRuns(ctx, workspaceID, page)
}

func (s *directoriesService) Changes(
	ctx context.Context,
	workspaceID, runID uuid.UUID,
) ([]entity.DirectorySyncChange, error) {
	if err := s.licensed(); err != nil {
		return nil, err
	}

	if err := s.authorize(ctx, workspaceID, entity.ActionRead); err != nil {
		return nil, err
	}

	runs, err := s.runs.ListRuns(ctx, workspaceID, entity.DirectoryRunPage{Limit: entity.DirectoryPageMaxSize})
	if err != nil {
		return nil, err
	}

	for _, run := range runs {
		if run.ID == runID {
			return s.runs.ListChanges(ctx, runID)
		}
	}

	return nil, entity.ErrDirectoryRunNotFound
}

func (s *directoriesService) authorize(
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
