package imports

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

type references struct {
	teams    map[string]uuid.UUID
	states   map[string]uuid.UUID
	groups   map[string]uuid.UUID
	labels   map[string]uuid.UUID
	projects map[string]uuid.UUID
	cycles   map[string]uuid.UUID
	issues   map[string]uuid.UUID
	comments map[string]uuid.UUID
	files    map[string]uuid.UUID
}

// stamped is the timestamp the created row itself carries, and the ledger keeps it so a
// revert can tell an edit somebody made afterwards from the row as this run left it. It has
// to be read back off the row rather than defaulted in the database: an import writes its
// rows with a Go clock read inside the transaction, and now() is the transaction's start
// time, so a ledger row that dates itself makes every row it names look edited a moment
// later by somebody who was never there.
type outcome struct {
	subject   string
	id        uuid.UUID
	reference string
	version   int
	stamped   time.Time
	reason    string
}

func (o outcome) left() bool {
	return o.reason != ""
}

func (s *importsService) applyChunk(
	ctx context.Context,
	run entity.ImportRun,
	decision entity.Decision,
	plan entity.MappingPlan,
	resource entity.ImportResource,
	records []entity.ImportRecord,
	log *recorder,
) error {
	log.reset()

	return s.transactor.WithTx(ctx, func(ctx context.Context) error {
		found, err := s.referenced(ctx, run, records)
		if err != nil {
			return err
		}

		for _, record := range records {
			s.applyRecord(ctx, run, decision, plan, found, record, log)
		}

		if err := s.ledger.Restamp(ctx, run.ID, log.restamped); err != nil {
			return err
		}

		if err := s.ledger.Record(ctx, run.ID, log.ledger); err != nil {
			return err
		}

		processed, err := s.settleChunk(ctx, run, resource, log)
		if err != nil {
			return err
		}

		if err := s.report.Record(ctx, run.ID, log.lines); err != nil {
			return err
		}

		return s.runs.Advance(ctx, run.ID, 0, processed)
	})
}

func (s *importsService) settleChunk(
	ctx context.Context,
	run entity.ImportRun,
	resource entity.ImportResource,
	log *recorder,
) (int, error) {
	processed := 0

	for _, state := range entity.ImportRecordStates() {
		if state == entity.ImportRecordStaged {
			continue
		}

		settling := make([]entity.ImportRecord, 0, len(log.applied))

		for _, settled := range log.applied {
			if settled.State == state {
				settling = append(settling, settled)
			}
		}

		returned, err := s.records.Settle(ctx, run.ID, resource, state, settling)
		if err != nil {
			return 0, err
		}

		processed += len(returned)
	}

	return processed, nil
}

// applyRecord treats what one row costs as an outcome rather than an error, and that only
// holds while the transaction the chunk runs in survives the row. Postgres refuses every
// command after a failed statement until it is rolled back, so a name the workspace already
// holds would otherwise take the ledger, the settlement and the counter down with it and leave
// the whole chunk to be retried into the same collision. The savepoint is what makes the rest
// of the chunk reachable.
func (s *importsService) applyRecord(
	ctx context.Context,
	run entity.ImportRun,
	decision entity.Decision,
	plan entity.MappingPlan,
	found *references,
	record entity.ImportRecord,
	log *recorder,
) {
	var made outcome

	err := s.transactor.WithSavepoint(ctx, func(ctx context.Context) error {
		var err error

		made, err = s.create(ctx, run, decision, plan, found, record, log)

		return err
	})

	switch {
	case err != nil:
		failure := entity.OutcomeForImport(err)

		log.note(record.Resource, record.ExternalID, made.subject, failure,
			map[string]string{"reason": err.Error()})
		log.settled(entity.ImportRecord{
			ExternalID:    record.ExternalID,
			State:         settledAs(failure),
			OutcomeDetail: truncate(err.Error(), entity.ImportSubjectMax),
		})

	case made.left():
		log.skipped(record.Resource, record.ExternalID, made.subject, made.reason)
		log.settled(entity.ImportRecord{
			ExternalID:    record.ExternalID,
			State:         entity.ImportRecordSkipped,
			OutcomeDetail: truncate(made.reason, entity.ImportSubjectMax),
		})

	default:
		log.note(record.Resource, record.ExternalID, made.subject, entity.ImportOutcomeCreated, nil)
		log.created(entity.ImportLedgerEntry{
			WorkspaceID:       run.WorkspaceID,
			Resource:          record.Resource,
			CreatedID:         made.id,
			ExternalID:        record.ExternalID,
			Reference:         made.reference,
			VersionAtCreate:   made.version,
			CreatedAtRecorded: made.stamped,
		})
		log.settled(entity.ImportRecord{
			ExternalID: record.ExternalID,
			State:      entity.ImportRecordApplied,
			CreatedID:  made.id,
		})

		found.remember(record.Resource, record.ExternalID, made.id)
	}
}

