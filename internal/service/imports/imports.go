package imports

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type importsService struct {
	runs     repository.ImportRun
	cursors  repository.ImportCursor
	records  repository.ImportRecord
	mappings repository.ImportMapping
	ledger   repository.ImportLedger
	report   repository.ImportReport

	members     repository.Membership
	teams       repository.Team
	states      repository.WorkflowState
	labels      repository.Label
	projects    repository.Project
	cycles      repository.Cycle
	issues      repository.Issue
	comments    repository.IssueComment
	relations   repository.IssueRelation
	groups      repository.LabelGroup
	teamMembers repository.TeamMember
	triage      repository.Triage
	files       repository.Attachment
	blobs       repository.Blob

	issueWriter    service.Issues
	projectWriter  service.Projects
	cycleWriter    service.Cycles
	labelWriter    service.Labels
	stateWriter    service.WorkflowStates
	commentWriter  service.IssueComments
	teamWriter     service.Teams
	relationWriter service.IssueRelations
	fileWriter     service.Attachments

	sources    service.ImportSources
	authorizer service.Authorizer
	jobs       repository.JobProducer
	transactor repository.Transactor
	cfg        config.Imports
}

func New(
	runs repository.ImportRun,
	cursors repository.ImportCursor,
	records repository.ImportRecord,
	mappings repository.ImportMapping,
	ledger repository.ImportLedger,
	report repository.ImportReport,
	members repository.Membership,
	teams repository.Team,
	states repository.WorkflowState,
	labels repository.Label,
	projects repository.Project,
	cycles repository.Cycle,
	issues repository.Issue,
	comments repository.IssueComment,
	relations repository.IssueRelation,
	groups repository.LabelGroup,
	teamMembers repository.TeamMember,
	triage repository.Triage,
	files repository.Attachment,
	blobs repository.Blob,
	issueWriter service.Issues,
	projectWriter service.Projects,
	cycleWriter service.Cycles,
	labelWriter service.Labels,
	stateWriter service.WorkflowStates,
	commentWriter service.IssueComments,
	teamWriter service.Teams,
	relationWriter service.IssueRelations,
	fileWriter service.Attachments,
	sources service.ImportSources,
	authorizer service.Authorizer,
	jobs repository.JobProducer,
	transactor repository.Transactor,
	cfg config.Imports,
) *importsService {
	return &importsService{
		runs:           runs,
		cursors:        cursors,
		records:        records,
		mappings:       mappings,
		ledger:         ledger,
		report:         report,
		members:        members,
		teams:          teams,
		states:         states,
		labels:         labels,
		projects:       projects,
		cycles:         cycles,
		issues:         issues,
		comments:       comments,
		relations:      relations,
		groups:         groups,
		teamMembers:    teamMembers,
		triage:         triage,
		files:          files,
		blobs:          blobs,
		issueWriter:    issueWriter,
		projectWriter:  projectWriter,
		cycleWriter:    cycleWriter,
		labelWriter:    labelWriter,
		stateWriter:    stateWriter,
		commentWriter:  commentWriter,
		teamWriter:     teamWriter,
		relationWriter: relationWriter,
		fileWriter:     fileWriter,
		sources:        sources,
		authorizer:     authorizer,
		jobs:           jobs,
		transactor:     transactor,
		cfg:            cfg,
	}
}

func NewImports(imports *importsService) service.Imports { return imports }

func NewImportRunner(imports *importsService) service.ImportRunner { return imports }

func NewImportRescue(imports *importsService) service.ImportRescue { return imports }

