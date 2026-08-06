package imports_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	attachmentrepo "github.com/usenorn/norn/internal/repository/attachment"
	blobrepo "github.com/usenorn/norn/internal/repository/blob"
	cyclerepo "github.com/usenorn/norn/internal/repository/cycle"
	importsrepo "github.com/usenorn/norn/internal/repository/imports"
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	issuecommentrepo "github.com/usenorn/norn/internal/repository/issuecomment"
	issuerelationrepo "github.com/usenorn/norn/internal/repository/issuerelation"
	jobqueuerepo "github.com/usenorn/norn/internal/repository/jobqueue"
	labelrepo "github.com/usenorn/norn/internal/repository/label"
	labelgrouprepo "github.com/usenorn/norn/internal/repository/labelgroup"
	membershiprepo "github.com/usenorn/norn/internal/repository/membership"
	projectrepo "github.com/usenorn/norn/internal/repository/project"
	teamrepo "github.com/usenorn/norn/internal/repository/team"
	teammemberrepo "github.com/usenorn/norn/internal/repository/teammember"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	triagerepo "github.com/usenorn/norn/internal/repository/triage"
	workflowstaterepo "github.com/usenorn/norn/internal/repository/workflowstate"
	"github.com/usenorn/norn/internal/service"
	attachmentsvc "github.com/usenorn/norn/internal/service/attachment"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	cyclesvc "github.com/usenorn/norn/internal/service/cycle"
	importssvc "github.com/usenorn/norn/internal/service/imports"
	issuesvc "github.com/usenorn/norn/internal/service/issue"
	issuecommentsvc "github.com/usenorn/norn/internal/service/issuecomment"
	issuerelationsvc "github.com/usenorn/norn/internal/service/issuerelation"
	labelsvc "github.com/usenorn/norn/internal/service/label"
	projectsvc "github.com/usenorn/norn/internal/service/project"
	teamsvc "github.com/usenorn/norn/internal/service/team"
	workflowstatesvc "github.com/usenorn/norn/internal/service/workflowstate"
)

const (
	testLeaseTTL = 2 * time.Minute
	testPageSize = 3
)

var errLostCommit = errors.New("the connection died before the chunk committed")

// errTransactionAborted is what Postgres answers with SQLSTATE 25P02 to every command that
// follows a failed one, until the transaction is rolled back to a savepoint or ended. A fake
// that lets the next write through after refusing one cannot see a chunk poisoned by a single
// colliding row, which is the whole of what a savepoint is here to prevent.
var errTransactionAborted = errors.New(
	"current transaction is aborted, commands ignored until end of transaction block",
)

func testImportConfig() config.Imports {
	return config.Imports{
		ChunkSize:       entity.ImportChunkSize,
		PageSize:        testPageSize,
		SliceBudget:     time.Minute,
		LeaseTTL:        testLeaseTTL,
		RescueBatch:     10,
		MaxAttempts:     5,
		MinBackoff:      time.Second,
		MaxBackoff:      time.Minute,
		RecordRetention: time.Hour,
	}
}

type advance struct {
	staged    int
	processed int
}

type parking struct {
	resource entity.ImportResource
	until    time.Time
}

type world struct {
	run        entity.ImportRun
	secret     string
	keyMissing bool

	cursors  []entity.ImportCursor
	records  []entity.ImportRecord
	ledger   []entity.ImportLedgerEntry
	lines    []entity.ImportReportLine
	mappings []entity.ImportMapping
	advances []advance
	saves    []entity.ImportCursor
	parks    []parking
	seq      int64
	order    int64
}

func (w *world) snapshot() world {
	kept := *w
	kept.cursors = slices.Clone(w.cursors)
	kept.records = slices.Clone(w.records)
	kept.ledger = slices.Clone(w.ledger)
	kept.lines = slices.Clone(w.lines)
	kept.mappings = slices.Clone(w.mappings)
	kept.advances = slices.Clone(w.advances)
	kept.saves = slices.Clone(w.saves)
	kept.parks = slices.Clone(w.parks)

	return kept
}

func (w *world) restore(kept world) { *w = kept }

func (w *world) recordAt(resource entity.ImportResource, externalID string) int {
	return slices.IndexFunc(w.records, func(record entity.ImportRecord) bool {
		return record.Resource == resource && record.ExternalID == externalID
	})
}

