package imports

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

const (
	reasonTeamNotVisible = "the team this belongs to is not visible to the importing account"
	reasonTeamLeftBehind = "the team this belongs to is not being imported"
	reasonAlreadyHere    = "mapped onto something this workspace already has"
	reasonLeftBehind     = "this source concept was marked to be left behind"
	reasonIssueMissing   = "the issue this belongs to was not imported"
	reasonCycleDropped   = "cycles are generated on a team's own cadence and cannot be back-dated, " +
		"so this issue arrives without the one its source recorded"
)

type sourceChoice struct {
	decision entity.ImportDecision
	targetID uuid.UUID
}

func choiceFor(
	plan entity.MappingPlan,
	kind entity.ImportMappingKind,
	key string,
) (sourceChoice, bool) {
	if strings.TrimSpace(key) == "" {
		return sourceChoice{}, false
	}

	mapping, found := plan.Lookup(kind, key)
	if !found {
		return sourceChoice{decision: entity.ImportDecisionCreate}, true
	}

	return sourceChoice{decision: mapping.Decision, targetID: mapping.TargetID}, true
}

func personNamed(person service.ImportUser) string {
	if strings.TrimSpace(person.Name) != "" {
		return person.Name
	}

	return person.Key
}

type previewWalk struct {
	plan    entity.MappingPlan
	scope   entity.TeamScope
	log     *recorder
	targets map[uuid.UUID]bool
	carried map[string]string
}

func newPreviewWalk(plan entity.MappingPlan, scope entity.TeamScope) *previewWalk {
	return &previewWalk{
		plan:    plan,
		scope:   scope,
		log:     newRecorder(entity.ImportPhaseExecute),
		targets: make(map[uuid.UUID]bool),
		carried: make(map[string]string),
	}
}

func (w *previewWalk) teamReachable(key string) (uuid.UUID, string) {
	choice, referenced := choiceFor(w.plan, entity.ImportMapTeam, key)
	if !referenced {
		return uuid.Nil, ""
	}

	switch choice.decision {
	case entity.ImportDecisionSkip:
		return uuid.Nil, reasonTeamLeftBehind
	case entity.ImportDecisionMap:
		if !w.scope.Covers(choice.targetID) {
			return uuid.Nil, reasonTeamNotVisible
		}

		return choice.targetID, ""
	default:
		return uuid.Nil, ""
	}
}

func (w *previewWalk) consider(record entity.ImportRecord) error {
	switch record.Resource {
	case entity.ImportTeam:
		return w.team(record)
	case entity.ImportWorkflowState:
		return w.state(record)
	case entity.ImportLabelGroup:
		return w.labelGroup(record)
	case entity.ImportLabel:
		return w.label(record)
	case entity.ImportProject:
		return w.project(record)
	case entity.ImportIssue:
		return w.issue(record)
	case entity.ImportIssueParent:
		return w.parent(record)
	case entity.ImportIssueRelation:
		return w.relation(record)
	case entity.ImportComment:
		return w.comment(record)
	default:
		return nil
	}
}

func (w *previewWalk) created(record entity.ImportRecord, subject string) {
	w.log.note(record.Resource, record.ExternalID, subject, entity.ImportOutcomeCreated, nil)
}

func (w *previewWalk) team(record entity.ImportRecord) error {
	payload, err := decodePayload[service.ImportTeamPayload](record)
	if err != nil {
		return err
	}

	subject := named(payload.Name, payload.Key)
	choice, _ := choiceFor(w.plan, entity.ImportMapTeam, record.ExternalID)

	switch choice.decision {
	case entity.ImportDecisionSkip:
		w.log.skipped(record.Resource, record.ExternalID, subject, reasonLeftBehind)
	case entity.ImportDecisionMap:
		w.log.skipped(record.Resource, record.ExternalID, subject, reasonAlreadyHere)
	default:
		w.created(record, subject)
	}

	return nil
}

