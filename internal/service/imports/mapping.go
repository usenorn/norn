package imports

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

const (
	suggestedByStateName = "a workflow state on this workspace's teams already carries that name"
	suggestedByLabelName = "a label in this workspace already carries that name"
)

type conceptKey struct {
	kind entity.ImportMappingKind
	key  string
}

type concepts struct {
	order []conceptKey
	held  map[conceptKey]entity.ImportMapping
}

func newConcepts() *concepts {
	return &concepts{held: make(map[conceptKey]entity.ImportMapping)}
}

func (c *concepts) add(kind entity.ImportMappingKind, key, label, email string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}

	at := conceptKey{kind: kind, key: key}

	held, known := c.held[at]
	if !known {
		c.order = append(c.order, at)
		c.held[at] = entity.ImportMapping{
			Kind:        kind,
			SourceKey:   key,
			SourceLabel: strings.TrimSpace(label),
			SourceEmail: strings.TrimSpace(email),
		}

		return
	}

	if held.SourceLabel == "" {
		held.SourceLabel = strings.TrimSpace(label)
	}

	if held.SourceEmail == "" {
		held.SourceEmail = strings.TrimSpace(email)
	}

	c.held[at] = held
}

func (c *concepts) user(person service.ImportUser) {
	if !person.Named() {
		return
	}

	c.add(entity.ImportMapUser, person.Key, person.Name, person.Email)
}

func (c *concepts) derived() []entity.ImportMapping {
	derived := make([]entity.ImportMapping, 0, len(c.order))

	for _, at := range c.order {
		derived = append(derived, c.held[at])
	}

	return derived
}

func (s *importsService) Mappings(
	ctx context.Context,
	workspaceID, runID uuid.UUID,
) (entity.MappingPlan, error) {
	decision, err := s.decide(ctx, workspaceID)
	if err != nil {
		return entity.MappingPlan{}, err
	}

	run, err := s.runs.GetByID(ctx, workspaceID, runID)
	if err != nil {
		return entity.MappingPlan{}, err
	}

	found, err := s.conceptsIn(ctx, run)
	if err != nil {
		return entity.MappingPlan{}, err
	}

	derived := found.derived()

	if err := s.suggest(ctx, workspaceID, decision, derived); err != nil {
		return entity.MappingPlan{}, err
	}

	if err := s.mappings.Save(ctx, run.ID, derived); err != nil {
		return entity.MappingPlan{}, err
	}

	saved, err := s.mappings.List(ctx, run.ID)
	if err != nil {
		return entity.MappingPlan{}, err
	}

	return entity.MappingPlan{Mappings: merged(derived, saved)}, nil
}

func merged(derived, saved []entity.ImportMapping) []entity.ImportMapping {
	decided := make(map[conceptKey]entity.ImportMapping, len(saved))

	for _, mapping := range saved {
		if mapping.Undecided() {
			continue
		}

		decided[conceptKey{kind: mapping.Kind, key: mapping.SourceKey}] = mapping
	}

	plan := make([]entity.ImportMapping, 0, len(derived))

	for _, mapping := range derived {
		if chosen, found := decided[conceptKey{kind: mapping.Kind, key: mapping.SourceKey}]; found {
			mapping.Decision = chosen.Decision
			mapping.TargetID = chosen.TargetID
			mapping.TargetValue = chosen.TargetValue
			mapping.DecidedByAccount = chosen.DecidedByAccount
			mapping.DecidedAt = chosen.DecidedAt
		}

		plan = append(plan, mapping)
	}

	return plan
}