func settledAs(failure entity.ImportOutcome) entity.ImportRecordState {
	if failure == entity.ImportOutcomeFailed {
		return entity.ImportRecordFailed
	}

	return entity.ImportRecordSkipped
}

func (s *importsService) create(
	ctx context.Context,
	run entity.ImportRun,
	decision entity.Decision,
	plan entity.MappingPlan,
	found *references,
	record entity.ImportRecord,
	log *recorder,
) (outcome, error) {
	switch record.Resource {
	case entity.ImportTeam:
		return s.createTeam(ctx, run, plan, record)
	case entity.ImportWorkflowState:
		return s.createState(ctx, run, plan, found, record, log)
	case entity.ImportLabelGroup:
		return s.createLabelGroup(ctx, run, record)
	case entity.ImportLabel:
		return s.createLabel(ctx, run, plan, found, record, log)
	case entity.ImportProject:
		return s.createProject(ctx, run, plan, record, log)
	case entity.ImportCycle:
		return s.createCycle(ctx, run, plan, found, record, log)
	case entity.ImportIssue:
		return s.createIssue(ctx, run, decision, plan, found, record, log)
	case entity.ImportIssueParent:
		return s.linkParent(ctx, run, decision, found, record)
	case entity.ImportIssueRelation:
		return s.linkRelation(ctx, run, found, record)
	case entity.ImportComment:
		return s.postComment(ctx, run, plan, found, record, log)
	case entity.ImportAttachment:
		return s.createAttachment(ctx, run, found, record)
	case entity.ImportEmbed:
		return s.applyEmbed(ctx, run, decision, found, record, log)
	default:
		return outcome{reason: reasonLeftBehind}, nil
	}
}

func (s *importsService) createTeam(
	ctx context.Context,
	run entity.ImportRun,
	plan entity.MappingPlan,
	record entity.ImportRecord,
) (outcome, error) {
	payload, err := decodePayload[service.ImportTeamPayload](record)
	if err != nil {
		return outcome{}, err
	}

	made := outcome{subject: named(payload.Name, payload.Key)}

	if reason := untaken(plan, entity.ImportMapTeam, record.ExternalID); reason != "" {
		made.reason = reason

		return made, nil
	}

	team, err := s.teamWriter.Create(ctx, service.CreateTeamInput{
		WorkspaceID: run.WorkspaceID,
		Key:         payload.Key,
		Name:        payload.Name,
	})
	if err != nil {
		return made, err
	}

	made.id = team.ID
	made.reference = team.Key
	made.stamped = team.UpdatedAt

	return made, nil
}

func (s *importsService) createState(
	ctx context.Context,
	run entity.ImportRun,
	plan entity.MappingPlan,
	found *references,
	record entity.ImportRecord,
	log *recorder,
) (outcome, error) {
	payload, err := decodePayload[service.ImportWorkflowStatePayload](record)
	if err != nil {
		return outcome{}, err
	}

	made := outcome{subject: payload.Name}

	if reason := untaken(plan, entity.ImportMapState, record.ExternalID); reason != "" {
		made.reason = reason

		return made, nil
	}

	teamID, reachable := target(plan, entity.ImportMapTeam, found.teams, payload.Team)
	if !reachable {
		made.reason = reasonTeamLeftBehind

		return made, nil
	}

	category := entity.StateCategory(payload.Category)

	if !category.Valid() {
		log.adjusted(record.Resource, record.ExternalID, payload.Name,
			payload.Category, string(entity.StateCategoryNotStarted))

		category = entity.StateCategoryNotStarted
	}

	origin := entity.NewImportOrigin(at(record.SourceCreatedAt), at(record.SourceUpdatedAt), uuid.Nil)

	state, err := s.stateWriter.Create(ctx, service.CreateWorkflowStateInput{
		WorkspaceID: run.WorkspaceID,
		TeamID:      teamID,
		Name:        payload.Name,
		Category:    category,
		Origin:      &origin,
	})
	if err != nil {
		return made, err
	}

	made.id = state.ID
	made.reference = state.Name
	made.stamped = state.UpdatedAt

	return made, nil
}

