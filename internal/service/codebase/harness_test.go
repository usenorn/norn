package codebase_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	agentrepo "github.com/usenorn/norn/internal/repository/agent"
	codebaserepo "github.com/usenorn/norn/internal/repository/codebase"
	runnerrepo "github.com/usenorn/norn/internal/repository/runner"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	"github.com/usenorn/norn/internal/service"
	auditsvc "github.com/usenorn/norn/internal/service/audit"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	codebasesvc "github.com/usenorn/norn/internal/service/codebase"
)

type harness struct {
	codebases  *codebaserepo.MockCodebase
	runners    *runnerrepo.MockRunner
	agents     *agentrepo.MockAgent
	authorizer *authorizersvc.MockAuthorizer
	audit      *auditsvc.MockAudit
	service    service.Codebases

	workspaceID uuid.UUID
	agent       entity.Agent
	runner      entity.Runner
	role        entity.MembershipRole
	caller      uuid.UUID
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	workspaceID := uuid.New()

	agent := entity.Agent{
		ID:             uuid.New(),
		WorkspaceID:    workspaceID,
		AccountID:      uuid.New(),
		OwnerAccountID: uuid.New(),
		Name:           "opsy",
		Status:         entity.AgentStatusActive,
	}

	h := &harness{
		codebases:   codebaserepo.NewMockCodebase(ctrl),
		runners:     runnerrepo.NewMockRunner(ctrl),
		agents:      agentrepo.NewMockAgent(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		audit:       auditsvc.NewMockAudit(ctrl),
		workspaceID: workspaceID,
		agent:       agent,
		role:        entity.MembershipRoleMember,
		caller:      agent.OwnerAccountID,
		runner: entity.Runner{
			ID:          uuid.New(),
			WorkspaceID: workspaceID,
			AgentID:     agent.ID,
			Name:        "vlad-mbp",
			Status:      entity.RunnerStatusActive,
		},
	}

	transactor := transactorrepo.NewMockTransactor(ctrl)
	transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	h.audit.EXPECT().Record(gomock.Any(), gomock.Any()).AnyTimes()

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request entity.AccessRequest) (entity.Decision, error) {
			return entity.Decision{
				Actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: h.caller},
				Role:  h.role,
				Scope: entity.TeamScope{WorkspaceID: request.WorkspaceID, AllTeams: true},
			}, nil
		}).
		AnyTimes()

	h.service = codebasesvc.New(h.codebases, h.runners, h.agents, h.authorizer, h.audit, transactor)

	return h
}

func (h *harness) asRunner() context.Context {
	agentID := h.agent.ID
	runnerID := h.runner.ID

	h.runners.EXPECT().GetByID(gomock.Any(), runnerID).Return(h.runner, nil).AnyTimes()

	return identity.WithActor(context.Background(), entity.Actor{
		Kind:      entity.ActorKindAgent,
		AccountID: h.agent.AccountID,
		AgentID:   &agentID,
		RunnerID:  &runnerID,
	})
}

func (h *harness) asPerson() context.Context {
	return identity.WithActor(context.Background(), entity.Actor{
		Kind:      entity.ActorKindUser,
		AccountID: h.caller,
	})
}

func (h *harness) connecting(repositories ...entity.CodebaseRepository) service.ConnectCodebaseInput {
	return service.ConnectCodebaseInput{
		Name:         "norn",
		RootPath:     "/Users/vlad/projects/norn",
		Repositories: repositories,
		SharedFiles:  []string{"AGENTS.md"},
		Runtimes:     []entity.CodebaseRuntime{entity.CodebaseRuntimeProcess},
		Tools:        []entity.CodingTool{{Name: "claude", Version: "2.0.1"}},
	}
}

func (h *harness) live(state entity.CodebaseState, repositories ...entity.CodebaseRepository) entity.Codebase {
	return entity.Codebase{
		ID:           uuid.New(),
		RunnerID:     h.runner.ID,
		WorkspaceID:  h.workspaceID,
		AgentID:      h.agent.ID,
		Name:         "norn",
		RootPath:     "/Users/vlad/projects/norn",
		State:        state,
		Repositories: repositories,
		ConnectedAt:  time.Now().UTC(),
	}
}

func repositoryAt(name, path string) entity.CodebaseRepository {
	return entity.CodebaseRepository{
		Name:          name,
		RelPath:       path,
		DefaultBranch: "main",
		Remote:        entity.RemoteFingerprint{Hash: name + "-hash", Host: "github.com", PathTail: "usenorn/" + name},
	}
}