func (w *world) stage(records []entity.ImportRecord) int {
	staged := 0

	for _, record := range records {
		if at := w.recordAt(record.Resource, record.ExternalID); at >= 0 {
			w.records[at].Payload = record.Payload
			w.records[at].ParentExternalID = record.ParentExternalID
			w.records[at].SourceCreatedAt = record.SourceCreatedAt
			w.records[at].SourceUpdatedAt = record.SourceUpdatedAt

			continue
		}

		w.seq++

		record.Seq = w.seq
		record.State = entity.ImportRecordStaged

		w.records = append(w.records, record)
		staged++
	}

	return staged
}

func (w *world) walkRecords(walk entity.ImportWalk) []entity.ImportRecord {
	walk = walk.Normalized()

	found := make([]entity.ImportRecord, 0, walk.Limit)

	for _, record := range w.records {
		if record.Resource != walk.Resource ||
			record.State != entity.ImportRecordStaged ||
			record.Seq <= walk.After {
			continue
		}

		found = append(found, record)

		if len(found) == walk.Limit {
			break
		}
	}

	return found
}

func (w *world) settleRecords(
	resource entity.ImportResource,
	state entity.ImportRecordState,
	outcomes []entity.ImportRecord,
) []string {
	settled := make([]string, 0, len(outcomes))

	for _, outcome := range outcomes {
		at := w.recordAt(resource, outcome.ExternalID)
		if at < 0 || w.records[at].State != entity.ImportRecordStaged {
			continue
		}

		w.records[at].State = state
		w.records[at].CreatedID = outcome.CreatedID
		w.records[at].OutcomeDetail = outcome.OutcomeDetail

		settled = append(settled, outcome.ExternalID)
	}

	return settled
}

// recordLedger defaults created_at_recorded exactly as the column does — to the moment the
// transaction opened — and never overwrites a stamp the apply pass carried in. Stamping every
// entry here instead would hide the whole of what this field is for: in Postgres now() is
// frozen at the start of the transaction, so a defaulted stamp sits a shade before the rows
// written inside it and makes each one read as edited by somebody afterwards.
func (w *world) recordLedger(
	runID uuid.UUID,
	entries []entity.ImportLedgerEntry,
	opened time.Time,
) {
	for _, entry := range entries {
		held := slices.ContainsFunc(w.ledger, func(kept entity.ImportLedgerEntry) bool {
			return kept.Resource == entry.Resource && kept.CreatedID == entry.CreatedID
		})
		if held {
			continue
		}

		w.order++

		entry.RunID = runID
		entry.OrderSeq = w.order

		if entry.CreatedAtRecorded.IsZero() {
			entry.CreatedAtRecorded = opened
		}

		w.ledger = append(w.ledger, entry)
	}
}

func (w *world) walkLedger(walk entity.ImportLedgerWalk) []entity.ImportLedgerEntry {
	walk = walk.Normalized()

	found := make([]entity.ImportLedgerEntry, 0, walk.Limit)

	for at := len(w.ledger) - 1; at >= 0; at-- {
		entry := w.ledger[at]

		if entry.RevertedAt != nil || entry.OrderSeq >= walk.Before {
			continue
		}

		found = append(found, entry)

		if len(found) == walk.Limit {
			break
		}
	}

	return found
}

func (w *world) resolve(resource entity.ImportResource, externalIDs []string) map[string]uuid.UUID {
	resolved := make(map[string]uuid.UUID, len(externalIDs))

	for _, entry := range w.ledger {
		if entry.Resource == resource && slices.Contains(externalIDs, entry.ExternalID) {
			resolved[entry.ExternalID] = entry.CreatedID
		}
	}

	return resolved
}

func (w *world) restampLedger(entries []entity.ImportLedgerEntry) {
	for _, entry := range entries {
		for index := range w.ledger {
			kept := &w.ledger[index]

			if kept.Resource != entry.Resource || kept.CreatedID != entry.CreatedID {
				continue
			}

			if kept.RevertedAt != nil {
				continue
			}

			kept.VersionAtCreate = entry.VersionAtCreate
		}
	}
}

func (w *world) settleLedger(entries []entity.ImportLedgerEntry, at time.Time) {
	for _, entry := range entries {
		for index := range w.ledger {
			kept := &w.ledger[index]

			if kept.Resource != entry.Resource || kept.CreatedID != entry.CreatedID {
				continue
			}

			if kept.RevertedAt != nil {
				continue
			}

			reverted := at
			kept.RevertedAt = &reverted
			kept.RevertOutcome = entry.RevertOutcome
		}
	}
}