func (s *importsService) Connect(ctx context.Context, input service.CreateImportInput) (entity.ImportRun, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionManage,
		WorkspaceID: input.WorkspaceID,
		Scoped:      true,
	})
	if err != nil {
		return entity.ImportRun{}, err
	}

	kind := strings.TrimSpace(input.SourceKind)

	if kind == "" || len(kind) > entity.ImportSourceKindMax {
		return entity.ImportRun{}, entity.NewValidationError(entity.FieldError{
			Field: "sourceKind",
			Code:  entity.ValidationCodeRequired,
		})
	}

	if _, err := s.sources.Lookup(kind); err != nil {
		return entity.ImportRun{}, err
	}

	now := time.Now().UTC()

	run, err := s.runs.Create(ctx, entity.ImportRun{
		WorkspaceID:         input.WorkspaceID,
		SourceKind:          kind,
		SourceLabel:         strings.TrimSpace(input.SourceLabel),
		RequestedByAccount:  decision.Actor.AccountID,
		RequestedActorKind:  decision.Actor.Kind,
		RequestedAuthMethod: decision.Actor.AuthMethod,
		Authority:           entity.AuthorityOf(decision.Actor, input.WorkspaceID),
		Status:              entity.ImportDraft,
		CreatedAt:           now,
		UpdatedAt:           now,
	})
	if err != nil {
		return entity.ImportRun{}, err
	}

	return run, nil
}

func (s *importsService) Configure(
	ctx context.Context,
	workspaceID, runID uuid.UUID,
	input service.ConfigureImportInput,
) (entity.ImportRun, error) {
	if _, err := s.decide(ctx, workspaceID); err != nil {
		return entity.ImportRun{}, err
	}

	run, err := s.runs.GetByID(ctx, workspaceID, runID)
	if err != nil {
		return entity.ImportRun{}, err
	}

	if !run.Status.Configurable() {
		return entity.ImportRun{}, entity.ErrImportStatusTransition
	}

	if len(input.Settings) > 0 && !json.Valid(input.Settings) {
		return entity.ImportRun{}, entity.NewValidationError(entity.FieldError{
			Field: "settings",
			Code:  entity.ValidationCodeMalformed,
		})
	}

	policy := input.UnknownReferences.Or(entity.ImportUnknownSkip)

	if !policy.Valid() {
		return entity.ImportRun{}, entity.NewValidationError(entity.FieldError{
			Field: "unknownReferences",
			Code:  entity.ValidationCodeUnsupportedValue,
		})
	}

	if err := s.runs.SaveSourceConfig(ctx, run.ID, input.Secret, input.Settings, policy); err != nil {
		return entity.ImportRun{}, err
	}

	return s.runs.GetByID(ctx, workspaceID, runID)
}

func (s *importsService) Get(ctx context.Context, workspaceID, runID uuid.UUID) (entity.ImportRun, error) {
	if _, err := s.decide(ctx, workspaceID); err != nil {
		return entity.ImportRun{}, err
	}

	return s.runs.GetByID(ctx, workspaceID, runID)
}

func (s *importsService) List(
	ctx context.Context,
	workspaceID uuid.UUID,
	page entity.ImportRunPage,
) ([]entity.ImportRun, error) {
	if _, err := s.decide(ctx, workspaceID); err != nil {
		return nil, err
	}

	return s.runs.ListByWorkspace(ctx, workspaceID, page)
}

func (s *importsService) Sources(ctx context.Context, workspaceID uuid.UUID) ([]string, error) {
	if _, err := s.decide(ctx, workspaceID); err != nil {
		return nil, err
	}

	return s.sources.Kinds(), nil
}

