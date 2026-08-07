package scm

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

type source struct {
	connection entity.SCMConnection
	repository entity.SCMRepository
	token      string
}

func (o source) target() entity.SCMTarget {
	return o.connection.Target(o.repository.FullName, o.token)
}

func (o source) workspaceID() uuid.UUID {
	return o.repository.WorkspaceID
}

func (s *sync) sourceFor(ctx context.Context, repositoryID uuid.UUID) (source, error) {
	stored, err := s.repositories.GetForDelivery(ctx, repositoryID)
	if err != nil {
		return source{}, err
	}

	connection, err := s.connections.GetForDelivery(ctx, stored.ConnectionID)
	if err != nil {
		return source{}, err
	}

	token, err := s.connections.Token(ctx, connection.ID)
	if err != nil {
		return source{}, err
	}

	return source{connection: connection, repository: stored, token: token}, nil
}

func (s *sync) teamsFor(
	ctx context.Context,
	from source,
	paths []string,
) ([]uuid.UUID, error) {
	routes, err := s.routes.ListByRepository(ctx, from.repository.ID)
	if err != nil {
		return nil, err
	}

	return routes.Teams(paths), nil
}

func (s *sync) reaches(ctx context.Context, from source, teamID uuid.UUID) (bool, error) {
	routes, err := s.routes.ListByRepository(ctx, from.repository.ID)
	if err != nil {
		return false, err
	}

	return routes.Reaches(teamID), nil
}