func (w *world) note(runID uuid.UUID, lines []entity.ImportReportLine) {
	for _, line := range lines {
		line.ID = uuid.New()
		line.RunID = runID
		line.RecordedAt = time.Now().UTC()

		w.lines = append(w.lines, line)
	}
}

func (w *world) saveCursor(cursor entity.ImportCursor) {
	at := slices.IndexFunc(w.cursors, func(kept entity.ImportCursor) bool {
		return kept.Resource == cursor.Resource
	})

	if at < 0 {
		w.cursors = append(w.cursors, cursor)
	} else {
		w.cursors[at] = cursor
	}

	w.saves = append(w.saves, cursor)
}

func (w *world) park(resource entity.ImportResource, until time.Time) {
	at := slices.IndexFunc(w.cursors, func(kept entity.ImportCursor) bool {
		return kept.Resource == resource
	})

	retry := until

	if at < 0 {
		w.cursors = append(w.cursors, entity.ImportCursor{
			RunID:      w.run.ID,
			Resource:   resource,
			RetryAfter: &retry,
		})
	} else {
		w.cursors[at].RetryAfter = &retry
	}

	w.parks = append(w.parks, parking{resource: resource, until: until})
}

func (w *world) mappingAt(kind entity.ImportMappingKind, sourceKey string) int {
	return slices.IndexFunc(w.mappings, func(mapping entity.ImportMapping) bool {
		return mapping.Kind == kind && mapping.SourceKey == sourceKey
	})
}

func (w *world) saveMappings(mappings []entity.ImportMapping) {
	for _, mapping := range mappings {
		at := w.mappingAt(mapping.Kind, mapping.SourceKey)

		if at < 0 {
			mapping.Decision = ""
			mapping.DecidedAt = nil

			w.mappings = append(w.mappings, mapping)

			continue
		}

		if w.mappings[at].DecidedAt != nil {
			continue
		}

		w.mappings[at].SourceLabel = mapping.SourceLabel
		w.mappings[at].SourceEmail = mapping.SourceEmail
		w.mappings[at].SuggestedTargetID = mapping.SuggestedTargetID
		w.mappings[at].SuggestedReason = mapping.SuggestedReason
	}
}

func (w *world) decideMappings(decisions []entity.ImportMapping, by uuid.UUID, at time.Time) {
	for _, decision := range decisions {
		decided := at
		decision.DecidedByAccount = by
		decision.DecidedAt = &decided

		held := w.mappingAt(decision.Kind, decision.SourceKey)
		if held < 0 {
			w.mappings = append(w.mappings, decision)

			continue
		}

		w.mappings[held].Decision = decision.Decision
		w.mappings[held].TargetID = decision.TargetID
		w.mappings[held].TargetValue = decision.TargetValue
		w.mappings[held].DecidedByAccount = by
		w.mappings[held].DecidedAt = &decided
	}
}

func (w *world) listMappings() []entity.ImportMapping {
	listed := make([]entity.ImportMapping, 0, len(w.mappings))

	for _, mapping := range w.mappings {
		if mapping.DecidedAt == nil {
			mapping.Decision = ""
		}

		listed = append(listed, mapping)
	}

	return listed
}

type harness struct {
	t     *testing.T
	cfg   config.Imports
	world *world

	runs     *importsrepo.MockImportRun
	cursors  *importsrepo.MockImportCursor
	records  *importsrepo.MockImportRecord
	mappings *importsrepo.MockImportMapping
	ledger   *importsrepo.MockImportLedger
	report   *importsrepo.MockImportReport

	members     *membershiprepo.MockMembership
	teams       *teamrepo.MockTeam
	states      *workflowstaterepo.MockWorkflowState
	labels      *labelrepo.MockLabel
	projects    *projectrepo.MockProject
	cycles      *cyclerepo.MockCycle
	issues      *issuerepo.MockIssue
	comments    *issuecommentrepo.MockIssueComment
	relations   *issuerelationrepo.MockIssueRelation
	groups      *labelgrouprepo.MockLabelGroup
	teamMembers *teammemberrepo.MockTeamMember
	triage      *triagerepo.MockTriage
	files       *attachmentrepo.MockAttachment
	blobs       *blobrepo.MockBlob

	issueWriter    *issuesvc.MockIssues
	projectWriter  *projectsvc.MockProjects
	cycleWriter    *cyclesvc.MockCycles
	labelWriter    *labelsvc.MockLabels
	stateWriter    *workflowstatesvc.MockWorkflowStates
	commentWriter  *issuecommentsvc.MockIssueComments
	teamWriter     *teamsvc.MockTeams
	relationWriter *issuerelationsvc.MockIssueRelations
	fileWriter     *attachmentsvc.MockAttachments

	sources    *importssvc.MockImportSources
	authorizer *authorizersvc.MockAuthorizer
	jobs       *jobqueuerepo.MockJobProducer
	transactor *transactorrepo.MockTransactor

	imports service.Imports
	runner  service.ImportRunner
	rescue  service.ImportRescue

	transactions int
	depth        int
	aborted      bool
	savepoints   int
	released     int
	unwound      int
	opened       time.Time
	doomed       int
	refusedBook  error
	heartbeat    func() error
	trace        []string
	committing   map[string]int
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	return newTunedHarness(t, testImportConfig())
}