func (s *importsService) createLabelGroup(
	ctx context.Context,
	run entity.ImportRun,
	record entity.ImportRecord,
) (outcome, error) {
	payload, err := decodePayload[service.ImportLabelGroupPayload](record)
	if err != nil {
		return outcome{}, err
	}

	made := outcome{subject: payload.Name}

	group, err := s.labelWriter.CreateGroup(ctx, run.WorkspaceID, payload.Name)
	if err != nil {
		return made, err
	}

	made.id = group.ID
	made.reference = group.Name
	made.stamped = group.UpdatedAt

	return made, nil
}

func (s *importsService) createLabel(
	ctx context.Context,
	run entity.ImportRun,
	plan entity.MappingPlan,
	found *references,
	record entity.ImportRecord,
	log *recorder,
) (outcome, error) {
	payload, err := decodePayload[service.ImportLabelPayload](record)
	if err != nil {
		return outcome{}, err
	}

	made := outcome{subject: payload.Name}

	if reason := untaken(plan, entity.ImportMapLabel, record.ExternalID); reason != "" {
		made.reason = reason

		return made, nil
	}

	teamID := uuid.Nil

	if strings.TrimSpace(payload.Team) != "" {
		reachable := false

		if teamID, reachable = target(plan, entity.ImportMapTeam, found.teams, payload.Team); !reachable {
			made.reason = reasonTeamLeftBehind

			return made, nil
		}
	}

	colour := entity.NearestLabelColor(payload.Color)

	if strings.TrimSpace(payload.Color) != "" && payload.Color != string(colour) {
		log.adjusted(record.Resource, record.ExternalID, payload.Name, payload.Color, string(colour))
	}

	origin := entity.NewImportOrigin(at(record.SourceCreatedAt), at(record.SourceUpdatedAt), uuid.Nil)

	label, err := s.labelWriter.Create(ctx, service.CreateLabelInput{
		WorkspaceID: run.WorkspaceID,
		TeamID:      teamID,
		GroupID:     found.groups[payload.Group],
		Name:        payload.Name,
		Color:       colour,
		Origin:      &origin,
	})
	if err != nil {
		return made, err
	}

	made.id = label.ID
	made.reference = label.Name
	made.stamped = label.UpdatedAt

	return made, nil
}

func (s *importsService) createProject(
	ctx context.Context,
	run entity.ImportRun,
	plan entity.MappingPlan,
	record entity.ImportRecord,
	log *recorder,
) (outcome, error) {
	payload, err := decodePayload[service.ImportProjectPayload](record)
	if err != nil {
		return outcome{}, err
	}

	made := outcome{subject: named(payload.Name, payload.Slug)}

	if reason := untaken(plan, entity.ImportMapProject, record.ExternalID); reason != "" {
		made.reason = reason

		return made, nil
	}

	lead := accountFor(plan, payload.Lead)

	if payload.Lead.Named() && lead == uuid.Nil {
		log.unattributed(record.Resource, record.ExternalID, made.subject, personNamed(payload.Lead))
	}

	origin := entity.NewImportOrigin(at(record.SourceCreatedAt), at(record.SourceUpdatedAt), lead)

	input := service.CreateProjectInput{
		WorkspaceID: run.WorkspaceID,
		Slug:        payload.Slug,
		Name:        payload.Name,
		Description: payload.Description,
		Origin:      &origin,
	}

	if lead != uuid.Nil {
		input.LeadAccountID = &lead
	}

	view, err := s.projectWriter.Create(ctx, input)
	if err != nil {
		return made, err
	}

	made.id = view.Project.ID
	made.reference = view.Project.Slug
	made.stamped = view.Project.UpdatedAt

	return made, nil
}

func (s *importsService) createCycle(
	ctx context.Context,
	run entity.ImportRun,
	plan entity.MappingPlan,
	found *references,
	record entity.ImportRecord,
	log *recorder,
) (outcome, error) {
	payload, err := decodePayload[service.ImportCyclePayload](record)
	if err != nil {
		return outcome{}, err
	}

	made := outcome{subject: cycleNamed(payload)}

	teamID, reachable := target(plan, entity.ImportMapTeam, found.teams, payload.Team)
	if !reachable {
		made.reason = reasonTeamLeftBehind

		return made, nil
	}

	origin := cycleOrigin(record, payload)

	view, err := s.cycleWriter.Create(ctx, service.CreateCycleInput{
		WorkspaceID: run.WorkspaceID,
		TeamID:      teamID,
		StartsOn:    payload.StartsOn,
		EndsOn:      payload.EndsOn,
		ClosedAt:    closedAt(payload, time.Now().UTC()),
		Origin:      &origin,
	})
	if err != nil {
		return made, err
	}

	if strings.TrimSpace(payload.Name) != "" {
		log.dropped(record.Resource, record.ExternalID, made.subject, reasonCycleNameDropped)
	}

	if payload.Number != 0 && payload.Number != view.Cycle.Number {
		log.adjusted(record.Resource, record.ExternalID, made.subject,
			strconv.Itoa(payload.Number), strconv.Itoa(view.Cycle.Number))
	}

	made.id = view.Cycle.ID
	made.reference = made.subject
	made.stamped = view.Cycle.UpdatedAt

	return made, nil
}

