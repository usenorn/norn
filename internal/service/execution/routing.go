package execution

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

const queuedBatch = 50

type placement struct {
	runner   entity.Runner
	codebase uuid.UUID
	free     int
	waiting  entity.ExecutionQueuedReason
}

func (p placement) found() bool {
	return p.runner.ID != uuid.Nil
}

func (s *executionsService) route(
	ctx context.Context,
	agentID, teamID uuid.UUID,
	skip []uuid.UUID,
) (placement, error) {
	machines, err := s.runners.ListByAgentID(ctx, agentID)
	if err != nil {
		return placement{}, err
	}

	reaching := make([]entity.Runner, 0, len(machines))

	for _, machine := range machines {
		if machine.Reaches(teamID) && !slices.Contains(skip, machine.ID) {
			reaching = append(reaching, machine)
		}
	}

	if len(reaching) == 0 {
		return placement{waiting: entity.QueuedNoRunner}, nil
	}

	waiting := entity.QueuedRunnersOffline
	best := placement{}

	for _, machine := range reaching {
		presence, err := s.channels.Presence(ctx, machine.ID)
		if err != nil {
			return placement{}, err
		}

		if !presence.Live() {
			continue
		}

		if machine.Paused() || presence.Load.Paused {
			waiting = worse(waiting, entity.QueuedRunnersPaused)

			continue
		}

		free, err := s.freeSlots(ctx, machine, presence)
		if err != nil {
			return placement{}, err
		}

		if presence.Load.DiskPressure || free <= 0 {
			waiting = worse(waiting, entity.QueuedRunnersBusy)

			continue
		}

		if best.found() && best.free >= free {
			continue
		}

		codebase, err := s.codebaseFor(ctx, machine)
		if err != nil {
			return placement{}, err
		}

		best = placement{runner: machine, codebase: codebase, free: free}
	}

	if best.found() {
		return best, nil
	}

	return placement{waiting: waiting}, nil
}

func worse(held, found entity.ExecutionQueuedReason) entity.ExecutionQueuedReason {
	order := entity.ExecutionQueuedReasons()

	if slices.Index(order, found) > slices.Index(order, held) {
		return found
	}

	return held
}

func (s *executionsService) freeSlots(
	ctx context.Context,
	machine entity.Runner,
	presence entity.RunnerPresence,
) (int, error) {
	held, err := s.executions.CountHeldSlots(ctx, machine.ID)
	if err != nil {
		return 0, err
	}

	used := max(presence.Load.Used, held)

	return presence.Load.Capacity - used, nil
}

func (s *executionsService) codebaseFor(
	ctx context.Context,
	machine entity.Runner,
) (uuid.UUID, error) {
	held, err := s.codebases.ListByRunnerID(ctx, machine.ID)
	if err != nil {
		return uuid.Nil, err
	}

	live := make([]entity.Codebase, 0, len(held))

	for _, codebase := range held {
		if !codebase.Disconnected() {
			live = append(live, codebase)
		}
	}

	if len(live) != 1 {
		return uuid.Nil, nil
	}

	return live[0].ID, nil
}

const sharingBatch = 10

func (s *executionsService) Placement(
	ctx context.Context,
	issue entity.Issue,
	agentID uuid.UUID,
) (service.ExecutionPlacement, error) {
	machines, err := s.runners.ListByAgentID(ctx, agentID)
	if err != nil {
		return service.ExecutionPlacement{}, err
	}

	readiness := make([]service.RunnerReadiness, 0, len(machines))

	for _, machine := range machines {
		presence, err := s.channels.Presence(ctx, machine.ID)
		if err != nil {
			return service.ExecutionPlacement{}, err
		}

		free, err := s.freeSlots(ctx, machine, presence)
		if err != nil {
			return service.ExecutionPlacement{}, err
		}

		readiness = append(readiness, service.RunnerReadiness{
			Runner:       machine,
			Connected:    presence.Live(),
			Reaches:      machine.Reaches(issue.TeamID),
			Capacity:     presence.Load.Capacity,
			Used:         max(presence.Load.Used, presence.Load.Capacity-free),
			Free:         max(free, 0),
			DiskPressure: presence.Load.DiskPressure,
		})
	}

	placed, err := s.route(ctx, agentID, issue.TeamID, nil)
	if err != nil {
		return service.ExecutionPlacement{}, err
	}

	sharing, err := s.sharing(ctx, issue, placed, machines)
	if err != nil {
		return service.ExecutionPlacement{}, err
	}

	return service.ExecutionPlacement{
		Runners:  readiness,
		RunnerID: placed.runner.ID,
		Waiting:  placed.waiting,
		Sharing:  sharing,
	}, nil
}

func (s *executionsService) sharing(
	ctx context.Context,
	issue entity.Issue,
	placed placement,
	machines []entity.Runner,
) ([]entity.Execution, error) {
	codebase := placed.codebase

	for _, machine := range machines {
		if codebase != uuid.Nil {
			break
		}

		if !machine.Reaches(issue.TeamID) {
			continue
		}

		held, err := s.codebaseFor(ctx, machine)
		if err != nil {
			return nil, err
		}

		codebase = held
	}

	if codebase == uuid.Nil {
		return nil, nil
	}

	return s.executions.ListSharingRepositories(
		ctx, issue.WorkspaceID, "", codebase, sharingBatch,
	)
}
