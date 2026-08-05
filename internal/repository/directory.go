package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=directory.go -destination=directory/mock_directory.go -package=directory -mock_names=Directory=MockDirectory,DirectorySync=MockDirectorySync

type Directory interface {
	GetConnection(ctx context.Context, workspaceID uuid.UUID) (entity.DirectoryConnection, error)
	GetConnectionByToken(ctx context.Context, tokenHash []byte) (entity.DirectoryConnection, error)
	SaveConnection(ctx context.Context, connection entity.DirectoryConnection) (entity.DirectoryConnection, error)
	DeleteConnection(ctx context.Context, workspaceID uuid.UUID) error
	StampSync(ctx context.Context, workspaceID uuid.UUID) error

	SaveUser(ctx context.Context, user entity.DirectoryUser) (entity.DirectoryUser, error)
	GetUser(ctx context.Context, workspaceID, id uuid.UUID) (entity.DirectoryUser, error)
	GetUserByName(ctx context.Context, workspaceID uuid.UUID, userName string) (entity.DirectoryUser, error)
	GetUserByAccount(ctx context.Context, workspaceID, accountID uuid.UUID) (entity.DirectoryUser, error)
	ListUsers(ctx context.Context, workspaceID uuid.UUID, userName string, limit int) ([]entity.DirectoryUser, error)
	DeleteUser(ctx context.Context, workspaceID, id uuid.UUID) error

	SaveGroup(ctx context.Context, group entity.DirectoryGroup) (entity.DirectoryGroup, error)
	GetGroup(ctx context.Context, workspaceID, id uuid.UUID) (entity.DirectoryGroup, error)
	GetGroupByName(ctx context.Context, workspaceID uuid.UUID, displayName string) (entity.DirectoryGroup, error)
	ListGroups(ctx context.Context, workspaceID uuid.UUID, displayName string, limit int) ([]entity.DirectoryGroup, error)
	DeleteGroup(ctx context.Context, workspaceID, id uuid.UUID) error

	AddGroupMember(ctx context.Context, groupID, directoryUserID uuid.UUID) error
	RemoveGroupMember(ctx context.Context, groupID, directoryUserID uuid.UUID) error
	ListGroupMembers(ctx context.Context, groupID uuid.UUID) ([]entity.DirectoryUser, error)
	ListGroupsForUser(ctx context.Context, directoryUserID uuid.UUID) ([]entity.DirectoryGroup, error)
}

type DirectorySync interface {
	RecordRun(ctx context.Context, run entity.DirectorySyncRun) (entity.DirectorySyncRun, error)
	ListRuns(ctx context.Context, workspaceID uuid.UUID, page entity.DirectoryRunPage) ([]entity.DirectorySyncRun, error)
	ListChanges(ctx context.Context, runID uuid.UUID) ([]entity.DirectorySyncChange, error)
}
