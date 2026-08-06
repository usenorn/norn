package entity

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
)

const (
	ImportChunkSize      = 25
	ImportSourceKindMax  = 64
	ImportExternalIDMax  = 512
	ImportSubjectMax     = 254
	ImportRunPageDefault = 25
	ImportRunPageMax     = 100
)

var (
	ErrImportRunNotFound         = errors.New("import run not found")
	ErrImportRunLeased           = errors.New("import run is being worked on elsewhere")
	ErrImportSourceUnknown       = errors.New("no import source of that kind is available")
	ErrImportStatusTransition    = errors.New("an import cannot move to that status from where it is")
	ErrImportUnmapped            = errors.New("every source concept must be mapped before an import runs")
	ErrImportPreviewStale        = errors.New("the preview no longer describes what this import would do")
	ErrImportPreviewRequired     = errors.New("an import runs only from a preview its requester has seen")
	ErrImportWouldTriage         = errors.New("imported issues would land in triage rather than the backlog")
	ErrImportMappingNotMember    = errors.New("an import can only attribute work to a member of this workspace")
	ErrImportMappingUnknownKind  = errors.New("that is not a concept an import knows how to map")
	ErrImportTeamMappingRequired = errors.New("every source project must map onto a team this account can reach")
	ErrImportNotRevertible       = errors.New("only a finished import can be reverted")
	ErrImportRecordNotStaged     = errors.New("import record is no longer staged")
	ErrImportCursorInvalid       = errors.New("import run cursor is invalid")

	ErrImportEncryptionKeyMissing = errors.New(
		"this instance has no encryption key, so an import source key cannot be stored or read. " +
			"Set NORN_SECURITY_ENCRYPTION_KEY to 32 base64-encoded random bytes and restart",
	)
)

type ImportResource string

const (
	ImportTeam          ImportResource = "team"
	ImportWorkflowState ImportResource = "workflow_state"
	ImportLabelGroup    ImportResource = "label_group"
	ImportLabel         ImportResource = "label"
	ImportProject       ImportResource = "project"
	ImportCycle         ImportResource = "cycle"
	ImportIssue         ImportResource = "issue"
	ImportIssueParent   ImportResource = "issue_parent"
	ImportIssueRelation ImportResource = "issue_relation"
	ImportComment       ImportResource = "comment"
	ImportAttachment    ImportResource = "attachment"
	ImportEmbed         ImportResource = "embed"
)

func ImportPhases() []ImportResource {
	return []ImportResource{
		ImportTeam,
		ImportWorkflowState,
		ImportLabelGroup,
		ImportLabel,
		ImportProject,
		ImportCycle,
		ImportIssue,
		ImportIssueParent,
		ImportIssueRelation,
		ImportComment,
		ImportAttachment,
		ImportEmbed,
	}
}

func (r ImportResource) Valid() bool {
	return slices.Contains(ImportPhases(), r)
}

// Fetched reports whether a resource arrives from the source. Two do not, and for the same
// reason: they are already inside a row the source did send, so they are derived as the page
// is staged. Every tracker models a parent as a property of the child, and an inline image
// is a link inside a body that the file record beside it already names. A relation belongs
// to neither of the issues it joins and so has no record to ride in on, which is why it is
// fetched.
func (r ImportResource) Fetched() bool {
	return r != ImportIssueParent && r != ImportEmbed
}

type ImportStatus string

const (
	ImportDraft     ImportStatus = "draft"
	ImportStaging   ImportStatus = "staging"
	ImportStaged    ImportStatus = "staged"
	ImportMapped    ImportStatus = "mapped"
	ImportExecuting ImportStatus = "executing"
	ImportImported  ImportStatus = "imported"
	ImportReverting ImportStatus = "reverting"
	ImportReverted  ImportStatus = "reverted"
	ImportFailed    ImportStatus = "failed"
)