func (s *importsService) Report(
	ctx context.Context,
	workspaceID, runID uuid.UUID,
) (service.ImportReportView, error) {
	if _, err := s.decide(ctx, workspaceID); err != nil {
		return service.ImportReportView{}, err
	}

	run, err := s.runs.GetByID(ctx, workspaceID, runID)
	if err != nil {
		return service.ImportReportView{}, err
	}

	created, err := s.ledger.Walk(ctx, run.ID, entity.ImportLedgerWalk{Limit: s.cfg.PageSize})
	if err != nil {
		return service.ImportReportView{}, err
	}

	view := service.ImportReportView{Run: run, Created: created}

	if len(created) == s.cfg.PageSize {
		view.NextCreatedCursor = created[len(created)-1].OrderSeq
	}

	view.Lines = make([]entity.ImportReportLine, 0, s.cfg.PageSize)

	for _, phase := range entity.ImportPhaseNames() {
		recorded, err := s.report.List(ctx, run.ID, phase, nil, s.cfg.PageSize)
		if err != nil {
			return service.ImportReportView{}, err
		}

		view.Lines = append(view.Lines, recorded...)

		if len(recorded) == s.cfg.PageSize && view.NextLineCursor == nil {
			last := recorded[len(recorded)-1]
			view.NextLineCursor = &entity.ImportReportCursor{RecordedAt: last.RecordedAt, ID: last.ID}
		}
	}

	return view, nil
}

func (s *importsService) decide(ctx context.Context, workspaceID uuid.UUID) (entity.Decision, error) {
	return s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionManage,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
}

func (s *importsService) lease(
	ctx context.Context,
	run entity.ImportRun,
	from, to entity.ImportStatus,
) (uuid.UUID, time.Time, error) {
	now := time.Now().UTC()
	expires := now.Add(s.cfg.LeaseTTL)

	token, err := s.runs.Claim(ctx, run.ID, from, to, repository.ImportLease{
		RunID:   run.ID,
		Token:   uuid.New(),
		Status:  to,
		Expires: expires,
	}, now)
	if err != nil {
		return uuid.Nil, time.Time{}, err
	}

	return token, expires, nil
}

func (s *importsService) beat(ctx context.Context, runID, token uuid.UUID) error {
	return s.runs.Heartbeat(ctx, runID, token, time.Now().UTC().Add(s.cfg.LeaseTTL))
}

func (s *importsService) fail(ctx context.Context, runID uuid.UUID, cause error) error {
	if err := s.runs.Settle(ctx, runID, entity.ImportFailed, cause.Error(), time.Now().UTC()); err != nil {
		return fmt.Errorf("settle failed import: %w", err)
	}

	return cause
}

func (s *importsService) eachRecord(
	ctx context.Context,
	runID uuid.UUID,
	resource entity.ImportResource,
	visit func(entity.ImportRecord) error,
) error {
	after := int64(0)

	for {
		records, err := s.records.Walk(ctx, runID, entity.ImportWalk{
			Resource: resource,
			After:    after,
			Limit:    s.cfg.ChunkSize,
		})
		if err != nil {
			return err
		}

		if len(records) == 0 {
			return nil
		}

		for _, record := range records {
			if err := visit(record); err != nil {
				return err
			}
		}

		after = records[len(records)-1].Seq
	}
}

func (s *importsService) workspaceTeams(
	ctx context.Context,
	workspaceID uuid.UUID,
	decision entity.Decision,
) ([]entity.Team, error) {
	teams, err := s.teams.ListVisibleTo(
		ctx, workspaceID, decision.Actor.AccountID, entity.TeamStatusActive, decision.Scope.IncludePrivate,
	)
	if err != nil {
		return nil, err
	}

	return slices.DeleteFunc(teams, func(team entity.Team) bool {
		return !decision.Scope.Covers(team.ID)
	}), nil
}

func decodePayload[T any](record entity.ImportRecord) (T, error) {
	var payload T

	if len(record.Payload) == 0 {
		return payload, nil
	}

	if err := json.Unmarshal(record.Payload, &payload); err != nil {
		return payload, fmt.Errorf(
			"decode %s payload for %q: %w", record.Resource, record.ExternalID, err,
		)
	}

	return payload, nil
}

func detailOf(line entity.ImportReportLine) string {
	if len(line.Detail) == 0 {
		return ""
	}

	keys := make([]string, 0, len(line.Detail))
	for key := range line.Detail {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+": "+line.Detail[key])
	}

	return strings.Join(parts, "; ")
}
