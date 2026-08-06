package cycle

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type cyclesService struct {
	cycles       repository.Cycle
	cadences     repository.CycleCadence
	scopeChanges repository.CycleScopeChange
	issues       repository.Issue
	teams        repository.Team
	workspaces   repository.Workspace
	authorizer   service.Authorizer
	emitter      service.WebhookEmitter
	transactor   repository.Transactor
}

func New(
	cycles repository.Cycle,
	cadences repository.CycleCadence,
	scopeChanges repository.CycleScopeChange,
	issues repository.Issue,
	teams repository.Team,
	workspaces repository.Workspace,
	authorizer service.Authorizer,
	emitter service.WebhookEmitter,
	transactor repository.Transactor,
) service.Cycles {
	return &cyclesService{
		cycles:       cycles,
		cadences:     cadences,
		scopeChanges: scopeChanges,
		issues:       issues,
		teams:        teams,
		workspaces:   workspaces,
		authorizer:   authorizer,
		emitter:      emitter,
		transactor:   transactor,
	}
}

func (s *cyclesService) decide(
	ctx context.Context,
	workspaceID uuid.UUID,
	action entity.Action,
) (entity.Decision, error) {
	return s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceCycle,
		Action:      action,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
}

func (s *cyclesService) liveTeam(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
	decision entity.Decision,
) (entity.Team, error) {
	team, err := s.teams.GetByID(ctx, teamID)
	if err != nil {
		return entity.Team{}, err
	}

	if team.WorkspaceID != workspaceID || !decision.Scope.Covers(team.ID) {
		return entity.Team{}, entity.ErrTeamNotFound
	}

	if team.Archived() {
		return entity.Team{}, entity.ErrTeamArchived
	}

	return team, nil
}

func (s *cyclesService) today(ctx context.Context, workspaceID uuid.UUID) (string, error) {
	workspace, err := s.workspaces.GetByID(ctx, workspaceID)
	if err != nil {
		return "", err
	}

	return entity.Today(time.Now().UTC(), workspace.Timezone), nil
}

func viewsOf(cycles []entity.Cycle, today string) []service.CycleView {
	views := make([]service.CycleView, 0, len(cycles))

	for _, cycle := range cycles {
		views = append(views, service.CycleView{Cycle: cycle, Phase: cycle.PhaseOn(today)})
	}

	return views
}

func (s *cyclesService) Cadence(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
) (service.CadenceView, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return service.CadenceView{}, err
	}

	if _, err := s.liveTeam(ctx, workspaceID, teamID, decision); err != nil {
		if errors.Is(err, entity.ErrTeamArchived) {
			return service.CadenceView{}, entity.ErrCycleCadenceNotFound
		}

		return service.CadenceView{}, err
	}

	cadence, err := s.cadences.Get(ctx, teamID)
	if err != nil {
		return service.CadenceView{}, err
	}

	today, err := s.today(ctx, workspaceID)
	if err != nil {
		return service.CadenceView{}, err
	}

	return s.cadenceView(ctx, cadence, today)
}

func (s *cyclesService) cadenceView(
	ctx context.Context,
	cadence entity.CycleCadence,
	today string,
) (service.CadenceView, error) {
	cycles, err := s.cycles.ListByTeamID(ctx, cadence.TeamID)
	if err != nil {
		return service.CadenceView{}, err
	}

	upcoming := make([]entity.Cycle, 0, entity.CycleUpcomingCount)

	for _, cycle := range cycles {
		if cycle.PhaseOn(today) == entity.CyclePhaseUpcoming {
			upcoming = append(upcoming, cycle)
		}
	}

	return service.CadenceView{Cadence: cadence, Upcoming: viewsOf(upcoming, today)}, nil
}