// closedAt keeps a cycle whose window has already run out from arriving open. A team is
// shown its current cycle, and an open cycle with a past end date answers that question
// forever, so every dashboard would sit on a sprint the source finished years ago.
// cycleOrigin falls back to the day the cycle started when the source did not say when it
// was created. Creating a cycle needs an attributed origin, and plenty of sources — a CSV
// above all — carry a sprint's window without carrying its paperwork; refusing those would
// mean a team's cycles arrive only if their exporter happened to include a field nobody
// looks at. A cycle existed no later than the day it began, so this is derived rather than
// invented.
func cycleOrigin(record entity.ImportRecord, payload service.ImportCyclePayload) entity.ImportOrigin {
	created, updated := at(record.SourceCreatedAt), at(record.SourceUpdatedAt)

	if created.IsZero() {
		if started, err := entity.ParseCalendarDate(payload.StartsOn); err == nil {
			created = started
		}
	}

	return entity.NewImportOrigin(created, updated, uuid.Nil)
}

func closedAt(payload service.ImportCyclePayload, now time.Time) *time.Time {
	day := payload.ClosedOn

	if strings.TrimSpace(day) == "" {
		if payload.EndsOn >= entity.FormatCalendarDate(now) {
			return nil
		}

		day = payload.EndsOn
	}

	ended, err := entity.ParseCalendarDate(day)
	if err != nil {
		return nil
	}

	closed := ended.AddDate(0, 0, 1)

	return &closed
}

func cycleNamed(payload service.ImportCyclePayload) string {
	return named(payload.Name, payload.StartsOn+" → "+payload.EndsOn)
}

// leftUnattributed is true only where somebody decided to leave this person off. A person
// nobody decided about is a reference that did not arrive, and the unknown-reference policy
// has already said what that costs — reporting it twice would tell the requester a row lost
// its owner deliberately when in fact nothing about them was ever answered.
func leftUnattributed(plan entity.MappingPlan, person service.ImportUser) bool {
	mapping, found := plan.Lookup(entity.ImportMapUser, person.Key)

	return found && mapping.Decision == entity.ImportDecisionUnattributed
}

