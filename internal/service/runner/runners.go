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
	sessions   repository.RunnerSession
	agents     repository.Agent
	authorizer service.Authorizer
	audit      service.Audit
	cfg        config.Runner
}

func New(
	runners repository.Runner,
	sessions repository.RunnerSession,
	agents repository.Agent,
	authorizer service.Authorizer,
	audit service.Audit,
	cfg config.Runner,
) service.Runners {
	return &runnersService{
		runners:    runners,
		sessions:   sessions,
		agents:     agents,
		authorizer: authorizer,
		audit:      audit,
		cfg:        cfg,
	}
}

func (s *runnersService) decide(
	ctx context.Context,
	workspaceID uuid.UUID,
	action entity.Action,
) (entity.Decision, error) {
	return s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceRunner,
		Action:      action,
		WorkspaceID: workspaceID,
	})
}

func (s *runnersService) List(ctx context.Context, workspaceID uuid.UUID) ([]entity.Runner, error) {
	if _, err := s.decide(ctx, workspaceID, entity.ActionRead); err != nil {
		return nil, err
	}

	return s.runners.ListByWorkspaceID(ctx, workspaceID)
}

func (s *runnersService) Revoke(ctx context.Context, workspaceID, runnerID uuid.UUID) error {
	if _, err := s.decide(ctx, workspaceID, entity.ActionManage); err != nil {
		return err
	}

	held, err := s.runners.GetByID(ctx, runnerID)
	if err != nil {
		return err
	}

	if held.WorkspaceID != workspaceID {
		return entity.ErrRunnerNotFound
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
