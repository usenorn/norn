package runner

import (
	"context"
	"strings"
	"time"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

func (s *runnersService) Enrol(
	ctx context.Context,
	input service.EnrolRunnerInput,
) (service.EnrolledRunner, error) {
	actor, agent, err := s.enrolling(ctx)
	if err != nil {
		return service.EnrolledRunner{}, err
	}

	key, err := entity.ParseRunnerPublicKey(input.PublicKey)
	if err != nil {
		return service.EnrolledRunner{}, err
	}

	host := entity.RunnerHost{
		Hostname: strings.TrimSpace(input.Host.Hostname),
		OS:       strings.TrimSpace(input.Host.OS),
		Arch:     strings.TrimSpace(input.Host.Arch),
		Version:  strings.TrimSpace(input.Host.Version),
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = host.Hostname
	}

	fields := append(
		[]entity.FieldError{entity.ValidateRunnerName("name", name)},
		entity.ValidateRunnerHost(host)...,
	)

	if err := entity.NewValidationError(fields...); err != nil {
		return service.EnrolledRunner{}, err
	}

	refresh, refreshHash, err := entity.NewRunnerSecret(entity.RunnerRefreshPrefix)
	if err != nil {
		return service.EnrolledRunner{}, err
	}

	now := time.Now().UTC()

	enrolled, err := s.runners.Enrol(ctx, entity.Runner{
		WorkspaceID: agent.WorkspaceID,
		AgentID:     agent.ID,
		Name:        name,
		Host:        host,
		Authority:   entity.AuthorityOf(actor, agent.WorkspaceID),
		PublicKey:   key,
		RefreshHash: refreshHash,
		Status:      entity.RunnerStatusActive,
		EnrolledAt:  now,
		UpdatedAt:   now,
	})
	if err != nil {
		return service.EnrolledRunner{}, err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  enrolled.WorkspaceID,
		Action:       entity.AuditRunnerEnrolled,
		ResourceKind: string(entity.ResourceRunner),
		ResourceID:   enrolled.ID,
		ResourceName: enrolled.Name,
	})

	return service.EnrolledRunner{Runner: enrolled, RefreshToken: refresh}, nil
}

func (s *runnersService) enrolling(ctx context.Context) (entity.Actor, entity.Agent, error) {
	actor, ok := identity.Actor(ctx)
	if !ok || actor.Kind != entity.ActorKindAgent || actor.AgentID == nil {
		return entity.Actor{}, entity.Agent{}, entity.ErrRunnerEnrolmentNotAgent
	}

	agent, err := s.agents.GetByAccountID(ctx, actor.AccountID)
	if err != nil {
		return entity.Actor{}, entity.Agent{}, err
	}

	if agent.Disabled() {
		return entity.Actor{}, entity.Agent{}, entity.ErrAgentDisabled
	}

	return actor, agent, nil
}
