package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=issue.go -destination=issue/mock_issue.go -package=issue -mock_names=Issue=MockIssue

type Issue interface {
	Create(ctx context.Context, issue entity.Issue) (entity.Issue, error)
	GetVisible(ctx context.Context, workspaceID, issueID uuid.UUID, scope entity.TeamScope) (entity.Issue, error)
	GetVisibleByReference(ctx context.Context, workspaceID uuid.UUID, reference entity.IssueReference, scope entity.TeamScope) (entity.Issue, error)
	ListVisible(ctx context.Context, scope entity.TeamScope, page entity.IssuePage) ([]entity.Issue, error)
	LockByID(ctx context.Context, workspaceID, issueID uuid.UUID, scope entity.TeamScope) (entity.Issue, error)
	Update(ctx context.Context, issueID uuid.UUID, expectedVersion int, change entity.IssueChange, timestamps *entity.StateTimestamps, changedAt time.Time) error
	MoveToTeam(ctx context.Context, issueID uuid.UUID, expectedVersion int, teamID, stateID uuid.UUID, timestamps entity.StateTimestamps, changedAt time.Time) error
	SetStatus(ctx context.Context, issueID uuid.UUID, expectedVersion int, lifecycle entity.IssueLifecycle, changedAt time.Time) error
	Purge(ctx context.Context, issueID uuid.UUID, due time.Time) error
	StampLabels(ctx context.Context, issueID uuid.UUID, expectedVersion int, changedAt time.Time) error
	ReassignState(ctx context.Context, fromStateID, toStateID uuid.UUID) error
	Ancestors(ctx context.Context, issueID uuid.UUID) ([]uuid.UUID, error)
	SubtreeHeight(ctx context.Context, issueID uuid.UUID) (int, error)
	SetParent(ctx context.Context, issueID uuid.UUID, expectedVersion int, parentID *uuid.UUID, depth int, changedAt time.Time) error
	ListChildren(ctx context.Context, workspaceID, parentID uuid.UUID, scope entity.TeamScope) ([]entity.Issue, error)
	ProgressByParent(ctx context.Context, scope entity.TeamScope, parentIDs []uuid.UUID) (map[uuid.UUID]entity.IssueProgress, error)
	ProgressByCategory(ctx context.Context, scope entity.TeamScope, teamID *uuid.UUID) (entity.IssueProgress, error)
	ProgressByCycle(ctx context.Context, scope entity.TeamScope, cycleID uuid.UUID) (entity.IssueProgress, error)
	ProgressByProject(ctx context.Context, scope entity.TeamScope, projectID uuid.UUID) (entity.IssueProgress, error)
	TallyByGroup(ctx context.Context, scope entity.TeamScope, page entity.IssuePage, groupBy entity.IssueGroupBy) ([]entity.IssueGroupTally, error)
	MoveIssuesToCycle(ctx context.Context, issueIDs []uuid.UUID, cycleID *uuid.UUID, changedAt time.Time) error
}
