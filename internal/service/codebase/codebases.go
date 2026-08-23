package codebase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type codebasesService struct {
	codebases  repository.Codebase
	runners    repository.Runner
	agents     repository.Agent
	authorizer service.Authorizer
	audit      service.Audit
	transactor repository.Transactor
}

func New(
	codebases repository.Codebase,
	runners repository.Runner,
	agents repository.Agent,
	authorizer service.Authorizer,
	audit service.Audit,
	transactor repository.Transactor,
) service.Codebases {
	return &codebasesService{
		codebases:  codebases,
		runners:    runners,
		agents:     agents,
		authorizer: authorizer,
		audit:      audit,
		transactor: transactor,
	}
}

func (s *codebasesService) calling(ctx context.Context) (entity.Runner, error) {
	actor, ok := identity.Actor(ctx)
	if !ok || actor.RunnerID == nil {
		return entity.Runner{}, entity.ErrCodebaseNotRunner
	}

	held, err := s.runners.GetByID(ctx, *actor.RunnerID)
	if err != nil {
		return entity.Runner{}, err
	}

	if held.Revoked() {
		return entity.Runner{}, entity.ErrRunnerRevoked
	}

	return held, nil
}

func (s *codebasesService) held(
	ctx context.Context,
	runner entity.Runner,
	codebaseID uuid.UUID,
) (entity.Codebase, error) {
	codebase, err := s.codebases.GetByID(ctx, codebaseID)
	if err != nil {
		return entity.Codebase{}, err
	}

	if codebase.RunnerID != runner.ID {
		return entity.Codebase{}, entity.ErrCodebaseNotFound
	}

	return codebase, nil
}

func inventoryOf(input service.ConnectCodebaseInput) repository.CodebaseInventory {
	repositories := make([]entity.CodebaseRepository, 0, len(input.Repositories))
	for _, held := range input.Repositories {
		repositories = append(repositories, entity.CodebaseRepository{
			Name:          strings.TrimSpace(held.Name),
			RelPath:       strings.TrimSpace(held.RelPath),
			DefaultBranch: strings.TrimSpace(held.DefaultBranch),
			Remote: entity.RemoteFingerprint{
				Hash:     strings.TrimSpace(held.Remote.Hash),
				Host:     strings.TrimSpace(held.Remote.Host),
				PathTail: strings.TrimSpace(held.Remote.PathTail),
			},
		})
	}

	shared := make([]string, 0, len(input.SharedFiles))
	for _, file := range input.SharedFiles {
		shared = append(shared, strings.TrimSpace(file))
	}

	runtimes := make([]entity.CodebaseRuntime, 0, len(input.Runtimes))
	runtimes = append(runtimes, input.Runtimes...)

	tools := make([]entity.CodingTool, 0, len(input.Tools))
	for _, tool := range input.Tools {
		tools = append(tools, entity.CodingTool{
			Name:    strings.TrimSpace(tool.Name),
			Version: strings.TrimSpace(tool.Version),
		})
	}

	return repository.CodebaseInventory{
		Name:           strings.TrimSpace(input.Name),
		RootPath:       strings.TrimSpace(input.RootPath),
		Repositories:   repositories,
		SharedFiles:    shared,
		Runtimes:       runtimes,
		Tools:          tools,
		PreviewGateway: input.PreviewGateway,
	}
}

func validate(inventory repository.CodebaseInventory) error {
	fields := []entity.FieldError{
		entity.ValidateCodebaseName("name", inventory.Name),
		entity.ValidateCodebaseRootPath("rootPath", inventory.RootPath),
	}

	fields = append(fields, entity.ValidateCodebaseRepositories("repositories", inventory.Repositories)...)
	fields = append(fields, entity.ValidateCodebaseSharedFiles("sharedFiles", inventory.SharedFiles)...)
	fields = append(fields, entity.ValidateCodebaseRuntimes("runtimes", inventory.Runtimes)...)
	fields = append(fields, entity.ValidateCodebaseTools("tools", inventory.Tools)...)

	return entity.NewValidationError(fields...)
}

