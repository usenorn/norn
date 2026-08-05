package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=directories.go -destination=directory/mock_directories.go -package=directory -mock_names=Directories=MockDirectories

type DirectoryAvailability struct {
	Available bool
	Holder    string
}

type DirectoryProfile struct {
	ExternalID  string
	UserName    string
	DisplayName string
	Active      bool
}

type DirectoryGroupProfile struct {
	ExternalID  string
	DisplayName string
	Members     []uuid.UUID
}

type DirectoryMembershipEdit struct {
	Add    []uuid.UUID
	Remove []uuid.UUID
}

type DirectoryUserView struct {
	User     entity.DirectoryUser
	Workload entity.DirectoryWorkload
}

type DirectorySettings struct {
	Connection entity.DirectoryConnection
	Connected  bool
	Token      string
}

type Directories interface {
	Availability(ctx context.Context) DirectoryAvailability

	Authenticate(ctx context.Context, token string) (entity.DirectoryConnection, error)

	PutUser(ctx context.Context, workspaceID uuid.UUID, id *uuid.UUID, profile DirectoryProfile) (DirectoryUserView, error)
	GetUser(ctx context.Context, workspaceID, id uuid.UUID) (entity.DirectoryUser, error)
	ListUsers(ctx context.Context, workspaceID uuid.UUID, userName string, limit int) ([]entity.DirectoryUser, error)
	SetUserActive(ctx context.Context, workspaceID, id uuid.UUID, active bool) (DirectoryUserView, error)
	DeleteUser(ctx context.Context, workspaceID, id uuid.UUID) (DirectoryUserView, error)

	PutGroup(ctx context.Context, workspaceID uuid.UUID, id *uuid.UUID, profile DirectoryGroupProfile) (entity.DirectoryGroup, error)
	GetGroup(ctx context.Context, workspaceID, id uuid.UUID) (entity.DirectoryGroup, error)
	ListGroups(ctx context.Context, workspaceID uuid.UUID, displayName string, limit int) ([]entity.DirectoryGroup, error)
	ListGroupMembers(ctx context.Context, workspaceID, id uuid.UUID) ([]entity.DirectoryUser, error)
	EditGroupMembers(ctx context.Context, workspaceID, id uuid.UUID, edit DirectoryMembershipEdit) (entity.DirectoryGroup, error)
	DeleteGroup(ctx context.Context, workspaceID, id uuid.UUID) error

	Settings(ctx context.Context, workspaceID uuid.UUID) (DirectorySettings, error)
	Connect(ctx context.Context, workspaceID uuid.UUID) (DirectorySettings, error)
	RotateToken(ctx context.Context, workspaceID uuid.UUID) (DirectorySettings, error)
	Configure(ctx context.Context, workspaceID uuid.UUID, connection entity.DirectoryConnection) (DirectorySettings, error)
	Disconnect(ctx context.Context, workspaceID uuid.UUID) error
	Runs(ctx context.Context, workspaceID uuid.UUID, page entity.DirectoryRunPage) ([]entity.DirectorySyncRun, error)
	Changes(ctx context.Context, workspaceID, runID uuid.UUID) ([]entity.DirectorySyncChange, error)
}
