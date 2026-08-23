package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=changeset.go -destination=changeset/mock_changeset.go -package=changeset -mock_names=ChangeSet=MockChangeSet

type ChangeSet interface {
	SaveResult(ctx context.Context, result entity.ExecutionResult) (entity.ExecutionResult, error)
	SaveChange(ctx context.Context, change entity.ExecutionChange) (entity.ExecutionChange, error)
	SaveValidation(
		ctx context.Context,
		validation entity.ExecutionValidation,
	) (entity.ExecutionValidation, error)
	LinkChange(ctx context.Context, changeID, codeLinkID uuid.UUID) error
	Get(ctx context.Context, executionID string) (entity.ExecutionChangeSet, error)
	ByIssue(ctx context.Context, workspaceID, issueID uuid.UUID) (entity.IssueChangeSet, error)
}