func (w *previewWalk) state(record entity.ImportRecord) error {
	payload, err := decodePayload[service.ImportWorkflowStatePayload](record)
	if err != nil {
		return err
	}

	if _, refused := w.teamReachable(payload.Team); refused != "" {
		w.log.skipped(record.Resource, record.ExternalID, payload.Name, refused)

		return nil
	}

	choice, _ := choiceFor(w.plan, entity.ImportMapState, record.ExternalID)

	switch choice.decision {
	case entity.ImportDecisionSkip:
		w.log.skipped(record.Resource, record.ExternalID, payload.Name, reasonLeftBehind)
	case entity.ImportDecisionMap:
		w.log.skipped(record.Resource, record.ExternalID, payload.Name, reasonAlreadyHere)
	default:
		w.created(record, payload.Name)

		if !entity.StateCategory(payload.Category).Valid() {
			w.log.adjusted(
				record.Resource, record.ExternalID, payload.Name,
				payload.Category, string(entity.StateCategoryNotStarted),
			)
		}
	}

	return nil
}

func (w *previewWalk) labelGroup(record entity.ImportRecord) error {
	payload, err := decodePayload[service.ImportLabelGroupPayload](record)
	if err != nil {
		return err
	}

	w.created(record, payload.Name)

	return nil
}

func (w *previewWalk) label(record entity.ImportRecord) error {
	payload, err := decodePayload[service.ImportLabelPayload](record)
	if err != nil {
		return err
	}

	if _, refused := w.teamReachable(payload.Team); refused != "" {
		w.log.skipped(record.Resource, record.ExternalID, payload.Name, refused)

		return nil
	}

	choice, _ := choiceFor(w.plan, entity.ImportMapLabel, record.ExternalID)

	switch choice.decision {
	case entity.ImportDecisionSkip:
		w.log.skipped(record.Resource, record.ExternalID, payload.Name, reasonLeftBehind)
	case entity.ImportDecisionMap:
		w.log.skipped(record.Resource, record.ExternalID, payload.Name, reasonAlreadyHere)
	default:
		w.created(record, payload.Name)

		nearest := string(entity.NearestLabelColor(payload.Color))

		if strings.TrimSpace(payload.Color) != "" && payload.Color != nearest {
			w.log.adjusted(record.Resource, record.ExternalID, payload.Name, payload.Color, nearest)
		}
	}

	return nil
}

func (w *previewWalk) project(record entity.ImportRecord) error {
	payload, err := decodePayload[service.ImportProjectPayload](record)
	if err != nil {
		return err
	}

	subject := named(payload.Name, payload.Slug)
	choice, _ := choiceFor(w.plan, entity.ImportMapProject, record.ExternalID)

	switch choice.decision {
	case entity.ImportDecisionSkip:
		w.log.skipped(record.Resource, record.ExternalID, subject, reasonLeftBehind)
	case entity.ImportDecisionMap:
		w.log.skipped(record.Resource, record.ExternalID, subject, reasonAlreadyHere)
	default:
		w.created(record, subject)
		w.attribution(record, subject, payload.Lead)
	}

	return nil
}

func (w *previewWalk) attribution(
	record entity.ImportRecord,
	subject string,
	person service.ImportUser,
) {
	if !person.Named() {
		return
	}

	if w.unattributed(person.Key) {
		w.log.unattributed(record.Resource, record.ExternalID, subject, personNamed(person))
	}
}

func (w *previewWalk) unattributed(key string) bool {
	mapping, found := w.plan.Lookup(entity.ImportMapUser, key)

	return found && mapping.Decision == entity.ImportDecisionUnattributed
}

func (w *previewWalk) issue(record entity.ImportRecord) error {
	payload, err := decodePayload[service.ImportIssuePayload](record)
	if err != nil {
		return err
	}

	targetID, refused := w.teamReachable(payload.Team)
	if refused != "" {
		w.log.skipped(record.Resource, record.ExternalID, payload.Title, refused)

		return nil
	}

	w.carried[record.ExternalID] = payload.Title
	w.created(record, payload.Title)

	if targetID != uuid.Nil {
		w.targets[targetID] = true
	}

	w.attribution(record, payload.Title, payload.Author)

	if strings.TrimSpace(payload.Cycle) != "" {
		w.log.dropped(record.Resource, record.ExternalID, payload.Title, reasonCycleDropped)
	}

	return nil
}

func (w *previewWalk) parent(record entity.ImportRecord) error {
	payload, err := decodePayload[service.ImportIssueParentPayload](record)
	if err != nil {
		return err
	}

	child, carried := w.carried[payload.Issue]
	parent, held := w.carried[payload.Parent]

	if !carried || !held {
		w.log.skipped(
			record.Resource, record.ExternalID, payload.Issue+" → "+payload.Parent, reasonIssueMissing,
		)

		return nil
	}

	w.created(record, child+" → "+parent)

	return nil
}