func (s *importsService) createIssue(
	ctx context.Context,
	run entity.ImportRun,
	decision entity.Decision,
	plan entity.MappingPlan,
	found *references,
	record entity.ImportRecord,
	log *recorder,
) (outcome, error) {
	payload, err := decodePayload[service.ImportIssuePayload](record)
	if err != nil {
		return outcome{}, err
	}

	made := outcome{subject: payload.Title}

	if payload.Defect != "" {
		made.reason = payload.Defect

		return made, nil
	}

	teamID, reachable := target(plan, entity.ImportMapTeam, found.teams, payload.Team)
	if !reachable {
		made.reason = reasonTeamLeftBehind

		return made, nil
	}

	if !decision.Scope.Covers(teamID) {
		made.reason = reasonTeamNotVisible

		return made, nil
	}

	stateID, _ := target(plan, entity.ImportMapState, found.states, payload.State)
	projectID, _ := target(plan, entity.ImportMapProject, found.projects, payload.Project)
	cycleID := found.cycles[payload.Cycle]
	assignee := accountFor(plan, payload.Assignee)

	labelIDs := make([]uuid.UUID, 0, len(payload.Labels))
	lost := make([]entity.ImportUnknownReferenceError, 0, len(payload.Labels))

	lost = missing(lost, plan, entity.ImportMapState, referenceState, payload.State, stateID)
	lost = missing(lost, plan, entity.ImportMapProject, referenceProject, payload.Project, projectID)

	if strings.TrimSpace(payload.Cycle) != "" && cycleID == uuid.Nil {
		lost = append(lost, entity.ImportUnknownReferenceError{
			Kind: referenceCycle, Reference: payload.Cycle,
		})
	}

	if unknownAssignee(plan, payload.Assignee) {
		lost = append(lost, entity.ImportUnknownReferenceError{
			Kind: referenceAssignee, Reference: payload.Assignee.Key,
		})
	}

	for _, key := range payload.Labels {
		labelID, carried := target(plan, entity.ImportMapLabel, found.labels, key)
		if carried {
			labelIDs = append(labelIDs, labelID)

			continue
		}

		lost = missing(lost, plan, entity.ImportMapLabel, referenceLabel, key, uuid.Nil)
	}

	if len(lost) > 0 {
		switch run.UnknownReferences.Or(entity.ImportUnknownSkip) {
		case entity.ImportUnknownSkip:
			made.reason = lost[0].Error()

			return made, nil

		case entity.ImportUnknownFail:
			return made, lost[0]

		default:
			for _, unknown := range lost {
				log.dropped(record.Resource, record.ExternalID, payload.Title, unknown.Error())
			}
		}
	}

	author := accountFor(plan, payload.Author)

	// Both ends are reported, once each, because an issue arriving with nobody on it is the
	// loss the mapping stage exists to make deliberate. A source that names only an assignee —
	// a file of rows usually does — would otherwise drop that person with the report silent.
	// The preview says exactly this too; a line one of them writes the other has to write.
	said := make(map[string]bool, 2)

	for _, absent := range []service.ImportUser{payload.Author, payload.Assignee} {
		if !absent.Named() || said[absent.Key] || !leftUnattributed(plan, absent) {
			continue
		}

		said[absent.Key] = true

		log.unattributed(record.Resource, record.ExternalID, payload.Title, personNamed(absent))
	}

	origin := entity.NewImportOrigin(at(record.SourceCreatedAt), at(record.SourceUpdatedAt), author)

	issue, err := s.issueWriter.Create(ctx, service.CreateIssueInput{
		WorkspaceID:       run.WorkspaceID,
		TeamID:            teamID,
		Title:             payload.Title,
		Description:       payload.Description,
		Priority:          priorityFor(plan, payload.Priority),
		AssigneeAccountID: assignee,
		Estimate:          payload.Estimate,
		DueOn:             payload.DueOn,
		StateID:           stateID,
		CycleID:           cycleID,
		ProjectID:         projectID,
		LabelIDs:          labelIDs,
		Origin:            &origin,
	})
	if err != nil {
		return made, err
	}

	made.id = issue.ID
	made.reference = issue.Reference()
	made.version = issue.Version
	made.stamped = issue.UpdatedAt

	return made, nil
}

func (s *importsService) linkParent(
	ctx context.Context,
	run entity.ImportRun,
	decision entity.Decision,
	found *references,
	record entity.ImportRecord,
) (outcome, error) {
	payload, err := decodePayload[service.ImportIssueParentPayload](record)
	if err != nil {
		return outcome{}, err
	}

	made := outcome{subject: payload.Issue + " → " + payload.Parent}

	childID, carried := found.issues[payload.Issue]
	parentID, held := found.issues[payload.Parent]

	if !carried || !held {
		made.reason = reasonIssueMissing

		return made, nil
	}

	child, err := s.issues.GetVisible(ctx, run.WorkspaceID, childID, decision.Scope)
	if err != nil {
		return made, err
	}

	reparented, err := s.issueWriter.SetParent(ctx, run.WorkspaceID, childID, service.SetIssueParentInput{
		ExpectedVersion: child.Version,
		ParentID:        &parentID,
	})
	if err != nil {
		return made, err
	}

	made.id = reparented.ID
	made.reference = reparented.Reference()
	made.version = reparented.Version
	made.stamped = reparented.UpdatedAt

	return made, nil
}

func (s *importsService) linkRelation(
	ctx context.Context,
	run entity.ImportRun,
	found *references,
	record entity.ImportRecord,
) (outcome, error) {
	payload, err := decodePayload[service.ImportIssueRelationPayload](record)
	if err != nil {
		return outcome{}, err
	}

	made := outcome{subject: payload.Issue + " " + payload.Kind + " " + payload.Related}

	issueID, carried := found.issues[payload.Issue]
	relatedID, held := found.issues[payload.Related]

	if !carried || !held {
		made.reason = reasonIssueMissing

		return made, nil
	}

	relation, err := s.relationWriter.Add(ctx, run.WorkspaceID, issueID, service.AddIssueRelationInput{
		Kind:          entity.IssueRelationView(payload.Kind),
		CounterpartID: relatedID,
	})
	if err != nil {
		return made, err
	}

	made.id = relation.ID
	made.reference = made.subject
	made.stamped = relation.CreatedAt

	return made, nil
}

