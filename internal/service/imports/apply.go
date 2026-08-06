package imports

import (
	"context"
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
	issues   map[string]uuid.UUID
	comments map[string]uuid.UUID
}

type outcome struct {
	subject   string
	id        uuid.UUID
	reference string
	version   int
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

func (s *importsService) applyRecord(
	ctx context.Context,
	run entity.ImportRun,
	decision entity.Decision,
	plan entity.MappingPlan,
	found *references,
	record entity.ImportRecord,
	log *recorder,
) {
	made, err := s.create(ctx, run, decision, plan, found, record, log)

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
			WorkspaceID:     run.WorkspaceID,
			Resource:        record.Resource,
			CreatedID:       made.id,
			ExternalID:      record.ExternalID,
			Reference:       made.reference,
			VersionAtCreate: made.version,
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
	case entity.ImportIssue:
		return s.createIssue(ctx, run, decision, plan, found, record, log)
	case entity.ImportIssueParent:
		return s.linkParent(ctx, run, decision, found, record)
	case entity.ImportIssueRelation:
		return s.linkRelation(ctx, run, found, record)
	case entity.ImportComment:
		return s.postComment(ctx, run, plan, found, record, log)
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

	return made, nil
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

	teamID, reachable := target(plan, entity.ImportMapTeam, found.teams, payload.Team)
	if !reachable {
		made.reason = reasonTeamLeftBehind

		return made, nil
	}

	if !decision.Scope.Covers(teamID) {
		made.reason = reasonTeamNotVisible

		return made, nil
	}

	author := accountFor(plan, payload.Author)

	if payload.Author.Named() && author == uuid.Nil {
		log.unattributed(record.Resource, record.ExternalID, payload.Title, personNamed(payload.Author))
	}

	if strings.TrimSpace(payload.Cycle) != "" {
		log.dropped(record.Resource, record.ExternalID, payload.Title, reasonCycleDropped)
	}

	stateID, _ := target(plan, entity.ImportMapState, found.states, payload.State)
	projectID, _ := target(plan, entity.ImportMapProject, found.projects, payload.Project)

	labelIDs := make([]uuid.UUID, 0, len(payload.Labels))

	for _, key := range payload.Labels {
		if labelID, carried := target(plan, entity.ImportMapLabel, found.labels, key); carried {
			labelIDs = append(labelIDs, labelID)
		}
	}

	origin := entity.NewImportOrigin(at(record.SourceCreatedAt), at(record.SourceUpdatedAt), author)

	issue, err := s.issueWriter.Create(ctx, service.CreateIssueInput{
		WorkspaceID:       run.WorkspaceID,
		TeamID:            teamID,
		Title:             payload.Title,
		Description:       payload.Description,
		Priority:          priorityFor(plan, payload.Priority),
		AssigneeAccountID: accountFor(plan, payload.Assignee),
		Estimate:          payload.Estimate,
		DueOn:             payload.DueOn,
		StateID:           stateID,
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

	return made, nil
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
		issues:   orEmpty(resolved[entity.ImportIssue]),
		comments: orEmpty(resolved[entity.ImportComment]),
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
	case entity.ImportIssue:
		r.issues[externalID] = id
	case entity.ImportComment:
		r.comments[externalID] = id
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

	case entity.ImportIssue:
		payload, err := decodePayload[service.ImportIssuePayload](record)
		if err != nil {
			return err
		}

		want(entity.ImportTeam, payload.Team)
		want(entity.ImportWorkflowState, payload.State)
		want(entity.ImportProject, payload.Project)
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
