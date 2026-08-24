package runner

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type runnersService struct {
	runners    repository.Runner
	channels   repository.RunnerChannel
	sessions   repository.RunnerSession
	agents     repository.Agent
	authorizer service.Authorizer
	audit      service.Audit
	cfg        config.Runner
	gateway    config.Gateway
	previews   config.Previews
}

func New(
	runners repository.Runner,
	channels repository.RunnerChannel,
	sessions repository.RunnerSession,
	agents repository.Agent,
	authorizer service.Authorizer,
	audit service.Audit,
	cfg config.Runner,
	gateway config.Gateway,
	previews config.Previews,
) service.Runners {
	return &runnersService{
		runners:    runners,
		channels:   channels,
		sessions:   sessions,
		agents:     agents,
		authorizer: authorizer,
		audit:      audit,
		cfg:        cfg,
		gateway:    gateway,
		previews:   previews,
	}
}

func (s *runnersService) decide(
	ctx context.Context,
	workspaceID uuid.UUID,
) (entity.Decision, error) {
	return s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceWorkspace,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
	})
}

func (s *runnersService) owned(
	ctx context.Context,
	workspaceID uuid.UUID,
	decision entity.Decision,
) (map[uuid.UUID]struct{}, error) {
	registered, err := s.agents.ListByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	mine := make(map[uuid.UUID]struct{}, len(registered))

	for _, agent := range registered {
		if agent.OwnerAccountID == decision.Actor.AccountID {
			mine[agent.ID] = struct{}{}
		}
	}

	return mine, nil
}

func (s *runnersService) reaches(
	ctx context.Context,
	workspaceID uuid.UUID,
	decision entity.Decision,
	machine entity.Runner,
) error {
	if decision.Role == entity.MembershipRoleAdmin {
		return nil
	}

	agent, err := s.agents.GetByID(ctx, workspaceID, machine.AgentID)
	if err != nil {
		return err
	}

	if agent.OwnerAccountID != decision.Actor.AccountID {
		return entity.ErrRunnerNotFound
	}

	return nil
}

func (s *runnersService) List(
	ctx context.Context,
	workspaceID uuid.UUID,
) ([]service.RunnerState, error) {
	decision, err := s.decide(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	enrolled, err := s.runners.ListByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	var mine map[uuid.UUID]struct{}

	if decision.Role != entity.MembershipRoleAdmin {
		mine, err = s.owned(ctx, workspaceID, decision)
		if err != nil {
			return nil, err
		}
	}

	states := make([]service.RunnerState, 0, len(enrolled))

	for _, machine := range enrolled {
		if mine != nil {
			if _, ok := mine[machine.AgentID]; !ok {
				continue
			}
		}

		presence, err := s.channels.Presence(ctx, machine.ID)
		if err != nil {
			return nil, err
		}

		states = append(states, service.RunnerState{
			Runner: machine,
			Load:   service.LoadOf(presence),
		})
	}

	return states, nil
}

func (s *runnersService) Pause(
	ctx context.Context,
	workspaceID, runnerID uuid.UUID,
) (entity.Runner, error) {
	paused := time.Now().UTC()

	return s.standby(ctx, workspaceID, runnerID, &paused)
}

func (s *runnersService) Resume(
	ctx context.Context,
	workspaceID, runnerID uuid.UUID,
) (entity.Runner, error) {
	return s.standby(ctx, workspaceID, runnerID, nil)
}

func (s *runnersService) standby(
	ctx context.Context,
	workspaceID, runnerID uuid.UUID,
	pausedAt *time.Time,
) (entity.Runner, error) {
	decision, err := s.decide(ctx, workspaceID)
	if err != nil {
		return entity.Runner{}, err
	}

	held, err := s.runners.GetByID(ctx, runnerID)
	if err != nil {
		return entity.Runner{}, err
	}

	if held.WorkspaceID != workspaceID {
		return entity.Runner{}, entity.ErrRunnerNotFound
	}

	if err := s.reaches(ctx, workspaceID, decision, held); err != nil {
		return entity.Runner{}, err
	}

	if held.Paused() == (pausedAt != nil) {
		return held, nil
	}

	settled, err := s.runners.SetPaused(ctx, workspaceID, runnerID, pausedAt)
	if err != nil {
		return entity.Runner{}, err
	}

	told := entity.ChannelRunnerResume
	action := entity.AuditRunnerResumed

	if pausedAt != nil {
		told = entity.ChannelRunnerPause
		action = entity.AuditRunnerPaused
	}

	message, err := entity.NewServerMessage(told, "", nil, time.Now().UTC())
	if err != nil {
		return entity.Runner{}, err
	}

	if _, err := s.channels.Append(ctx, runnerID, message); err != nil {
		return entity.Runner{}, err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  workspaceID,
		Action:       action,
		ResourceKind: string(entity.ResourceRunner),
		ResourceID:   runnerID,
		ResourceName: settled.Name,
	})

	return settled, nil
}

func (s *runnersService) Revoke(ctx context.Context, workspaceID, runnerID uuid.UUID) error {
	decision, err := s.decide(ctx, workspaceID)
	if err != nil {
		return err
	}

	held, err := s.runners.GetByID(ctx, runnerID)
	if err != nil {
		return err
	}

	if held.WorkspaceID != workspaceID {
		return entity.ErrRunnerNotFound
	}

	if err := s.reaches(ctx, workspaceID, decision, held); err != nil {
		return err
	}

	if err := s.runners.Revoke(ctx, workspaceID, runnerID, time.Now().UTC()); err != nil {
		return err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  workspaceID,
		Action:       entity.AuditRunnerRevoked,
		ResourceKind: string(entity.ResourceRunner),
		ResourceID:   runnerID,
		ResourceName: held.Name,
	})

	return nil
}

func (s *runnersService) Self(ctx context.Context) (entity.Runner, error) {
	actor, ok := identity.Actor(ctx)
	if !ok || actor.RunnerID == nil {
		return entity.Runner{}, entity.ErrRunnerCredentialInvalid
	}

	return s.runners.GetByID(ctx, *actor.RunnerID)
}