func newTunedHarness(t *testing.T, cfg config.Imports) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		t:          t,
		cfg:        cfg,
		committing: map[string]int{},
		world: &world{run: entity.ImportRun{
			ID:                 uuid.New(),
			WorkspaceID:        uuid.New(),
			SourceKind:         "fake",
			SourceLabel:        "Northwind",
			RequestedByAccount: uuid.New(),
			RequestedActorKind: entity.ActorKindUser,
			Status:             entity.ImportDraft,
		}},
		runs:           importsrepo.NewMockImportRun(ctrl),
		cursors:        importsrepo.NewMockImportCursor(ctrl),
		records:        importsrepo.NewMockImportRecord(ctrl),
		mappings:       importsrepo.NewMockImportMapping(ctrl),
		ledger:         importsrepo.NewMockImportLedger(ctrl),
		report:         importsrepo.NewMockImportReport(ctrl),
		members:        membershiprepo.NewMockMembership(ctrl),
		teams:          teamrepo.NewMockTeam(ctrl),
		states:         workflowstaterepo.NewMockWorkflowState(ctrl),
		labels:         labelrepo.NewMockLabel(ctrl),
		projects:       projectrepo.NewMockProject(ctrl),
		cycles:         cyclerepo.NewMockCycle(ctrl),
		issues:         issuerepo.NewMockIssue(ctrl),
		comments:       issuecommentrepo.NewMockIssueComment(ctrl),
		relations:      issuerelationrepo.NewMockIssueRelation(ctrl),
		groups:         labelgrouprepo.NewMockLabelGroup(ctrl),
		teamMembers:    teammemberrepo.NewMockTeamMember(ctrl),
		triage:         triagerepo.NewMockTriage(ctrl),
		files:          attachmentrepo.NewMockAttachment(ctrl),
		blobs:          blobrepo.NewMockBlob(ctrl),
		issueWriter:    issuesvc.NewMockIssues(ctrl),
		projectWriter:  projectsvc.NewMockProjects(ctrl),
		cycleWriter:    cyclesvc.NewMockCycles(ctrl),
		labelWriter:    labelsvc.NewMockLabels(ctrl),
		stateWriter:    workflowstatesvc.NewMockWorkflowStates(ctrl),
		commentWriter:  issuecommentsvc.NewMockIssueComments(ctrl),
		teamWriter:     teamsvc.NewMockTeams(ctrl),
		relationWriter: issuerelationsvc.NewMockIssueRelations(ctrl),
		fileWriter:     attachmentsvc.NewMockAttachments(ctrl),
		sources:        importssvc.NewMockImportSources(ctrl),
		authorizer:     authorizersvc.NewMockAuthorizer(ctrl),
		jobs:           jobqueuerepo.NewMockJobProducer(ctrl),
		transactor:     transactorrepo.NewMockTransactor(ctrl),
	}

	h.transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(h.commit).
		AnyTimes()

	h.transactor.EXPECT().
		WithSavepoint(gomock.Any(), gomock.Any()).
		DoAndReturn(h.savepoint).
		AnyTimes()

	built := importssvc.New(
		h.runs, h.cursors, h.records, h.mappings, h.ledger, h.report,
		h.members, h.teams, h.states, h.labels, h.projects, h.cycles, h.issues, h.comments,
		h.relations, h.groups, h.teamMembers, h.triage, h.files, h.blobs,
		h.issueWriter, h.projectWriter, h.cycleWriter, h.labelWriter, h.stateWriter,
		h.commentWriter, h.teamWriter, h.relationWriter, h.fileWriter,
		h.sources, h.authorizer, h.jobs, h.transactor,
		cfg,
	)

	h.imports = importssvc.NewImports(built)
	h.runner = importssvc.NewImportRunner(built)
	h.rescue = importssvc.NewImportRescue(built)

	return h
}