func (s *cyclesService) SetCadence(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
	input service.SetCadenceInput,
) (service.CadenceView, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return service.CadenceView{}, err
	}

	if err := entity.ValidateCycleLength(input.LengthWeeks); err != nil {
		return service.CadenceView{}, err
	}

	if err := entity.ValidateCycleWeekday(int(input.StartsOn)); err != nil {
		return service.CadenceView{}, err
	}

	if _, err := s.liveTeam(ctx, workspaceID, teamID, decision); err != nil {
		return service.CadenceView{}, err
	}

	today, err := s.today(ctx, workspaceID)
	if err != nil {
		return service.CadenceView{}, err
	}

	var view service.CadenceView

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		anchor, err := s.reanchor(ctx, teamID, today, input.StartsOn)
		if err != nil {
			return err
		}

		cadence, err := s.cadences.Upsert(ctx, entity.CycleCadence{
			TeamID:      teamID,
			WorkspaceID: workspaceID,
			LengthWeeks: input.LengthWeeks,
			AnchorOn:    anchor,
		})
		if err != nil {
			return err
		}

		if _, err := s.fill(ctx, cadence, today); err != nil {
			return err
		}

		view, err = s.cadenceView(ctx, cadence, today)
		if err != nil {
			return err
		}

		return s.emitCadence(ctx, workspaceID, teamID, view.Upcoming, decision)
	})
	if err != nil {
		return service.CadenceView{}, err
	}

	return view, nil
}

func (s *cyclesService) reanchor(
	ctx context.Context,
	teamID uuid.UUID,
	today string,
	weekday time.Weekday,
) (string, error) {
	existing, err := s.cycles.ListByTeamID(ctx, teamID)
	if err != nil {
		return "", err
	}

	settled := today

	for _, cycle := range existing {
		if !cycle.Started(today) {
			continue
		}

		after, err := entity.CycleCadence{}.StartAfter(cycle.EndsOn)
		if err != nil {
			return "", err
		}

		if after > settled {
			settled = after
		}
	}

	if err := s.cycles.DeleteVacantFrom(ctx, teamID, settled); err != nil {
		return "", err
	}

	remaining, err := s.cycles.ListByTeamID(ctx, teamID)
	if err != nil {
		return "", err
	}

	for _, cycle := range remaining {
		after, err := entity.CycleCadence{}.StartAfter(cycle.EndsOn)
		if err != nil {
			return "", err
		}

		if after > settled {
			settled = after
		}
	}

	return entity.NextWeekdayOnOrAfter(settled, weekday)
}

func (s *cyclesService) DisableCadence(ctx context.Context, workspaceID, teamID uuid.UUID) error {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return err
	}

	if _, err := s.liveTeam(ctx, workspaceID, teamID, decision); err != nil {
		return err
	}

	today, err := s.today(ctx, workspaceID)
	if err != nil {
		return err
	}

	return s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.cadences.Delete(ctx, teamID); err != nil {
			return err
		}

		if err := s.cycles.DeleteVacantFrom(ctx, teamID, today); err != nil {
			return err
		}

		return s.emitCadence(ctx, workspaceID, teamID, nil, decision)
	})
}

func (s *cyclesService) Create(
	ctx context.Context,
	input service.CreateCycleInput,
) (service.CycleView, error) {
	// Cycles come from a cadence; creating one by hand is a product decision nobody has taken.
	// Refusing an origin that no request body can forge keeps this an import-only entry point,
	// so widening the repository to write a whole cycle does not quietly become a new capability.
	if !entity.OriginAttributed(input.Origin) {
		return service.CycleView{}, entity.ErrCycleCreateRequiresOrigin
	}

	decision, err := s.decide(ctx, input.WorkspaceID, entity.ActionManage)
	if err != nil {
		return service.CycleView{}, err
	}

	if err := entity.NewValidationError(
		entity.ValidateCycleDate("startsOn", input.StartsOn),
		entity.ValidateCycleDate("endsOn", input.EndsOn),
		entity.ValidateCycleWindow(input.StartsOn, input.EndsOn),
	); err != nil {
		return service.CycleView{}, err
	}

	if _, err := s.liveTeam(ctx, input.WorkspaceID, input.TeamID, decision); err != nil {
		return service.CycleView{}, err
	}

	number := input.Number

	if number == 0 {
		highest, err := s.cycles.HighestNumber(ctx, input.TeamID)
		if err != nil {
			return service.CycleView{}, err
		}

		number = highest + 1
	}

	today, err := s.today(ctx, input.WorkspaceID)
	if err != nil {
		return service.CycleView{}, err
	}

	created, err := s.cycles.Create(ctx, entity.Cycle{
		WorkspaceID: input.WorkspaceID,
		TeamID:      input.TeamID,
		Number:      number,
		StartsOn:    input.StartsOn,
		EndsOn:      input.EndsOn,
		ClosedAt:    input.ClosedAt,
		Origin:      input.Origin,
	})
	if err != nil {
		return service.CycleView{}, err
	}

	return service.CycleView{Cycle: created, Phase: created.PhaseOn(today)}, nil
}

