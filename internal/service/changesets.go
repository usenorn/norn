package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=changesets.go -destination=changeset/mock_changesets.go -package=changeset -mock_names=ChangeSets=MockChangeSets

type ChangeSets interface {
	Updated(ctx context.Context, runner entity.Runner, message entity.ChannelMessage) error
	Resulted(ctx context.Context, runner entity.Runner, message entity.ChannelMessage) error

	ForIssue(ctx context.Context, workspaceID, issueID uuid.UUID) (entity.IssueChangeSet, error)
}
