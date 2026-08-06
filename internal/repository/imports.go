package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=imports.go -destination=imports/mock_imports.go -package=imports -mock_names=ImportRun=MockImportRun,ImportCursor=MockImportCursor,ImportRecord=MockImportRecord,ImportMapping=MockImportMapping,ImportLedger=MockImportLedger,ImportReport=MockImportReport

type ImportLease struct {
	RunID   uuid.UUID
	Token   uuid.UUID
	Status  entity.ImportStatus
	Expires time.Time
}

type ImportSourceSecret struct {
	Secret   string
	Settings json.RawMessage
}

type ImportRun interface {
	Create(ctx context.Context, run entity.ImportRun) (entity.ImportRun, error)
	GetByID(ctx context.Context, workspaceID, runID uuid.UUID) (entity.ImportRun, error)
	SaveSourceConfig(ctx context.Context, runID uuid.UUID, secret string, settings json.RawMessage, policy entity.ImportUnknownPolicy) error
	SourceConfigForStaging(ctx context.Context, runID uuid.UUID) (ImportSourceSecret, error)
	Claim(ctx context.Context, runID uuid.UUID, from, to entity.ImportStatus, lease ImportLease, at time.Time) (uuid.UUID, error)
	Heartbeat(ctx context.Context, runID, token uuid.UUID, expires time.Time) error
	Release(ctx context.Context, runID, token uuid.UUID, at time.Time) error
	Advance(ctx context.Context, runID uuid.UUID, staged, processed int) error
	Settle(ctx context.Context, runID uuid.UUID, status entity.ImportStatus, detail string, at time.Time) error
	AcceptPreview(ctx context.Context, runID uuid.UUID, digest string, acknowledgeTriage bool, at time.Time) error
	RecordReverter(ctx context.Context, runID uuid.UUID, actor entity.Actor, at time.Time) error
	ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, page entity.ImportRunPage) ([]entity.ImportRun, error)
	ListLeaseExpired(ctx context.Context, at, idleSince time.Time, limit int) ([]entity.ImportRun, error)
}

type ImportCursor interface {
	List(ctx context.Context, runID uuid.UUID) ([]entity.ImportCursor, error)
	Save(ctx context.Context, cursor entity.ImportCursor) error
	Park(ctx context.Context, runID uuid.UUID, resource entity.ImportResource, until time.Time) error
}

type ImportRecord interface {
	Stage(ctx context.Context, runID uuid.UUID, records []entity.ImportRecord) (int, error)
	Walk(ctx context.Context, runID uuid.UUID, walk entity.ImportWalk) ([]entity.ImportRecord, error)
	Settle(ctx context.Context, runID uuid.UUID, resource entity.ImportResource, state entity.ImportRecordState, outcomes []entity.ImportRecord) ([]string, error)
	Purge(ctx context.Context, before time.Time, limit int) (int, error)
}

type ImportMapping interface {
	List(ctx context.Context, runID uuid.UUID) ([]entity.ImportMapping, error)
	Save(ctx context.Context, runID uuid.UUID, mappings []entity.ImportMapping) error
	Decide(ctx context.Context, runID uuid.UUID, decisions []entity.ImportMapping, by uuid.UUID, at time.Time) error
}

type ImportLedger interface {
	Record(ctx context.Context, runID uuid.UUID, entries []entity.ImportLedgerEntry) error
	Walk(ctx context.Context, runID uuid.UUID, walk entity.ImportLedgerWalk) ([]entity.ImportLedgerEntry, error)
	Resolve(ctx context.Context, runID uuid.UUID, resource entity.ImportResource, externalIDs []string) (map[string]uuid.UUID, error)
	Restamp(ctx context.Context, runID uuid.UUID, entries []entity.ImportLedgerEntry) error
	Settle(ctx context.Context, runID uuid.UUID, entries []entity.ImportLedgerEntry, at time.Time) error
}

type ImportReport interface {
	Record(ctx context.Context, runID uuid.UUID, lines []entity.ImportReportLine) error
	List(ctx context.Context, runID uuid.UUID, phase entity.ImportPhase, after *entity.ImportReportCursor, limit int) ([]entity.ImportReportLine, error)
}