func (h *harness) commit(ctx context.Context, fn func(context.Context) error) error {
	h.transactions++
	h.opened = time.Now().UTC()
	h.depth++

	kept := h.world.snapshot()

	err := fn(ctx)

	h.depth--
	h.aborted = false

	if err == nil && h.transactions == h.doomed {
		err = errLostCommit
	}

	if err != nil {
		h.world.restore(kept)

		return err
	}

	return nil
}

func (h *harness) savepoint(ctx context.Context, fn func(context.Context) error) error {
	if h.depth == 0 {
		return fn(ctx)
	}

	h.savepoints++

	kept := h.world.snapshot()

	if err := fn(ctx); err != nil {
		h.world.restore(kept)
		h.aborted = false
		h.unwound++

		return err
	}

	h.released++

	return nil
}

// statement is what every mocked command inside a chunk answers through, because the thing that
// decides whether the chunk survives a refused row is not the refusal itself but what the
// database says to the command after it.
func (h *harness) statement(err error) error {
	if h.aborted {
		return errTransactionAborted
	}

	if err != nil && h.depth > 0 {
		h.aborted = true
	}

	return err
}

func (h *harness) run() entity.ImportRun { return h.world.run }

func (h *harness) followed(step string) { h.trace = append(h.trace, step) }

func (h *harness) backed() *harness {
	h.t.Helper()

	h.backRuns()
	h.backCursors()
	h.backRecords()
	h.backMappings()
	h.backLedger()

	h.report.EXPECT().
		Record(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, runID uuid.UUID, lines []entity.ImportReportLine) error {
			if err := h.statement(nil); err != nil {
				return err
			}

			h.world.note(runID, lines)

			return nil
		}).
		AnyTimes()

	return h
}

func (h *harness) backRuns() {
	h.runs.EXPECT().
		GetByID(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, workspaceID, runID uuid.UUID) (entity.ImportRun, error) {
			if workspaceID != h.world.run.WorkspaceID || runID != h.world.run.ID {
				return entity.ImportRun{}, entity.ErrImportRunNotFound
			}

			return h.world.run, nil
		}).
		AnyTimes()

	h.runs.EXPECT().
		SaveSourceConfig(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_ uuid.UUID,
			secret string,
			settings json.RawMessage,
			policy entity.ImportUnknownPolicy,
		) error {
			if h.world.keyMissing && secret != "" {
				return entity.ErrImportEncryptionKeyMissing
			}

			if secret != "" {
				h.world.secret = secret
				h.world.run.SourceSecretSet = true
			}

			h.world.run.Settings = settings
			h.world.run.UnknownReferences = policy

			return nil
		}).
		AnyTimes()

	h.runs.EXPECT().
		SourceConfigForStaging(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID) (repository.ImportSourceSecret, error) {
			if h.world.keyMissing {
				return repository.ImportSourceSecret{}, entity.ErrImportEncryptionKeyMissing
			}

			return repository.ImportSourceSecret{
				Secret:   h.world.secret,
				Settings: h.world.run.Settings,
			}, nil
		}).
		AnyTimes()

	h.runs.EXPECT().
		Claim(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_ uuid.UUID,
			from, to entity.ImportStatus,
			lease repository.ImportLease,
			at time.Time,
		) (uuid.UUID, error) {
			held := h.world.run.Status == to &&
				(h.world.run.LeaseExpiresAt == nil || h.world.run.LeaseExpiresAt.Before(at))

			if h.world.run.Status != from && !held {
				return uuid.Nil, entity.ErrImportRunLeased
			}

			expires := lease.Expires

			h.world.run.Status = to
			h.world.run.LeaseToken = lease.Token
			h.world.run.LeaseExpiresAt = &expires
			h.world.run.Attempt++

			return lease.Token, nil
		}).
		AnyTimes()

	h.runs.EXPECT().
		Heartbeat(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, token uuid.UUID, expires time.Time) error {
			if h.heartbeat != nil {
				if err := h.heartbeat(); err != nil {
					return err
				}
			}

			if token != h.world.run.LeaseToken {
				return entity.ErrImportRunLeased
			}

			held := expires
			h.world.run.LeaseExpiresAt = &held

			return nil
		}).
		AnyTimes()

	h.runs.EXPECT().
		Release(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, token uuid.UUID, _ time.Time) error {
			if token != h.world.run.LeaseToken {
				return nil
			}

			h.world.run.LeaseToken = uuid.Nil
			h.world.run.LeaseExpiresAt = nil

			return nil
		}).
		AnyTimes()

	h.runs.EXPECT().
		Advance(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, staged, processed int) error {
			if err := h.statement(nil); err != nil {
				return err
			}

			h.world.advances = append(h.world.advances, advance{staged: staged, processed: processed})
			h.world.run.Staged += staged
			h.world.run.Processed += processed

			return nil
		}).
		AnyTimes()

	h.runs.EXPECT().
		Settle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_ uuid.UUID,
			status entity.ImportStatus,
			detail string,
			_ time.Time,
		) error {
			h.world.run.Status = status
			h.world.run.PhaseError = detail
			h.world.run.LeaseToken = uuid.Nil
			h.world.run.LeaseExpiresAt = nil

			return nil
		}).
		AnyTimes()

	h.runs.EXPECT().
		AcceptPreview(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_ uuid.UUID,
			digest string,
			acknowledgeTriage bool,
			_ time.Time,
		) error {
			h.world.run.PreviewDigest = digest
			h.world.run.AcknowledgeTriage = acknowledgeTriage

			return nil
		}).
		AnyTimes()

	h.runs.EXPECT().
		RecordReverter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, actor entity.Actor, _ time.Time) error {
			h.world.run.RevertedByAccount = actor.AccountID
			h.world.run.RevertedActorKind = actor.Kind
			h.world.run.RevertedAuthMethod = actor.AuthMethod

			return nil
		}).
		AnyTimes()
}