func (s *importsService) postComment(
	ctx context.Context,
	run entity.ImportRun,
	plan entity.MappingPlan,
	found *references,
	record entity.ImportRecord,
	log *recorder,
) (outcome, error) {
	payload, err := decodePayload[service.ImportCommentPayload](record)
	if err != nil {
		return outcome{}, err
	}

	made := outcome{subject: payload.Body}

	issueID, carried := found.issues[payload.Issue]
	if !carried {
		made.reason = reasonIssueMissing

		return made, nil
	}

	author := accountFor(plan, payload.Author)

	if payload.Author.Named() && author == uuid.Nil {
		log.unattributed(record.Resource, record.ExternalID, payload.Body, personNamed(payload.Author))
	}

	origin := entity.NewImportOrigin(at(record.SourceCreatedAt), at(record.SourceUpdatedAt), author)

	posted, err := s.commentWriter.Post(ctx, run.WorkspaceID, issueID, service.PostCommentInput{
		ParentCommentID: found.comments[payload.Parent],
		Body:            payload.Body,
		Origin:          &origin,
	})
	if err != nil {
		return made, err
	}

	made.id = posted.Comment.ID
	made.reference = posted.Comment.ID.String()
	made.stamped = posted.Comment.UpdatedAt

	return made, nil
}

func (s *importsService) createAttachment(
	ctx context.Context,
	run entity.ImportRun,
	found *references,
	record entity.ImportRecord,
) (outcome, error) {
	payload, err := decodePayload[service.ImportAttachmentPayload](record)
	if err != nil {
		return outcome{}, err
	}

	made := outcome{subject: named(payload.FileName, payload.SourceURL)}

	issueID, carried := found.issues[payload.Issue]
	if !carried {
		made.reason = reasonIssueMissing

		return made, nil
	}

	commentID := uuid.Nil

	if strings.TrimSpace(payload.Comment) != "" {
		if commentID, carried = found.comments[payload.Comment]; !carried {
			made.reason = reasonCommentMissing

			return made, nil
		}
	}

	origin := entity.NewImportOrigin(at(record.SourceCreatedAt), at(record.SourceUpdatedAt), uuid.Nil)

	stored, err := s.fileWriter.Adopt(ctx, run.WorkspaceID, issueID, service.AdoptAttachmentInput{
		ObjectKey:   payload.ObjectKey,
		FileName:    payload.FileName,
		ContentType: payload.ContentType,
		SizeBytes:   payload.SizeBytes,
		CommentID:   commentID,
		Origin:      &origin,
	})
	if err != nil {
		return made, err
	}

	made.id = stored.ID
	made.reference = stored.ObjectKey
	made.stamped = stored.UpdatedAt

	return made, nil
}

func (s *importsService) applyEmbed(
	ctx context.Context,
	run entity.ImportRun,
	decision entity.Decision,
	found *references,
	record entity.ImportRecord,
	log *recorder,
) (outcome, error) {
	payload, err := decodePayload[service.ImportEmbedPayload](record)
	if err != nil {
		return outcome{}, err
	}

	made := outcome{subject: strings.Join(payload.Attachments, ", ")}

	placed := make(map[string]string, len(payload.Attachments))
	lost := 0

	for _, key := range payload.Attachments {
		fileID, stored := found.files[key]
		if !stored {
			lost++

			continue
		}

		placed[entity.ImportEmbedMarker(key)] = entity.AttachmentContentPath(run.WorkspaceID, fileID)
	}

	if len(placed) == 0 {
		made.reason = reasonEmbedUnstored

		return made, nil
	}

	if strings.TrimSpace(payload.Comment) != "" {
		return s.embedInComment(ctx, run, found, payload, made, lost, placed, record, log)
	}

	return s.embedInIssue(ctx, run, decision, found, payload, made, lost, placed, record, log)
}

// embedInIssue restamps the issue's own ledger entry at the version this rewrite left it at.
// A revert reads that number to tell a person's edit from the import's, and an issue whose
// description the import rewrote after creating it is a version ahead of what the issue phase
// recorded: without the restamp the revert would refuse to remove rows nobody but the import
// had ever touched.
func (s *importsService) embedInIssue(
	ctx context.Context,
	run entity.ImportRun,
	decision entity.Decision,
	found *references,
	payload service.ImportEmbedPayload,
	made outcome,
	lost int,
	placed map[string]string,
	record entity.ImportRecord,
	log *recorder,
) (outcome, error) {
	issueID, carried := found.issues[payload.Issue]
	if !carried {
		made.reason = reasonIssueMissing

		return made, nil
	}

	issue, err := s.issues.GetVisible(ctx, run.WorkspaceID, issueID, decision.Scope)
	if err != nil {
		return made, err
	}

	body := rewritten(issue.Description, placed)

	if body == issue.Description {
		made.reason = reasonEmbedUnmarked

		return made, nil
	}

	updated, err := s.issueWriter.Update(ctx, run.WorkspaceID, issueID, service.UpdateIssueInput{
		ExpectedVersion: issue.Version,
		Description:     &body,
	})
	if err != nil {
		return made, err
	}

	if lost > 0 {
		log.dropped(record.Resource, record.ExternalID, made.subject, reasonEmbedUnstored)
	}

	log.restamp(entity.ImportLedgerEntry{
		Resource:        entity.ImportIssue,
		CreatedID:       updated.ID,
		VersionAtCreate: updated.Version,
	})

	made.id = updated.ID
	made.reference = updated.Reference()
	made.version = updated.Version
	made.stamped = updated.UpdatedAt

	return made, nil
}