func (w *previewWalk) relation(record entity.ImportRecord) error {
	payload, err := decodePayload[service.ImportIssueRelationPayload](record)
	if err != nil {
		return err
	}

	issue, carried := w.carried[payload.Issue]
	related, held := w.carried[payload.Related]

	if !carried || !held {
		w.log.skipped(
			record.Resource, record.ExternalID,
			payload.Issue+" "+payload.Kind+" "+payload.Related, reasonIssueMissing,
		)

		return nil
	}

	w.created(record, issue+" "+payload.Kind+" "+related)

	return nil
}

func (w *previewWalk) comment(record entity.ImportRecord) error {
	payload, err := decodePayload[service.ImportCommentPayload](record)
	if err != nil {
		return err
	}

	if _, carried := w.carried[payload.Issue]; !carried {
		w.log.skipped(record.Resource, record.ExternalID, payload.Body, reasonIssueMissing)

		return nil
	}

	w.created(record, payload.Body)
	w.attribution(record, payload.Body, payload.Author)

	return nil
}

func (w *previewWalk) result() entity.ImportPreview {
	view := entity.ImportPreview{
		Created:      make([]entity.ImportPreviewLine, 0),
		Changed:      make([]entity.ImportPreviewLine, 0),
		Skipped:      make([]entity.ImportPreviewLine, 0),
		Unattributed: make([]entity.ImportPreviewLine, 0),
		Warnings:     make([]entity.ImportPreviewLine, 0),
	}

	for _, line := range w.log.lines {
		entry := entity.ImportPreviewLine{
			Resource:   line.Resource,
			Subject:    line.Subject,
			ExternalID: line.ExternalID,
			Outcome:    line.Outcome,
			Detail:     detailOf(line),
		}

		switch line.Outcome {
		case entity.ImportOutcomeCreated:
			view.Created = append(view.Created, entry)
		case entity.ImportOutcomeAdjusted:
			view.Changed = append(view.Changed, entry)
		case entity.ImportOutcomeUnattributed:
			view.Unattributed = append(view.Unattributed, entry)
		case entity.ImportOutcomeDropped:
			view.Warnings = append(view.Warnings, entry)
		default:
			view.Skipped = append(view.Skipped, entry)
		}
	}

	return view
}

func (s *importsService) Preview(
	ctx context.Context,
	workspaceID, runID uuid.UUID,
) (entity.ImportPreview, error) {
	decision, err := s.decide(ctx, workspaceID)
	if err != nil {
		return entity.ImportPreview{}, err
	}

	run, err := s.runs.GetByID(ctx, workspaceID, runID)
	if err != nil {
		return entity.ImportPreview{}, err
	}

	saved, err := s.mappings.List(ctx, run.ID)
	if err != nil {
		return entity.ImportPreview{}, err
	}

	walk := newPreviewWalk(entity.MappingPlan{Mappings: saved}, decision.Scope)

	for _, resource := range entity.ImportPhases() {
		if err := s.eachRecord(ctx, run.ID, resource, walk.consider); err != nil {
			return entity.ImportPreview{}, err
		}
	}

	view := walk.result()

	view.TriageTeams, err = s.triageTeams(ctx, run, walk.targets)
	if err != nil {
		return entity.ImportPreview{}, err
	}

	return view, nil
}

func (s *importsService) triageTeams(
	ctx context.Context,
	run entity.ImportRun,
	targets map[uuid.UUID]bool,
) ([]string, error) {
	routed := make([]string, 0)

	if len(targets) == 0 {
		return routed, nil
	}

	joined, err := s.teams.ListByWorkspaceMember(ctx, run.WorkspaceID, run.RequestedByAccount)
	if err != nil {
		return nil, err
	}

	member := make(map[uuid.UUID]bool, len(joined))
	for _, team := range joined {
		member[team.ID] = true
	}

	for teamID := range targets {
		settings, err := s.triage.Settings(ctx, run.WorkspaceID, teamID)
		if err != nil {
			if errors.Is(err, entity.ErrTriageDisabled) {
				continue
			}

			return nil, err
		}

		if !settings.Routes(run.Requester().Kind, member[teamID]) {
			continue
		}

		team, err := s.teams.GetByID(ctx, teamID)
		if err != nil {
			return nil, err
		}

		routed = append(routed, team.Name)
	}

	sort.Strings(routed)

	return routed, nil
}