func (s *importsService) conceptsIn(
	ctx context.Context,
	run entity.ImportRun,
) (*concepts, error) {
	found := newConcepts()

	readers := map[entity.ImportResource]func(entity.ImportRecord) error{
		entity.ImportTeam: func(record entity.ImportRecord) error {
			payload, err := decodePayload[service.ImportTeamPayload](record)
			if err != nil {
				return err
			}

			found.add(entity.ImportMapTeam, record.ExternalID, named(payload.Name, payload.Key), "")

			return nil
		},
		entity.ImportWorkflowState: func(record entity.ImportRecord) error {
			payload, err := decodePayload[service.ImportWorkflowStatePayload](record)
			if err != nil {
				return err
			}

			found.add(entity.ImportMapState, record.ExternalID, payload.Name, "")
			found.add(entity.ImportMapTeam, payload.Team, "", "")

			return nil
		},
		entity.ImportLabel: func(record entity.ImportRecord) error {
			payload, err := decodePayload[service.ImportLabelPayload](record)
			if err != nil {
				return err
			}

			found.add(entity.ImportMapLabel, record.ExternalID, payload.Name, "")
			found.add(entity.ImportMapTeam, payload.Team, "", "")

			return nil
		},
		entity.ImportCycle: func(record entity.ImportRecord) error {
			payload, err := decodePayload[service.ImportCyclePayload](record)
			if err != nil {
				return err
			}

			found.add(entity.ImportMapTeam, payload.Team, "", "")

			return nil
		},
		entity.ImportProject: func(record entity.ImportRecord) error {
			payload, err := decodePayload[service.ImportProjectPayload](record)
			if err != nil {
				return err
			}

			found.add(entity.ImportMapProject, record.ExternalID, named(payload.Name, payload.Slug), "")
			found.user(payload.Lead)

			return nil
		},
		entity.ImportIssue: func(record entity.ImportRecord) error {
			payload, err := decodePayload[service.ImportIssuePayload](record)
			if err != nil {
				return err
			}

			found.add(entity.ImportMapTeam, payload.Team, "", "")
			found.add(entity.ImportMapState, payload.State, "", "")
			found.add(entity.ImportMapPriority, payload.Priority, payload.Priority, "")
			found.add(entity.ImportMapProject, payload.Project, "", "")

			for _, label := range payload.Labels {
				found.add(entity.ImportMapLabel, label, "", "")
			}

			found.user(payload.Author)
			found.user(payload.Assignee)

			return nil
		},
		entity.ImportComment: func(record entity.ImportRecord) error {
			payload, err := decodePayload[service.ImportCommentPayload](record)
			if err != nil {
				return err
			}

			found.user(payload.Author)

			return nil
		},
	}

	for _, resource := range entity.ImportPhases() {
		read, interesting := readers[resource]
		if !interesting {
			continue
		}

		if err := s.eachRecord(ctx, run.ID, resource, read); err != nil {
			return nil, err
		}
	}

	return found, nil
}

func named(name, fallback string) string {
	if strings.TrimSpace(name) != "" {
		return name
	}

	return fallback
}

func (s *importsService) suggest(
	ctx context.Context,
	workspaceID uuid.UUID,
	decision entity.Decision,
	derived []entity.ImportMapping,
) error {
	candidates, err := s.candidates(ctx, workspaceID)
	if err != nil {
		return err
	}

	states, err := s.workspaceStates(ctx, workspaceID, decision)
	if err != nil {
		return err
	}

	labels, err := s.labels.ListByWorkspaceID(ctx, workspaceID, decision.Scope)
	if err != nil {
		return err
	}

	byState := make(map[string]uuid.UUID, len(states))
	for _, state := range states {
		byState[strings.ToLower(strings.TrimSpace(state.Name))] = state.ID
	}

	byLabel := make(map[string]uuid.UUID, len(labels))
	for _, label := range labels {
		byLabel[strings.ToLower(strings.TrimSpace(label.Name))] = label.ID
	}

	for index, mapping := range derived {
		switch mapping.Kind {
		case entity.ImportMapUser:
			derived[index] = entity.SuggestAccount(mapping, candidates)
		case entity.ImportMapState:
			derived[index] = matched(mapping, byState, suggestedByStateName)
		case entity.ImportMapLabel:
			derived[index] = matched(mapping, byLabel, suggestedByLabelName)
		}
	}

	return nil
}

func matched(
	mapping entity.ImportMapping,
	existing map[string]uuid.UUID,
	reason string,
) entity.ImportMapping {
	name := strings.ToLower(strings.TrimSpace(mapping.SourceLabel))
	if name == "" {
		return mapping
	}

	id, found := existing[name]
	if !found {
		return mapping
	}

	mapping.SuggestedTargetID = id
	mapping.SuggestedReason = reason

	return mapping
}

func (s *importsService) candidates(
	ctx context.Context,
	workspaceID uuid.UUID,
) ([]entity.ImportCandidate, error) {
	page := entity.MembershipPage{Limit: s.cfg.PageSize}.Normalized()
	candidates := make([]entity.ImportCandidate, 0, page.Limit)

	for {
		members, err := s.members.ListPageByWorkspaceID(ctx, workspaceID, page)
		if err != nil {
			return nil, err
		}

		for _, member := range members {
			if member.Membership.Deactivated() {
				continue
			}

			candidates = append(candidates, entity.ImportCandidate{
				AccountID:   member.Membership.AccountID,
				Email:       member.Email,
				DisplayName: member.DisplayName,
			})
		}

		if len(members) < page.Limit {
			return candidates, nil
		}

		cursor := members[len(members)-1].Cursor()
		page.Cursor = &cursor
	}
}

