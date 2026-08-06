package service

import (
	"context"
	"encoding/json"
	"io"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=imports.go -destination=imports/mock_imports.go -package=imports -mock_names=Imports=MockImports,ImportRunner=MockImportRunner,ImportRescue=MockImportRescue,ImportSource=MockImportSource,ImportSourceProbe=MockImportSourceProbe,ImportSources=MockImportSources

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

// ImportSourceProbe is optional and asks a source one bounded question — which teams it
// holds, what a file's header row reads — before anything is staged. It bends the rule
// that a source is only ever called from the staging job because a team selection and a
// column mapping have to be chosen before there is anything to select from; what keeps it
// bounded is that it answers with a catalogue and never with records.
type ImportSourceProbe interface {
	Probe(ctx context.Context, config ImportSourceConfig) (entity.ImportCatalogue, error)
}

// ImportSources hands out the adapter for a source kind. In production it holds none.
type ImportSources interface {
	Lookup(kind string) (ImportSource, error)
	Kinds() []string
}

// ImportSourceConfig is split because the two halves have different readers: Settings is
// handed back to whoever configures the run every time they look at it, while Secret is
// opened in one place only, on the way into a fetch.
type ImportSourceConfig struct {
	Secret   string
	Settings json.RawMessage
}

type ImportFetchRequest struct {
	Resource entity.ImportResource
	Cursor   string
	PageHint int
	Config   ImportSourceConfig
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

type ConfigureImportInput struct {
	Secret            string
	Settings          json.RawMessage
	UnknownReferences entity.ImportUnknownPolicy
}

type MapImportInput struct {
	Decisions []entity.ImportMapping
}

// ImportUpload is a file a source reads its rows from rather than fetching them. The body is
// streamed to storage under the run's own prefix, which is the only area a file source is
// allowed to open, so an upload cannot address an object belonging to anything else.
type ImportUpload struct {
	FileName string
	Body     io.Reader
}

type ImportFile struct {
	ObjectKey string
	FileName  string
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
	List(ctx context.Context, workspaceID uuid.UUID, page entity.ImportRunPage) ([]entity.ImportRun, error)
	Sources(ctx context.Context, workspaceID uuid.UUID) ([]string, error)
	Configure(ctx context.Context, workspaceID, runID uuid.UUID, input ConfigureImportInput) (entity.ImportRun, error)
	Upload(ctx context.Context, workspaceID, runID uuid.UUID, upload ImportUpload) (ImportFile, error)
	Catalogue(ctx context.Context, workspaceID, runID uuid.UUID) (entity.ImportCatalogue, error)
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