func (h *harness) backCursors() {
	h.cursors.EXPECT().
		List(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID) ([]entity.ImportCursor, error) {
			return slices.Clone(h.world.cursors), nil
		}).
		AnyTimes()

	h.cursors.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cursor entity.ImportCursor) error {
			h.world.saveCursor(cursor)

			return nil
		}).
		AnyTimes()

	h.cursors.EXPECT().
		Park(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_ uuid.UUID,
			resource entity.ImportResource,
			until time.Time,
		) error {
			h.world.park(resource, until)

			return nil
		}).
		AnyTimes()
}

func (h *harness) backRecords() {
	h.records.EXPECT().
		Stage(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, records []entity.ImportRecord) (int, error) {
			for _, record := range records {
				h.committing[string(record.Resource)+"/"+record.ExternalID] = h.transactions
			}

			return h.world.stage(records), nil
		}).
		AnyTimes()

	h.records.EXPECT().
		Walk(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_ uuid.UUID,
			walk entity.ImportWalk,
		) ([]entity.ImportRecord, error) {
			return h.world.walkRecords(walk), nil
		}).
		AnyTimes()

	h.records.EXPECT().
		Settle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_ uuid.UUID,
			resource entity.ImportResource,
			state entity.ImportRecordState,
			outcomes []entity.ImportRecord,
		) ([]string, error) {
			if err := h.statement(nil); err != nil {
				return nil, err
			}

			return h.world.settleRecords(resource, state, outcomes), nil
		}).
		AnyTimes()
}

func (h *harness) backMappings() {
	h.mappings.EXPECT().
		List(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID) ([]entity.ImportMapping, error) {
			return h.world.listMappings(), nil
		}).
		AnyTimes()

	h.mappings.EXPECT().
		Save(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, mappings []entity.ImportMapping) error {
			h.world.saveMappings(mappings)

			return nil
		}).
		AnyTimes()

	h.mappings.EXPECT().
		Decide(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_ uuid.UUID,
			decisions []entity.ImportMapping,
			by uuid.UUID,
			at time.Time,
		) error {
			h.world.decideMappings(decisions, by, at)

			return nil
		}).
		AnyTimes()
}