func (s *cyclesService) List(
	ctx context.Context,
	workspaceID uuid.UUID,
	input service.ListCyclesInput,
) ([]service.CycleView, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return nil, err
	}

	if input.TeamID != nil {
		if _, err := s.teams.GetByID(ctx, *input.TeamID); err != nil {
			return nil, err
		}
	}

	cycles, err := s.cycles.ListVisible(ctx, decision.Scope, repository.CycleFilter{TeamID: input.TeamID})
	if err != nil {
		return nil, err
	}

	today, err := s.today(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	views := viewsOf(cycles, today)

	if input.Phase == "" {
		return views, nil
	}

	matching := make([]service.CycleView, 0, len(views))

	for _, view := range views {
		if view.Phase == input.Phase {
			matching = append(matching, view)
		}
	}

	return matching, nil
}

func (s *cyclesService) Get(
	ctx context.Context,
	workspaceID, cycleID uuid.UUID,
) (service.CycleView, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return service.CycleView{}, err
	}

	cycle, err := s.cycles.GetVisible(ctx, workspaceID, cycleID, decision.Scope)
	if err != nil {
		return service.CycleView{}, err
	}

	today, err := s.today(ctx, workspaceID)
	if err != nil {
		return service.CycleView{}, err
	}

	return service.CycleView{Cycle: cycle, Phase: cycle.PhaseOn(today)}, nil
}

func (s *cyclesService) Current(
	ctx context.Context,
	workspaceID uuid.UUID,
) ([]service.TeamCycle, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return nil, err
	}

	cadences, err := s.cadences.ListByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	cycles, err := s.cycles.ListVisible(ctx, decision.Scope, repository.CycleFilter{})
	if err != nil {
		return nil, err
	}

	today, err := s.today(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	current := make([]service.TeamCycle, 0, len(cadences))

	for _, cadence := range cadences {
		if !decision.Scope.Covers(cadence.TeamID) {
			continue
		}

		if view, found := showing(cycles, cadence.TeamID, today); found {
			current = append(current, service.TeamCycle{TeamID: cadence.TeamID, View: view})
		}
	}

	return current, nil
}

func showing(cycles []entity.Cycle, teamID uuid.UUID, today string) (service.CycleView, bool) {
	var upcoming *entity.Cycle

	for i, cycle := range cycles {
		if cycle.TeamID != teamID {
			continue
		}

		switch cycle.PhaseOn(today) {
		case entity.CyclePhaseCurrent, entity.CyclePhaseEnded:
			return service.CycleView{Cycle: cycle, Phase: cycle.PhaseOn(today)}, true
		case entity.CyclePhaseUpcoming:
			if upcoming == nil {
				upcoming = &cycles[i]
			}
		case entity.CyclePhaseClosed:
		}
	}

	if upcoming == nil {
		return service.CycleView{}, false
	}

	return service.CycleView{Cycle: *upcoming, Phase: entity.CyclePhaseUpcoming}, true
}

