package scm

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

const conflictLogSize = 50

func (s *connections) Identities(
	ctx context.Context,
	workspaceID uuid.UUID,
) (entity.SCMIdentities, error) {
	if _, err := s.administers(ctx, workspaceID); err != nil {
		return nil, err
	}

	return s.identities.List(ctx, workspaceID)
}

func (s *connections) MapIdentity(
	ctx context.Context,
	workspaceID uuid.UUID,
	input service.MapSCMIdentityInput,
) (entity.SCMIdentity, error) {
	if _, err := s.administers(ctx, workspaceID); err != nil {
		return entity.SCMIdentity{}, err
	}

	fields := make([]entity.FieldError, 0, 2)

	if field := entity.ValidateSCMLogin("login", input.Login); field.Field != "" {
		fields = append(fields, field)
	}

	if !input.Provider.Valid() {
		fields = append(fields, entity.FieldError{
			Field: "provider",
			Code:  entity.ValidationCodeUnsupportedValue,
		})
	}

	if len(fields) > 0 {
		return entity.SCMIdentity{}, entity.ValidationError{Fields: fields}
	}

	if _, err := s.memberships.Get(ctx, workspaceID, input.AccountID); err != nil {
		return entity.SCMIdentity{}, err
	}

	return s.identities.Create(ctx, entity.SCMIdentity{
		WorkspaceID: workspaceID,
		AccountID:   input.AccountID,
		Provider:    input.Provider,
		Login:       entity.NormalizeSCMLogin(input.Login),
	})
}

func (s *connections) UnmapIdentity(
	ctx context.Context,
	workspaceID, identityID uuid.UUID,
) error {
	if _, err := s.administers(ctx, workspaceID); err != nil {
		return err
	}

	return s.identities.Delete(ctx, workspaceID, identityID)
}

func (s *connections) Conflicts(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) ([]entity.MirrorConflict, error) {
	if _, _, err := s.reads(ctx, workspaceID, issueID); err != nil {
		return nil, err
	}

	return s.conflicts.ListByIssue(ctx, workspaceID, issueID, conflictLogSize)
}