func (s *importsService) workspaceStates(
	ctx context.Context,
	workspaceID uuid.UUID,
	decision entity.Decision,
) ([]entity.WorkflowState, error) {
	teams, err := s.workspaceTeams(ctx, workspaceID, decision)
	if err != nil {
		return nil, err
	}

	states := make([]entity.WorkflowState, 0, len(teams))

	for _, team := range teams {
		onTeam, err := s.states.ListByTeamID(ctx, team.ID)
		if err != nil {
			return nil, err
		}

		states = append(states, onTeam...)
	}

	return states, nil
}

func (s *importsService) Map(
	ctx context.Context,
	workspaceID, runID uuid.UUID,
	input service.MapImportInput,
) (entity.MappingPlan, error) {
	decision, err := s.decide(ctx, workspaceID)
	if err != nil {
		return entity.MappingPlan{}, err
	}

	run, err := s.runs.GetByID(ctx, workspaceID, runID)
	if err != nil {
		return entity.MappingPlan{}, err
	}

	for _, chosen := range input.Decisions {
		if err := chosen.Validate(); err != nil {
			return entity.MappingPlan{}, err
		}

		if err := s.reachable(ctx, workspaceID, decision, chosen); err != nil {
			return entity.MappingPlan{}, err
		}
	}

	if err := s.mappings.Decide(
		ctx, run.ID, input.Decisions, decision.Actor.AccountID, time.Now().UTC(),
	); err != nil {
		return entity.MappingPlan{}, err
	}

	plan, err := s.Mappings(ctx, workspaceID, runID)
	if err != nil {
		return entity.MappingPlan{}, err
	}

	if err := creatable(plan, decision); err != nil {
		return entity.MappingPlan{}, err
	}

	if err := s.restage(ctx, run, plan); err != nil {
		return entity.MappingPlan{}, err
	}

	return plan, nil
}

func (s *importsService) reachable(
	ctx context.Context,
	workspaceID uuid.UUID,
	decision entity.Decision,
	chosen entity.ImportMapping,
) error {
	if chosen.Decision != entity.ImportDecisionMap {
		return nil
	}

	switch chosen.Kind {
	case entity.ImportMapUser:
		return s.memberOf(ctx, workspaceID, chosen.TargetID)

	case entity.ImportMapTeam:
		if !decision.Scope.Covers(chosen.TargetID) {
			return entity.ErrTeamNotFound
		}

	case entity.ImportMapProject:
		if _, err := s.projects.GetByID(ctx, workspaceID, chosen.TargetID); err != nil {
			return err
		}
	}

	return nil
}

func (s *importsService) memberOf(ctx context.Context, workspaceID, accountID uuid.UUID) error {
	if accountID == uuid.Nil {
		return entity.ErrImportMappingNotMember
	}

	membership, err := s.members.Get(ctx, workspaceID, accountID)
	if err != nil {
		if errors.Is(err, entity.ErrMembershipNotFound) {
			return entity.ErrImportMappingNotMember
		}

		return err
	}

	if membership.Deactivated() {
		return entity.ErrImportMappingNotMember
	}

	return nil
}

func creatable(plan entity.MappingPlan, decision entity.Decision) error {
	if decision.Role == entity.MembershipRoleAdmin {
		return nil
	}

	for _, mapping := range plan.Mappings {
		if mapping.Kind != entity.ImportMapTeam && mapping.Kind != entity.ImportMapProject {
			continue
		}

		if mapping.Undecided() || mapping.Decision == entity.ImportDecisionCreate {
			return entity.ErrImportTeamMappingRequired
		}
	}

	return nil
}

func (s *importsService) restage(
	ctx context.Context,
	run entity.ImportRun,
	plan entity.MappingPlan,
) error {
	now := time.Now().UTC()

	switch {
	case plan.Complete() && run.Status.CanTransitionTo(entity.ImportMapped):
		return s.runs.Settle(ctx, run.ID, entity.ImportMapped, "", now)
	case !plan.Complete() && run.Status == entity.ImportMapped:
		return s.runs.Settle(ctx, run.ID, entity.ImportStaged, "", now)
	default:
		return nil
	}
}