func (s *cyclesService) Scope(
	ctx context.Context,
	workspaceID, cycleID uuid.UUID,
) (service.CycleScope, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return service.CycleScope{}, err
	}

	cycle, err := s.cycles.GetVisible(ctx, workspaceID, cycleID, decision.Scope)
	if err != nil {
		return service.CycleScope{}, err
	}

	changes, err := s.scopeChanges.ListByCycleID(ctx, cycle.ID)
	if err != nil {
		return service.CycleScope{}, err
	}

	issues, err := s.membership(ctx, cycle, decision.Scope)
	if err != nil {
		return service.CycleScope{}, err
	}

	joined := map[uuid.UUID]bool{}

	for _, change := range changes {
		if change.Change == entity.CycleScopeChangeAdded {
			joined[change.IssueID] = true
		}
	}

	scope := service.CycleScope{
		Original: make([]entity.Issue, 0, len(issues)),
		Added:    make([]entity.Issue, 0),
		Changes:  changes,
	}

	for _, issue := range issues {
		if joined[issue.ID] {
			scope.Added = append(scope.Added, issue)

			continue
		}

		scope.Original = append(scope.Original, issue)
	}

	return scope, nil
}

func (s *cyclesService) membership(
	ctx context.Context,
	cycle entity.Cycle,
	scope entity.TeamScope,
) ([]entity.Issue, error) {
	cycleID := cycle.ID

	return s.issues.ListVisible(ctx, scope, entity.IssuePage{
		Limit:   entity.IssuePageMaxSize,
		CycleID: &cycleID,
	})
}

func (s *cyclesService) Close(
	ctx context.Context,
	workspaceID, cycleID uuid.UUID,
	input service.CloseCycleInput,
) (service.CycleView, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return service.CycleView{}, err
	}

	today, err := s.today(ctx, workspaceID)
	if err != nil {
		return service.CycleView{}, err
	}

	var closed entity.Cycle

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		cycle, err := s.cycles.LockByID(ctx, cycleID)
		if err != nil {
			return err
		}

		if cycle.WorkspaceID != workspaceID || !decision.Scope.Covers(cycle.TeamID) {
			return entity.ErrCycleNotFound
		}

		switch cycle.PhaseOn(today) {
		case entity.CyclePhaseClosed:
			return entity.ErrCycleClosed
		case entity.CyclePhaseUpcoming, entity.CyclePhaseCurrent:
			return entity.ErrCycleNotEnded
		case entity.CyclePhaseEnded:
		}

		issues, err := s.membership(ctx, cycle, decision.Scope)
		if err != nil {
			return err
		}

		open := entity.OpenIssues(issues)

		if len(open) > 0 && !input.Rollover.Valid() {
			return entity.ErrCycleRolloverRequired
		}

		if err := s.roll(ctx, cycle, open, input, decision); err != nil {
			return err
		}

		rollover := input.Rollover
		if len(open) == 0 {
			rollover = entity.CycleRolloverBacklog
		}

		var account *uuid.UUID

		if decision.Actor.AccountID != uuid.Nil {
			actor := decision.Actor.AccountID
			account = &actor
		}

		closed, err = s.cycles.Close(ctx, cycle.ID, time.Now().UTC(), account, rollover)
		if err != nil {
			return err
		}

		return s.emit(ctx, entity.WebhookCycleClosed, closed, decision)
	})
	if err != nil {
		return service.CycleView{}, err
	}

	return service.CycleView{Cycle: closed, Phase: entity.CyclePhaseClosed}, nil
}