func ImportStatuses() []ImportStatus {
	return []ImportStatus{
		ImportDraft,
		ImportStaging,
		ImportStaged,
		ImportMapped,
		ImportExecuting,
		ImportImported,
		ImportReverting,
		ImportReverted,
		ImportFailed,
	}
}

func (s ImportStatus) Valid() bool {
	return slices.Contains(ImportStatuses(), s)
}

func (s ImportStatus) Settled() bool {
	return s == ImportImported || s == ImportReverted || s == ImportFailed
}

func (s ImportStatus) Leasable() bool {
	return s == ImportStaging || s == ImportExecuting || s == ImportReverting
}

func (s ImportStatus) Revertible() bool {
	return s == ImportImported || s == ImportFailed
}

func (s ImportStatus) CanTransitionTo(target ImportStatus) bool {
	if !target.Valid() {
		return false
	}

	if target == ImportFailed {
		return s != ImportReverted && s != ImportFailed
	}

	switch s {
	case ImportDraft:
		return target == ImportStaging
	case ImportStaging:
		return target == ImportStaging || target == ImportStaged
	case ImportStaged:
		return target == ImportStaged || target == ImportMapped
	case ImportMapped:
		return target == ImportStaged || target == ImportExecuting
	case ImportExecuting:
		return target == ImportExecuting || target == ImportImported
	case ImportImported:
		return target == ImportReverting
	case ImportReverting:
		return target == ImportReverting || target == ImportReverted
	case ImportFailed:
		return target == ImportReverting
	default:
		return false
	}
}

// Configurable reports whether a run's source configuration may still be changed. Rows
// were drained with the key and the selection the run was given, so a run that has moved
// past staged would otherwise hold rows that its own configuration no longer explains.
func (s ImportStatus) Configurable() bool {
	return s == ImportDraft || s == ImportStaging || s == ImportStaged
}

type ImportUnknownPolicy string

const (
	ImportUnknownCreate ImportUnknownPolicy = "create"
	ImportUnknownSkip   ImportUnknownPolicy = "skip"
	ImportUnknownFail   ImportUnknownPolicy = "fail"
)

func ImportUnknownPolicies() []ImportUnknownPolicy {
	return []ImportUnknownPolicy{ImportUnknownCreate, ImportUnknownSkip, ImportUnknownFail}
}

func (p ImportUnknownPolicy) Valid() bool {
	return slices.Contains(ImportUnknownPolicies(), p)
}

func (p ImportUnknownPolicy) Or(fallback ImportUnknownPolicy) ImportUnknownPolicy {
	if p == "" {
		return fallback
	}

	return p
}

type ImportRecordState string

const (
	ImportRecordStaged  ImportRecordState = "staged"
	ImportRecordApplied ImportRecordState = "applied"
	ImportRecordSkipped ImportRecordState = "skipped"
	ImportRecordFailed  ImportRecordState = "failed"
)

func ImportRecordStates() []ImportRecordState {
	return []ImportRecordState{
		ImportRecordStaged,
		ImportRecordApplied,
		ImportRecordSkipped,
		ImportRecordFailed,
	}
}

func (s ImportRecordState) Valid() bool {
	return slices.Contains(ImportRecordStates(), s)
}

type ImportOutcome string

const (
	ImportOutcomeCreated         ImportOutcome = "created"
	ImportOutcomeSkipped         ImportOutcome = "skipped"
	ImportOutcomeUnattributed    ImportOutcome = "unattributed"
	ImportOutcomeAdjusted        ImportOutcome = "adjusted"
	ImportOutcomeDropped         ImportOutcome = "dropped"
	ImportOutcomeFailed          ImportOutcome = "failed"
	ImportOutcomeDeleted         ImportOutcome = "deleted"
	ImportOutcomeArchived        ImportOutcome = "archived"
	ImportOutcomeRetained        ImportOutcome = "retained"
	ImportOutcomeSkippedModified ImportOutcome = "skipped_modified"
	ImportOutcomeSkippedInUse    ImportOutcome = "skipped_in_use"
)

