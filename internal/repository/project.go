package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=project.go -destination=project/mock_project.go -package=project -mock_names=Project=MockProject,ProjectMember=MockProjectMember,ProjectStatusUpdate=MockProjectStatusUpdate,ProjectLink=MockProjectLink

type ProjectFilter struct {
	State           entity.ProjectState
	ForAccountID    *uuid.UUID
	ForTeamID       *uuid.UUID
	IncludeArchived bool
}

type ProjectSettings struct {
	Name          string
	Description   string
	LeadAccountID *uuid.UUID
	TargetOn      string
	ClearTarget   bool
	ClearLead     bool
}

type Project interface {
	Create(ctx context.Context, project entity.Project) (entity.Project, error)
	GetByID(ctx context.Context, workspaceID, projectID uuid.UUID) (entity.Project, error)
	GetBySlug(ctx context.Context, workspaceID uuid.UUID, slug string) (entity.Project, error)
	LockByID(ctx context.Context, projectID uuid.UUID) (entity.Project, error)
	ListByWorkspaceID(ctx context.Context, workspaceID uuid.UUID, filter ProjectFilter) ([]entity.Project, error)
	UpdateSettings(ctx context.Context, projectID uuid.UUID, settings ProjectSettings) (entity.Project, error)
	SetState(ctx context.Context, projectID uuid.UUID, state entity.ProjectState) (entity.Project, error)
	Archive(ctx context.Context, projectID uuid.UUID, archivedAt time.Time) (entity.Project, error)
	Unarchive(ctx context.Context, projectID uuid.UUID) (entity.Project, error)
	Delete(ctx context.Context, projectID uuid.UUID) error
	SetTeams(ctx context.Context, workspaceID, projectID uuid.UUID, teamIDs []uuid.UUID) error
	HasConcealedWork(ctx context.Context, scope entity.TeamScope, projectID uuid.UUID) (bool, error)
}

type ProjectMember interface {
	Create(ctx context.Context, membership entity.ProjectMembership) (entity.ProjectMembership, error)
	Get(ctx context.Context, projectID, accountID uuid.UUID) (entity.ProjectMembership, error)
	ListByProjectID(ctx context.Context, projectID uuid.UUID) ([]entity.ProjectMembership, error)
	Delete(ctx context.Context, projectID, accountID uuid.UUID) error
}

type ProjectLink interface {
	Add(ctx context.Context, projectID uuid.UUID, label, url string) (entity.ProjectLink, error)
	ListByProjectID(ctx context.Context, projectID uuid.UUID) ([]entity.ProjectLink, error)
	Remove(ctx context.Context, projectID, linkID uuid.UUID) error
}

type ProjectStatusUpdate interface {
	Record(ctx context.Context, update entity.ProjectStatusUpdate) (entity.ProjectStatusUpdate, error)
	ListByProjectID(ctx context.Context, projectID uuid.UUID) ([]entity.ProjectStatusUpdate, error)
}