func (s *cyclesService) roll(
	ctx context.Context,
	cycle entity.Cycle,
	open []entity.Issue,
	input service.CloseCycleInput,
	decision entity.Decision,
) error {
	if len(open) == 0 {
		return nil
	}

	destinations := map[uuid.UUID]entity.CycleRollover{}

	for _, issue := range open {
		destinations[issue.ID] = input.Rollover
	}

	for _, override := range input.Overrides {
		if _, member := destinations[override.IssueID]; !member {
			continue
		}

		if !override.Destination.Valid() {
			return entity.NewValidationError(entity.FieldError{
				Field: "overrides",
				Code:  entity.ValidationCodeUnsupportedValue,
			})
		}

		destinations[override.IssueID] = override.Destination
	}

	var forward, backlog []uuid.UUID

	for _, issue := range open {
		if destinations[issue.ID] == entity.CycleRolloverNext {
			forward = append(forward, issue.ID)

			continue
		}

		backlog = append(backlog, issue.ID)
	}

	now := time.Now().UTC()

	if len(forward) > 0 {
		next, err := s.cycles.NextAfter(ctx, cycle.TeamID, cycle.EndsOn)
		if err != nil {
			return err
		}

		if err := s.issues.MoveIssuesToCycle(ctx, forward, &next.ID, now); err != nil {
			return err
		}

		if err := s.mark(ctx, cycle, forward, entity.CycleScopeChangeRolledOver, decision, now); err != nil {
			return err
		}
	}

	if len(backlog) == 0 {
		return nil
	}

	if err := s.issues.MoveIssuesToCycle(ctx, backlog, nil, now); err != nil {
		return err
	}

	return s.mark(ctx, cycle, backlog, entity.CycleScopeChangeReturned, decision, now)
}

func (s *cyclesService) mark(
	ctx context.Context,
	cycle entity.Cycle,
	issueIDs []uuid.UUID,
	kind entity.CycleScopeChangeKind,
	decision entity.Decision,
	now time.Time,
) error {
	for _, issueID := range issueIDs {
		if err := s.scopeChanges.Record(ctx, entity.CycleScopeChange{
			CycleID:        cycle.ID,
			IssueID:        issueID,
			Change:         kind,
			ActorAccountID: decision.Actor.AccountID,
			ChangedAt:      now,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (s *cyclesService) Generate(ctx context.Context) error {
	listings, err := s.cadences.ListAll(ctx)
	if err != nil {
		return err
	}

	for _, listing := range listings {
		today := entity.Today(time.Now().UTC(), listing.Timezone)

		if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
			cadence, err := s.cadences.Lock(ctx, listing.Cadence.TeamID)
			if err != nil {
				return err
			}

			started, err := s.fill(ctx, cadence, today)
			if err != nil {
				return err
			}

			for _, cycle := range started {
				if err := s.emit(ctx, entity.WebhookCycleStarted, cycle, entity.Decision{}); err != nil {
					return err
				}
			}

			return nil
		}); err != nil {
			logging.From(ctx).ErrorContext(
				ctx,
				"could not generate cycles for a team",
				"team_id", listing.Cadence.TeamID.String(),
				"error", err.Error(),
			)
		}
	}

	return nil
}

func (s *cyclesService) fill(
	ctx context.Context,
	cadence entity.CycleCadence,
	today string,
) ([]entity.Cycle, error) {
	existing, err := s.cycles.ListByTeamID(ctx, cadence.TeamID)
	if err != nil {
		return nil, err
	}

	number, err := s.cycles.HighestNumber(ctx, cadence.TeamID)
	if err != nil {
		return nil, err
	}

	next := cadence.AnchorOn
	upcoming := 0

	for _, cycle := range existing {
		if !cycle.Started(today) {
			upcoming++
		}

		after, err := cadence.StartAfter(cycle.EndsOn)
		if err != nil {
			return nil, err
		}

		if after > next {
			next = after
		}
	}

	var created []entity.Cycle

	for upcoming < entity.CycleUpcomingCount {
		starts, ends, err := cadence.WindowFrom(next)
		if err != nil {
			return nil, err
		}

		next, err = cadence.StartAfter(ends)
		if err != nil {
			return nil, err
		}

		if ends < today {
			continue
		}

		number++

		cycle, err := s.cycles.Create(ctx, entity.Cycle{
			WorkspaceID: cadence.WorkspaceID,
			TeamID:      cadence.TeamID,
			Number:      number,
			StartsOn:    starts,
			EndsOn:      ends,
		})
		if err != nil {
			return nil, err
		}

		created = append(created, cycle)

		if starts > today {
			upcoming++
		}
	}

	return created, nil
}