func ImportOutcomes() []ImportOutcome {
	return []ImportOutcome{
		ImportOutcomeCreated,
		ImportOutcomeSkipped,
		ImportOutcomeUnattributed,
		ImportOutcomeAdjusted,
		ImportOutcomeDropped,
		ImportOutcomeFailed,
		ImportOutcomeDeleted,
		ImportOutcomeArchived,
		ImportOutcomeRetained,
		ImportOutcomeSkippedModified,
		ImportOutcomeSkippedInUse,
	}
}

func (o ImportOutcome) Valid() bool {
	return slices.Contains(ImportOutcomes(), o)
}

func (o ImportOutcome) Reverted() bool {
	return o == ImportOutcomeDeleted || o == ImportOutcomeArchived
}

func OutcomeForImport(err error) ImportOutcome {
	var validation ValidationError

	switch {
	case err == nil:
		return ImportOutcomeCreated

	case errors.Is(err, ErrTeamNotFound),
		errors.Is(err, ErrIssueNotFound),
		errors.Is(err, ErrProjectNotFound),
		errors.Is(err, ErrLabelNotFound),
		errors.Is(err, ErrWorkflowStateNotFound),
		errors.Is(err, ErrIssueCommentNotAuthor),
		errors.Is(err, ErrAccountForbidden):
		return ImportOutcomeSkipped

	case errors.Is(err, ErrIssueRelationExists),
		errors.Is(err, ErrIssueRelationSelf),
		errors.Is(err, ErrProjectSlugTaken),
		errors.Is(err, ErrCycleOverlaps),
		errors.Is(err, ErrCycleTeamMismatch),
		errors.Is(err, ErrStorageExhausted),
		errors.Is(err, ErrAttachmentTooLarge):
		return ImportOutcomeSkipped

	case errors.Is(err, ErrTeamKeyTaken),
		errors.Is(err, ErrWorkflowStateNameTaken),
		errors.Is(err, ErrLabelNameTaken),
		errors.Is(err, ErrLabelGroupNameTaken),
		errors.Is(err, ErrLabelGroupExclusive),
		errors.Is(err, ErrIssueReferenceTaken):
		return ImportOutcomeSkipped

	case errors.As(err, &validation):
		return ImportOutcomeSkipped

	default:
		return ImportOutcomeFailed
	}
}

type ImportPhase string

const (
	ImportPhaseExecute ImportPhase = "execute"
	ImportPhaseRevert  ImportPhase = "revert"
)

func ImportPhaseNames() []ImportPhase {
	return []ImportPhase{ImportPhaseExecute, ImportPhaseRevert}
}

func (p ImportPhase) Valid() bool {
	return slices.Contains(ImportPhaseNames(), p)
}

