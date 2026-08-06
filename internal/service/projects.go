package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=projects.go -destination=project/mock_projects.go -package=project -mock_names=Projects=MockProjects

type ProjectMemberView struct {
	Membership  entity.ProjectMembership
	DisplayName string
	Email       string
}

type ProjectView struct {
	Project       entity.Project
	ConcealedWork bool
	LatestStatus  *entity.ProjectStatusUpdate
}

type CreateProjectInput struct {
	WorkspaceID   uuid.UUID
	Slug          string
	Name          string
	Description   string
	LeadAccountID *uuid.UUID
	TargetOn      string
	Origin        *entity.ImportOrigin
}

type UpdateProjectInput struct {
	Name          *string
	Description   *string
	LeadAccountID *uuid.UUID
	TargetOn      *string
	Clear         []string
}

type ListProjectsInput struct {
	State    entity.ProjectState
	Archived bool
	Mine     bool
}

type PostProjectStatusInput struct {
	Health entity.ProjectHealth
	Body   string
}

type Projects interface {
	Create(ctx context.Context, input CreateProjectInput) (ProjectView, error)
	Get(ctx context.Context, workspaceID, projectID uuid.UUID) (ProjectView, error)
	GetBySlug(ctx context.Context, workspaceID uuid.UUID, slug string) (ProjectView, error)
	List(ctx context.Context, workspaceID uuid.UUID, input ListProjectsInput) ([]ProjectView, error)
	Update(ctx context.Context, workspaceID, projectID uuid.UUID, input UpdateProjectInput) (ProjectView, error)
	SetState(ctx context.Context, workspaceID, projectID uuid.UUID, state entity.ProjectState) (ProjectView, error)
	Archive(ctx context.Context, workspaceID, projectID uuid.UUID) (ProjectView, error)
	Unarchive(ctx context.Context, workspaceID, projectID uuid.UUID) (ProjectView, error)
	Remove(ctx context.Context, workspaceID, projectID uuid.UUID) error
	ListMembers(ctx context.Context, workspaceID, projectID uuid.UUID) ([]ProjectMemberView, error)
	AddMember(ctx context.Context, workspaceID, projectID, accountID uuid.UUID) (ProjectMemberView, error)
	RemoveMember(ctx context.Context, workspaceID, projectID, accountID uuid.UUID) error
	Activity(ctx context.Context, workspaceID, projectID uuid.UUID, input ListActivityInput) (ActivityPage, error)
	PostStatus(ctx context.Context, workspaceID, projectID uuid.UUID, input PostProjectStatusInput) (entity.ProjectStatusUpdate, error)
	ListStatus(ctx context.Context, workspaceID, projectID uuid.UUID) ([]entity.ProjectStatusUpdate, error)
}
