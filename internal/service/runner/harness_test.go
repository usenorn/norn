package runner_test

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	agentrepo "github.com/usenorn/norn/internal/repository/agent"
	runnerrepo "github.com/usenorn/norn/internal/repository/runner"
	runnersessionrepo "github.com/usenorn/norn/internal/repository/runnersession"
	"github.com/usenorn/norn/internal/service"
	auditsvc "github.com/usenorn/norn/internal/service/audit"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	runnersvc "github.com/usenorn/norn/internal/service/runner"
)

const (
	testClockSkew     = 3 * time.Minute
	testTunnelHost    = "tunnel.norn.ink"
	testPreviewDomain = "norn.ink"
)

type harness struct {
	runners    *runnerrepo.MockRunner
	sessions   *runnersessionrepo.MockRunnerSession
	agents     *agentrepo.MockAgent
	authorizer *authorizersvc.MockAuthorizer
	audit      *auditsvc.MockAudit
	service    service.Runners

	workspaceID uuid.UUID
	agent       entity.Agent
	scopes      entity.APIScopeSet
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	workspaceID := uuid.New()

	h := &harness{
		runners:     runnerrepo.NewMockRunner(ctrl),
		sessions:    runnersessionrepo.NewMockRunnerSession(ctrl),
		agents:      agentrepo.NewMockAgent(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		audit:       auditsvc.NewMockAudit(ctrl),
		workspaceID: workspaceID,
		scopes: entity.APIScopeSet{
			entity.NewAPIScope(entity.ResourceIssue, entity.ActionRead),
			entity.NewAPIScope(entity.ResourceIssue, entity.ActionManage),
		},
		agent: entity.Agent{
			ID:             uuid.New(),
			WorkspaceID:    workspaceID,
			AccountID:      uuid.New(),
			OwnerAccountID: uuid.New(),
			Name:           "opsy",
			Status:         entity.AgentStatusActive,
		},
	}

	h.audit.EXPECT().Record(gomock.Any(), gomock.Any()).AnyTimes()

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request entity.AccessRequest) (entity.Decision, error) {
			return entity.Decision{
				Actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()},
				Scope: entity.TeamScope{WorkspaceID: request.WorkspaceID, AllTeams: true},
			}, nil
		}).
		AnyTimes()

	h.service = runnersvc.New(
		h.runners,
		h.sessions,
		h.agents,
		h.authorizer,
		h.audit,
		config.Runner{
			AccessTTL:    15 * time.Minute,
			TicketTTL:    time.Minute,
			NonceTTL:     10 * time.Minute,
			MaxClockSkew: testClockSkew,
		},
		config.Gateway{TunnelHost: testTunnelHost},
		config.Previews{BaseDomain: testPreviewDomain, Scheme: "https"},
	)

	return h
}

func (h *harness) asAgent() context.Context {
	agentID := h.agent.ID

	return identity.WithActor(context.Background(), entity.Actor{
		Kind:      entity.ActorKindAgent,
		AccountID: h.agent.AccountID,
		AgentID:   &agentID,
		Scopes:    h.scopes,
		Grants: entity.APITokenGrants{{
			WorkspaceID: h.workspaceID,
			AllTeams:    true,
		}},
	})
}

func (h *harness) expectAgent() {
	h.agents.EXPECT().GetByAccountID(gomock.Any(), h.agent.AccountID).Return(h.agent, nil).AnyTimes()
	h.agents.EXPECT().GetByID(gomock.Any(), h.workspaceID, h.agent.ID).Return(h.agent, nil).AnyTimes()
}

type device struct {
	public  ed25519.PublicKey
	private ed25519.PrivateKey
}

func newDevice(t *testing.T) device {
	t.Helper()

	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate device key: %v", err)
	}

	return device{public: public, private: private}
}

func (d device) sign(runnerID uuid.UUID, nonce string, issuedAt time.Time) string {
	assertion := entity.RunnerAssertion{
		RunnerID: runnerID,
		Nonce:    nonce,
		IssuedAt: issuedAt,
		Audience: entity.RunnerAssertionAudience,
	}

	return base64.StdEncoding.EncodeToString(ed25519.Sign(d.private, assertion.SigningPayload()))
}

func (h *harness) enrolled(d device, refresh string) entity.Runner {
	return entity.Runner{
		ID:          uuid.New(),
		WorkspaceID: h.workspaceID,
		AgentID:     h.agent.ID,
		Name:        "vlad-mbp",
		Host: entity.RunnerHost{
			Hostname: "vlad-mbp",
			OS:       "darwin",
			Arch:     "arm64",
			Version:  "0.1.0",
		},
		Authority:   entity.NewRequestedAuthority(true, nil, h.scopes.Strings()),
		PublicKey:   d.public,
		RefreshHash: entity.HashRunnerSecret(refresh),
		Status:      entity.RunnerStatusActive,
		EnrolledAt:  time.Now().UTC(),
	}
}

func (h *harness) exchanging(runner entity.Runner, refresh string, d device) service.ExchangeRunnerTokenInput {
	issuedAt := time.Now().UTC()
	nonce := "nonce-that-is-long-enough"

	return service.ExchangeRunnerTokenInput{
		RefreshToken: refresh,
		RunnerID:     runner.ID,
		Nonce:        nonce,
		IssuedAt:     issuedAt,
		Audience:     entity.RunnerAssertionAudience,
		Signature:    d.sign(runner.ID, nonce, issuedAt),
	}
}