type ImportRun struct {
	ID                  uuid.UUID
	WorkspaceID         uuid.UUID
	SourceKind          string
	SourceLabel         string
	RequestedByAccount  uuid.UUID
	RequestedActorKind  ActorKind
	RequestedAuthMethod SessionAuthMethod
	RevertedByAccount   uuid.UUID
	RevertedActorKind   ActorKind
	RevertedAuthMethod  SessionAuthMethod
	Status              ImportStatus
	PhaseError          string
	PreviewDigest       string
	AcknowledgeTriage   bool
	Settings            json.RawMessage
	UnknownReferences   ImportUnknownPolicy
	SourceSecretSet     bool
	LeaseToken          uuid.UUID
	LeaseExpiresAt      *time.Time
	Attempt             int
	Staged              int
	Processed           int
	StagedAt            *time.Time
	MappedAt            *time.Time
	StartedAt           *time.Time
	FinishedAt          *time.Time
	RevertedAt          *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (r ImportRun) Requester() Actor {
	return actorFrom(r.RequestedActorKind, r.RequestedByAccount, r.RequestedAuthMethod)
}

func (r ImportRun) Reverter() Actor {
	return actorFrom(r.RevertedActorKind, r.RevertedByAccount, r.RevertedAuthMethod)
}

func actorFrom(kind ActorKind, account uuid.UUID, method SessionAuthMethod) Actor {
	if kind == "" {
		kind = ActorKindUser
	}

	return Actor{Kind: kind, AccountID: account, AuthMethod: method}
}

type ImportCursor struct {
	RunID      uuid.UUID
	Resource   ImportResource
	Cursor     string
	Exhausted  bool
	Fetched    int
	RetryAfter *time.Time
	UpdatedAt  time.Time
}

func (c ImportCursor) Parked(now time.Time) bool {
	return c.RetryAfter != nil && c.RetryAfter.After(now)
}

type ImportRecord struct {
	RunID            uuid.UUID
	Resource         ImportResource
	ExternalID       string
	ParentExternalID string
	Payload          []byte
	SourceCreatedAt  *time.Time
	SourceUpdatedAt  *time.Time
	State            ImportRecordState
	OutcomeDetail    string
	CreatedID        uuid.UUID
	Seq              int64
}

type ImportLedgerEntry struct {
	RunID             uuid.UUID
	WorkspaceID       uuid.UUID
	Resource          ImportResource
	CreatedID         uuid.UUID
	ExternalID        string
	Reference         string
	VersionAtCreate   int
	CreatedAtRecorded time.Time
	OrderSeq          int64
	RevertedAt        *time.Time
	RevertOutcome     ImportOutcome
}

type ImportReportLine struct {
	ID         uuid.UUID
	RunID      uuid.UUID
	Phase      ImportPhase
	Resource   ImportResource
	Subject    string
	ExternalID string
	Outcome    ImportOutcome
	Detail     map[string]string
	RecordedAt time.Time
}

type ImportReportCursor struct {
	RecordedAt time.Time
	ID         uuid.UUID
}

type ImportRunCursor struct {
	CreatedAt time.Time
	RunID     uuid.UUID
}

func (c ImportRunCursor) Encode() string {
	return base64.RawURLEncoding.EncodeToString(
		[]byte(c.RunID.String() + c.CreatedAt.UTC().Format(time.RFC3339Nano)),
	)
}

func DecodeImportRunCursor(raw string) (ImportRunCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) < ActivityCursorIDLen {
		return ImportRunCursor{}, ErrImportCursorInvalid
	}

	runID, err := uuid.Parse(string(decoded[:ActivityCursorIDLen]))
	if err != nil {
		return ImportRunCursor{}, ErrImportCursorInvalid
	}

	createdAt, err := time.Parse(time.RFC3339Nano, string(decoded[ActivityCursorIDLen:]))
	if err != nil {
		return ImportRunCursor{}, ErrImportCursorInvalid
	}

	return ImportRunCursor{CreatedAt: createdAt, RunID: runID}, nil
}

func (r ImportRun) Cursor() ImportRunCursor {
	return ImportRunCursor{CreatedAt: r.CreatedAt, RunID: r.ID}
}

type ImportRunPage struct {
	Cursor *ImportRunCursor
	Limit  int
}

func (p ImportRunPage) Normalized() ImportRunPage {
	if p.Limit <= 0 || p.Limit > ImportRunPageMax {
		p.Limit = ImportRunPageDefault
	}

	return p
}

type ImportWalk struct {
	Resource ImportResource
	After    int64
	Limit    int
}

func (w ImportWalk) Normalized() ImportWalk {
	if w.Limit <= 0 {
		w.Limit = ImportChunkSize
	}

	return w
}

type ImportLedgerWalk struct {
	Before int64
	Limit  int
}

func (w ImportLedgerWalk) Normalized() ImportLedgerWalk {
	if w.Limit <= 0 {
		w.Limit = ImportChunkSize
	}

	if w.Before <= 0 {
		w.Before = maxLedgerOrder
	}

	return w
}

const maxLedgerOrder int64 = 1<<62 - 1