func (s *codebasesService) Connect(
	ctx context.Context,
	input service.ConnectCodebaseInput,
) (entity.Codebase, error) {
	runner, err := s.calling(ctx)
	if err != nil {
		return entity.Codebase{}, err
	}

	inventory := inventoryOf(input)

	if inventory.Name == "" {
		inventory.Name = inventory.RootPath
	}

	if err := validate(inventory); err != nil {
		return entity.Codebase{}, err
	}

	var (
		codebase entity.Codebase
		fresh    bool
	)

	now := time.Now().UTC()

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		live, err := s.codebases.GetLiveByRoot(ctx, runner.ID, inventory.RootPath)
		if err != nil {
			if !errors.Is(err, entity.ErrCodebaseNotFound) {
				return err
			}

			fresh = true
			codebase, err = s.codebases.Connect(ctx, runner.ID, inventory, now)

			return err
		}

		state := entity.CodebaseStateActive
		if !entity.SameRepositorySet(live.Repositories, inventory.Repositories) {
			state = entity.CodebaseStateDrift
		}

		codebase, err = s.codebases.Replace(ctx, live.ID, inventory, state, now)

		return err
	})
	if err != nil {
		return entity.Codebase{}, err
	}

	if fresh {
		s.record(ctx, entity.AuditCodebaseConnected, runner, codebase)
	}

	return codebase, nil
}

func (s *codebasesService) Mine(ctx context.Context) ([]entity.Codebase, error) {
	runner, err := s.calling(ctx)
	if err != nil {
		return nil, err
	}

	return s.codebases.ListByRunnerID(ctx, runner.ID)
}

func (s *codebasesService) Confirm(
	ctx context.Context,
	codebaseID uuid.UUID,
) (entity.Codebase, error) {
	runner, err := s.calling(ctx)
	if err != nil {
		return entity.Codebase{}, err
	}

	codebase, err := s.held(ctx, runner, codebaseID)
	if err != nil {
		return entity.Codebase{}, err
	}

	if codebase.Disconnected() {
		return entity.Codebase{}, entity.ErrCodebaseDisconnected
	}

	return s.codebases.Confirm(ctx, codebase.ID, time.Now().UTC())
}

func (s *codebasesService) Disconnect(
	ctx context.Context,
	codebaseID uuid.UUID,
) (entity.Codebase, error) {
	runner, err := s.calling(ctx)
	if err != nil {
		return entity.Codebase{}, err
	}

	codebase, err := s.held(ctx, runner, codebaseID)
	if err != nil {
		return entity.Codebase{}, err
	}

	disconnected, err := s.codebases.Disconnect(ctx, codebase.ID, time.Now().UTC())
	if err != nil {
		return entity.Codebase{}, err
	}

	s.record(ctx, entity.AuditCodebaseDisconnected, runner, disconnected)

	return disconnected, nil
}

func (s *codebasesService) ListByAgent(
	ctx context.Context,
	workspaceID, agentID uuid.UUID,
) ([]entity.Codebase, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceWorkspace,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, err
	}

	agent, err := s.agents.GetByID(ctx, workspaceID, agentID)
	if err != nil {
		return nil, err
	}

	if decision.Role != entity.MembershipRoleAdmin &&
		agent.OwnerAccountID != decision.Actor.AccountID {
		return nil, entity.ErrAgentNotFound
	}

	return s.codebases.ListByAgentID(ctx, agent.ID)
}

func (s *codebasesService) record(
	ctx context.Context,
	action entity.AuditAction,
	runner entity.Runner,
	codebase entity.Codebase,
) {
	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  runner.WorkspaceID,
		Action:       action,
		ResourceKind: string(entity.ResourceRunner),
		ResourceID:   codebase.ID,
		ResourceName: codebase.Name,
	})
}
