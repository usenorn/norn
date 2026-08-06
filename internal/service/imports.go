package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=imports.go -destination=imports/mock_imports.go -package=imports -mock_names=Imports=MockImports,ImportRunner=MockImportRunner,ImportRescue=MockImportRescue,ImportSource=MockImportSource,ImportSources=MockImportSources

// ImportSource is what a foreign tracker looks like from in here. Nothing in this
// repository implements it: an adapter is source-specific logic and arrives with the
// slice that needs it. Fetch reports a rate limit by returning
// entity.ImportRateLimitedError, which the staging job honours by parking rather than
// sleeping, so a throttled source slows an import down instead of failing it.
type ImportSource interface {
	Kind() string
	Resources() []entity.ImportResource
	Fetch(ctx context.Context, request ImportFetchRequest) (ImportFetchPage, error)
}

// ImportSources hands out the adapter for a source kind. In production it holds none.
type ImportSources interface {
	Lookup(kind string) (ImportSource, error)
	Kinds() []string
}

type ImportFetchRequest struct {
	Resource entity.ImportResource
	Cursor   string
	PageHint int
}

type ImportFetchPage struct {
	Records    []entity.ImportRecord
	NextCursor string
	Done       bool
}

type CreateImportInput struct {
	WorkspaceID uuid.UUID
	SourceKind  string
	SourceLabel string
}

type MapImportInput struct {
	Decisions []entity.ImportMapping
}

type ExecuteImportInput struct {
	PreviewDigest     string
	AcknowledgeTriage bool
}

type ImportReportView struct {
	Run               entity.ImportRun
	Created           []entity.ImportLedgerEntry
	Lines             []entity.ImportReportLine
	NextCreatedCursor int64
	NextLineCursor    *entity.ImportReportCursor
}

type Imports interface {
	Connect(ctx context.Context, input CreateImportInput) (entity.ImportRun, error)
	Get(ctx context.Context, workspaceID, runID uuid.UUID) (entity.ImportRun, error)
	Stage(ctx context.Context, workspaceID, runID uuid.UUID) (entity.ImportRun, error)
	Mappings(ctx context.Context, workspaceID, runID uuid.UUID) (entity.MappingPlan, error)
	Map(ctx context.Context, workspaceID, runID uuid.UUID, input MapImportInput) (entity.MappingPlan, error)
	Preview(ctx context.Context, workspaceID, runID uuid.UUID) (entity.ImportPreview, error)
	Execute(ctx context.Context, workspaceID, runID uuid.UUID, input ExecuteImportInput) (entity.ImportRun, error)
	Revert(ctx context.Context, workspaceID, runID uuid.UUID) (entity.ImportRun, error)
	Report(ctx context.Context, workspaceID, runID uuid.UUID) (ImportReportView, error)
}

type ImportRunner interface {
	RunStage(ctx context.Context, payload entity.ImportStagePayload) error
	RunExecute(ctx context.Context, payload entity.ImportExecutePayload) error
	RunRevert(ctx context.Context, payload entity.ImportRevertPayload) error
}

type ImportRescue interface {
	Rescue(ctx context.Context) (int, error)
	Sweep(ctx context.Context) (int, error)
}