func (h *harness) backLedger() {
	h.ledger.EXPECT().
		Record(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, runID uuid.UUID, entries []entity.ImportLedgerEntry) error {
			if err := h.statement(h.refusedBook); err != nil {
				return err
			}

			h.world.recordLedger(runID, entries, h.opened)

			return nil
		}).
		AnyTimes()

	h.ledger.EXPECT().
		Walk(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_ uuid.UUID,
			walk entity.ImportLedgerWalk,
		) ([]entity.ImportLedgerEntry, error) {
			return h.world.walkLedger(walk), nil
		}).
		AnyTimes()

	h.ledger.EXPECT().
		Resolve(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_ uuid.UUID,
			resource entity.ImportResource,
			externalIDs []string,
		) (map[string]uuid.UUID, error) {
			return h.world.resolve(resource, externalIDs), nil
		}).
		AnyTimes()

	h.ledger.EXPECT().
		Restamp(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, entries []entity.ImportLedgerEntry) error {
			if err := h.statement(nil); err != nil {
				return err
			}

			h.world.restampLedger(entries)

			return nil
		}).
		AnyTimes()

	h.ledger.EXPECT().
		Settle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_ uuid.UUID,
			entries []entity.ImportLedgerEntry,
			at time.Time,
		) error {
			h.world.settleLedger(entries, at)

			return nil
		}).
		AnyTimes()
}

func (h *harness) offering(source service.ImportSource) *harness {
	h.sources.EXPECT().Lookup(gomock.Any()).Return(source, nil).AnyTimes()

	return h
}

func (h *harness) allow(decision entity.Decision) *harness {
	decision.Actor = entity.Actor{
		Kind:      entity.ActorKindUser,
		AccountID: h.world.run.RequestedByAccount,
	}

	if decision.Role == "" {
		decision.Role = entity.MembershipRoleAdmin
	}

	if decision.Scope.WorkspaceID == uuid.Nil {
		decision.Scope.WorkspaceID = h.world.run.WorkspaceID
	}

	h.authorizer.EXPECT().Decide(gomock.Any(), gomock.Any()).Return(decision, nil).AnyTimes()

	return h
}

func (h *harness) scopedTo(teamIDs ...uuid.UUID) *harness {
	return h.allow(entity.Decision{Scope: entity.TeamScope{
		WorkspaceID:    h.world.run.WorkspaceID,
		IncludePrivate: true,
		TeamIDs:        teamIDs,
	}})
}

func (h *harness) configured(secret string, settings json.RawMessage) *harness {
	h.world.secret = secret
	h.world.run.SourceSecretSet = secret != ""
	h.world.run.Settings = settings

	return h
}

func (h *harness) withoutAnEncryptionKey() *harness {
	h.world.keyMissing = true

	return h
}

func (h *harness) resolving(policy entity.ImportUnknownPolicy) *harness {
	h.world.run.UnknownReferences = policy

	return h
}

func (h *harness) at(status entity.ImportStatus) *harness {
	h.world.run.Status = status

	return h
}

func (h *harness) stage(
	resource entity.ImportResource,
	records ...entity.ImportRecord,
) *harness {
	h.t.Helper()

	staging := slices.Clone(records)

	for index := range staging {
		staging[index].RunID = h.world.run.ID
		staging[index].Resource = resource
	}

	h.world.stage(staging)

	return h
}

func (h *harness) stageEverything(source *staticSource) *harness {
	h.t.Helper()

	for _, resource := range entity.ImportPhases() {
		h.stage(resource, source.held[resource]...)
	}

	return h
}

func (h *harness) workspaceHolds(members ...entity.WorkspaceMember) *harness {
	h.members.EXPECT().
		ListPageByWorkspaceID(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_ uuid.UUID,
			page entity.MembershipPage,
		) ([]entity.WorkspaceMember, error) {
			start := 0

			if page.Cursor != nil {
				start = 1 + slices.IndexFunc(members, func(member entity.WorkspaceMember) bool {
					return member.Membership.AccountID == page.Cursor.AccountID
				})
			}

			return members[start:min(start+page.Limit, len(members))], nil
		}).
		AnyTimes()

	return h
}