func (s *importsService) embedInComment(
	ctx context.Context,
	run entity.ImportRun,
	found *references,
	payload service.ImportEmbedPayload,
	made outcome,
	lost int,
	placed map[string]string,
	record entity.ImportRecord,
	log *recorder,
) (outcome, error) {
	issueID, carried := found.issues[payload.Issue]
	commentID, said := found.comments[payload.Comment]

	if !carried || !said {
		made.reason = reasonCommentMissing

		return made, nil
	}

	comment, err := s.comments.GetByID(ctx, run.WorkspaceID, commentID)
	if err != nil {
		return made, err
	}

	body := rewritten(comment.Body, placed)

	if body == comment.Body {
		made.reason = reasonEmbedUnmarked

		return made, nil
	}

	edited, err := s.commentWriter.Edit(ctx, run.WorkspaceID, issueID, commentID, service.EditCommentInput{
		Body: body,
	})
	if err != nil {
		return made, err
	}

	if lost > 0 {
		log.dropped(record.Resource, record.ExternalID, made.subject, reasonEmbedUnstored)
	}

	made.id = edited.ID
	made.reference = edited.ID.String()
	made.stamped = edited.UpdatedAt

	return made, nil
}

func rewritten(body string, placed map[string]string) string {
	for marker, path := range placed {
		body = strings.ReplaceAll(body, marker, path)
	}

	return body
}

func (s *importsService) referenced(
	ctx context.Context,
	run entity.ImportRun,
	records []entity.ImportRecord,
) (*references, error) {
	wanted := map[entity.ImportResource]map[string]bool{}

	want := func(of entity.ImportResource, keys ...string) {
		for _, key := range keys {
			if strings.TrimSpace(key) == "" {
				continue
			}

			if wanted[of] == nil {
				wanted[of] = map[string]bool{}
			}

			wanted[of][key] = true
		}
	}

	for _, record := range records {
		if err := collect(record, want); err != nil {
			return nil, err
		}
	}

	resolved := map[entity.ImportResource]map[string]uuid.UUID{}

	for of, keys := range wanted {
		externalIDs := make([]string, 0, len(keys))
		for key := range keys {
			externalIDs = append(externalIDs, key)
		}

		found, err := s.ledger.Resolve(ctx, run.ID, of, externalIDs)
		if err != nil {
			return nil, err
		}

		resolved[of] = found
	}

	return &references{
		teams:    orEmpty(resolved[entity.ImportTeam]),
		states:   orEmpty(resolved[entity.ImportWorkflowState]),
		groups:   orEmpty(resolved[entity.ImportLabelGroup]),
		labels:   orEmpty(resolved[entity.ImportLabel]),
		projects: orEmpty(resolved[entity.ImportProject]),
		cycles:   orEmpty(resolved[entity.ImportCycle]),
		issues:   orEmpty(resolved[entity.ImportIssue]),
		comments: orEmpty(resolved[entity.ImportComment]),
		files:    orEmpty(resolved[entity.ImportAttachment]),
	}, nil
}

func orEmpty(found map[string]uuid.UUID) map[string]uuid.UUID {
	if found == nil {
		return map[string]uuid.UUID{}
	}

	return found
}

func (r *references) remember(resource entity.ImportResource, externalID string, id uuid.UUID) {
	switch resource {
	case entity.ImportTeam:
		r.teams[externalID] = id
	case entity.ImportWorkflowState:
		r.states[externalID] = id
	case entity.ImportLabelGroup:
		r.groups[externalID] = id
	case entity.ImportLabel:
		r.labels[externalID] = id
	case entity.ImportProject:
		r.projects[externalID] = id
	case entity.ImportCycle:
		r.cycles[externalID] = id
	case entity.ImportIssue:
		r.issues[externalID] = id
	case entity.ImportComment:
		r.comments[externalID] = id
	case entity.ImportAttachment:
		r.files[externalID] = id
	}
}