func (h *harness) teamsVisible(teams ...entity.Team) *harness {
	h.teams.EXPECT().
		ListVisibleTo(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(teams, nil).
		AnyTimes()

	return h
}

func (h *harness) statesOn(teamID uuid.UUID, states ...entity.WorkflowState) *harness {
	h.states.EXPECT().ListByTeamID(gomock.Any(), teamID).Return(states, nil).AnyTimes()

	return h
}

func (h *harness) labelsHeld(labels ...entity.Label) *harness {
	h.labels.EXPECT().
		ListByWorkspaceID(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(labels, nil).
		AnyTimes()

	return h
}

func (h *harness) alreadyMade(entries ...entity.ImportLedgerEntry) *harness {
	for index := range entries {
		entries[index].WorkspaceID = h.world.run.WorkspaceID
	}

	h.world.recordLedger(h.world.run.ID, entries, time.Now().UTC())

	return h
}

func (h *harness) decided(mappings ...entity.ImportMapping) *harness {
	h.world.decideMappings(mappings, h.world.run.RequestedByAccount, time.Now().UTC())

	return h
}

func (h *harness) forget() *harness {
	h.transactions = 0
	h.savepoints = 0
	h.released = 0
	h.unwound = 0
	h.trace = nil
	h.world.advances = nil
	h.world.saves = nil
	h.world.parks = nil
	h.world.lines = nil
	h.world.ledger = nil

	return h
}

func (h *harness) wroteNothing(during string) {
	h.t.Helper()

	because := "%s wrote %s. It is what the requester is shown before deciding and what Execute " +
		"recomputes to compare digests, so anything it changes it would change again on every " +
		"refresh and on the way into the run itself."

	if h.transactions != 0 {
		h.t.Errorf(because, during, "opened a transaction")
	}

	if len(h.world.ledger) != 0 {
		h.t.Errorf(because, during, "recorded ledger entries")
	}

	if len(h.world.lines) != 0 {
		h.t.Errorf(because, during, "recorded report lines")
	}

	if len(h.world.advances) != 0 {
		h.t.Errorf(because, during, "moved the run's counters")
	}

	if len(h.world.saves) != 0 || len(h.world.parks) != 0 {
		h.t.Errorf(because, during, "moved a cursor")
	}

	for _, record := range h.world.records {
		if record.State != entity.ImportRecordStaged {
			h.t.Errorf(because, during, "settled the record "+record.ExternalID)
		}
	}
}

func (h *harness) linesFor(outcome entity.ImportOutcome) []entity.ImportReportLine {
	found := make([]entity.ImportReportLine, 0, len(h.world.lines))

	for _, line := range h.world.lines {
		if line.Outcome == outcome {
			found = append(found, line)
		}
	}

	return found
}

func (h *harness) lineFor(externalID string) (entity.ImportReportLine, bool) {
	for _, line := range h.world.lines {
		if line.ExternalID == externalID {
			return line, true
		}
	}

	return entity.ImportReportLine{}, false
}

func (h *harness) lineOf(
	resource entity.ImportResource,
	externalID string,
) (entity.ImportReportLine, bool) {
	for _, line := range h.world.lines {
		if line.Resource == resource && line.ExternalID == externalID {
			return line, true
		}
	}

	return entity.ImportReportLine{}, false
}

func (h *harness) stagedIn(resource entity.ImportResource, externalID string) int {
	h.t.Helper()

	at, staged := h.committing[string(resource)+"/"+externalID]
	if !staged {
		h.t.Fatalf("nothing ever staged the %s %q", resource, externalID)
	}

	return at
}

func (h *harness) recordState(
	resource entity.ImportResource,
	externalID string,
) entity.ImportRecordState {
	at := h.world.recordAt(resource, externalID)
	if at < 0 {
		return ""
	}

	return h.world.records[at].State
}

func (h *harness) ledgerFor(resource entity.ImportResource) []entity.ImportLedgerEntry {
	found := make([]entity.ImportLedgerEntry, 0, len(h.world.ledger))

	for _, entry := range h.world.ledger {
		if entry.Resource == resource {
			found = append(found, entry)
		}
	}

	return found
}

func payloadOf(t *testing.T, value any) []byte {
	t.Helper()

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode import payload: %v", err)
	}

	return encoded
}

func teamNamed(name string) entity.Team {
	return entity.Team{
		ID:     uuid.New(),
		Key:    "CORE",
		Name:   name,
		Status: entity.TeamStatusActive,
	}
}

func memberNamed(display, email string) entity.WorkspaceMember {
	return entity.WorkspaceMember{
		Membership:  entity.Membership{AccountID: uuid.New(), Role: entity.MembershipRoleMember},
		DisplayName: display,
		Email:       email,
		SortName:    display,
	}
}