func collect(record entity.ImportRecord, want func(entity.ImportResource, ...string)) error {
	switch record.Resource {
	case entity.ImportWorkflowState:
		payload, err := decodePayload[service.ImportWorkflowStatePayload](record)
		if err != nil {
			return err
		}

		want(entity.ImportTeam, payload.Team)

	case entity.ImportLabel:
		payload, err := decodePayload[service.ImportLabelPayload](record)
		if err != nil {
			return err
		}

		want(entity.ImportTeam, payload.Team)
		want(entity.ImportLabelGroup, payload.Group)

	case entity.ImportCycle:
		payload, err := decodePayload[service.ImportCyclePayload](record)
		if err != nil {
			return err
		}

		want(entity.ImportTeam, payload.Team)

	case entity.ImportIssue:
		payload, err := decodePayload[service.ImportIssuePayload](record)
		if err != nil {
			return err
		}

		want(entity.ImportTeam, payload.Team)
		want(entity.ImportWorkflowState, payload.State)
		want(entity.ImportProject, payload.Project)
		want(entity.ImportCycle, payload.Cycle)
		want(entity.ImportLabel, payload.Labels...)

	case entity.ImportIssueParent:
		payload, err := decodePayload[service.ImportIssueParentPayload](record)
		if err != nil {
			return err
		}

		want(entity.ImportIssue, payload.Issue, payload.Parent)

	case entity.ImportIssueRelation:
		payload, err := decodePayload[service.ImportIssueRelationPayload](record)
		if err != nil {
			return err
		}

		want(entity.ImportIssue, payload.Issue, payload.Related)

	case entity.ImportComment:
		payload, err := decodePayload[service.ImportCommentPayload](record)
		if err != nil {
			return err
		}

		want(entity.ImportIssue, payload.Issue)
		want(entity.ImportComment, payload.Parent)

	case entity.ImportAttachment:
		payload, err := decodePayload[service.ImportAttachmentPayload](record)
		if err != nil {
			return err
		}

		want(entity.ImportIssue, payload.Issue)
		want(entity.ImportComment, payload.Comment)

	case entity.ImportEmbed:
		payload, err := decodePayload[service.ImportEmbedPayload](record)
		if err != nil {
			return err
		}

		want(entity.ImportIssue, payload.Issue)
		want(entity.ImportComment, payload.Comment)
		want(entity.ImportAttachment, payload.Attachments...)
	}

	return nil
}

func untaken(plan entity.MappingPlan, kind entity.ImportMappingKind, key string) string {
	mapping, found := plan.Lookup(kind, key)
	if !found {
		return ""
	}

	switch mapping.Decision {
	case entity.ImportDecisionSkip:
		return reasonLeftBehind
	case entity.ImportDecisionMap:
		return reasonAlreadyHere
	default:
		return ""
	}
}

func target(
	plan entity.MappingPlan,
	kind entity.ImportMappingKind,
	created map[string]uuid.UUID,
	key string,
) (uuid.UUID, bool) {
	if strings.TrimSpace(key) == "" {
		return uuid.Nil, false
	}

	if mapping, found := plan.Lookup(kind, key); found {
		switch mapping.Decision {
		case entity.ImportDecisionSkip:
			return uuid.Nil, false
		case entity.ImportDecisionMap:
			return mapping.TargetID, mapping.TargetID != uuid.Nil
		}
	}

	id, made := created[key]

	return id, made
}

func missing(
	lost []entity.ImportUnknownReferenceError,
	plan entity.MappingPlan,
	kind entity.ImportMappingKind,
	names, key string,
	resolved uuid.UUID,
) []entity.ImportUnknownReferenceError {
	if strings.TrimSpace(key) == "" || resolved != uuid.Nil || plan.Skips(kind, key) {
		return lost
	}

	return append(lost, entity.ImportUnknownReferenceError{Kind: names, Reference: key})
}

func priorityFor(plan entity.MappingPlan, key string) entity.IssuePriority {
	mapping, found := plan.Lookup(entity.ImportMapPriority, key)
	if !found || mapping.Decision != entity.ImportDecisionMap {
		return entity.IssuePriorityNone
	}

	priority := entity.IssuePriority(mapping.TargetValue)
	if !priority.Valid() {
		return entity.IssuePriorityNone
	}

	return priority
}

func accountFor(plan entity.MappingPlan, person service.ImportUser) uuid.UUID {
	if !person.Named() {
		return uuid.Nil
	}

	id, _ := plan.Target(entity.ImportMapUser, person.Key)

	return id
}

func at(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}

	return *value
}
