package runner_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

func enrolling(d device) service.EnrolRunnerInput {
	return service.EnrolRunnerInput{
		Name:      "vlad-mbp",
		PublicKey: base64.StdEncoding.EncodeToString(d.public),
		Host: entity.RunnerHost{
			Hostname: "vlad-mbp.local",
			OS:       "darwin",
			Arch:     "arm64",
			Version:  "0.1.0",
		},
	}
}

func TestEnrollingBindsTheMachineToTheCallingAgent(t *testing.T) {
	h := newHarness(t)
	d := newDevice(t)

	h.expectAgent()

	var stored entity.Runner

	h.runners.EXPECT().
		Enrol(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, runner entity.Runner) (entity.Runner, error) {
			stored = runner
			runner.ID = uuid.New()

			return runner, nil
		})

	enrolled, err := h.service.Enrol(h.asAgent(), enrolling(d))
	if err != nil {
		t.Fatalf("enrol: %v", err)
	}

	if stored.AgentID != h.agent.ID {
		t.Fatalf("runner bound to agent %s, want %s", stored.AgentID, h.agent.ID)
	}

	if stored.WorkspaceID != h.workspaceID {
		t.Fatalf("runner landed in workspace %s, want the agent's %s", stored.WorkspaceID, h.workspaceID)
	}

	if !stored.PublicKey.Equal(d.public) {
		t.Fatalf("stored a different device key than the one presented")
	}

	if enrolled.RefreshToken == "" {
		t.Fatalf("enrolment returned no refresh secret; the machine would have nothing to present")
	}

	if !bytes.Equal(stored.RefreshHash, entity.HashRunnerSecret(enrolled.RefreshToken)) {
		t.Fatalf(
			"stored refresh hash does not match the secret handed back, so the machine could " +
				"never present it",
		)
	}
}

func TestEnrollingCarriesTheAgentsAuthoritySoTheMachineIsNoWiderThanTheAgent(t *testing.T) {
	h := newHarness(t)
	d := newDevice(t)

	h.expectAgent()

	var stored entity.Runner

	h.runners.EXPECT().
		Enrol(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, runner entity.Runner) (entity.Runner, error) {
			stored = runner

			return runner, nil
		})

	if _, err := h.service.Enrol(h.asAgent(), enrolling(d)); err != nil {
		t.Fatalf("enrol: %v", err)
	}

	if !stored.Authority.Scopes.SubsetOf(h.scopes) || len(stored.Authority.Scopes) != len(h.scopes) {
		t.Fatalf(
			"runner recorded scopes %v, want exactly the agent's %v; a runner that keeps no "+
				"authority resolves to an unconfined actor",
			stored.Authority.Scopes, h.scopes,
		)
	}

	if !stored.Authority.AllTeams {
		t.Fatalf("runner lost the agent's all-teams grant")
	}
}

func TestEnrollingFallsBackToTheHostnameWhenNoNameIsGiven(t *testing.T) {
	h := newHarness(t)
	d := newDevice(t)

	h.expectAgent()

	input := enrolling(d)
	input.Name = "   "

	var stored entity.Runner

	h.runners.EXPECT().
		Enrol(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, runner entity.Runner) (entity.Runner, error) {
			stored = runner

			return runner, nil
		})

	if _, err := h.service.Enrol(h.asAgent(), input); err != nil {
		t.Fatalf("enrol: %v", err)
	}

	if stored.Name != input.Host.Hostname {
		t.Fatalf("runner named %q, want the hostname %q", stored.Name, input.Host.Hostname)
	}
}

func TestOnlyAnAgentMayBindAMachineToItself(t *testing.T) {
	d := newDevice(t)

	cases := []struct {
		name  string
		actor entity.Actor
	}{
		{name: "signed-in person", actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: uuid.New()}},
		{name: "plain api token", actor: entity.Actor{Kind: entity.ActorKindToken, AccountID: uuid.New()}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			h := newHarness(t)

			ctx := identity.WithActor(context.Background(), testCase.actor)

			if _, err := h.service.Enrol(ctx, enrolling(d)); !errors.Is(err, entity.ErrRunnerEnrolmentNotAgent) {
				t.Fatalf("enrol as %s returned %v, want it refused", testCase.name, err)
			}
		})
	}
}

func TestADisabledAgentCannotBindANewMachine(t *testing.T) {
	h := newHarness(t)
	d := newDevice(t)

	disabled := h.agent
	disabled.Status = entity.AgentStatusDisabled

	h.agents.EXPECT().GetByAccountID(gomock.Any(), h.agent.AccountID).Return(disabled, nil)

	if _, err := h.service.Enrol(h.asAgent(), enrolling(d)); !errors.Is(err, entity.ErrAgentDisabled) {
		t.Fatalf("enrol for a disabled agent returned %v, want it refused", err)
	}
}

func TestAMalformedDeviceKeyIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	h := newHarness(t)

	h.expectAgent()

	input := enrolling(newDevice(t))
	input.PublicKey = base64.StdEncoding.EncodeToString([]byte("too short"))

	if _, err := h.service.Enrol(h.asAgent(), input); !errors.Is(err, entity.ErrRunnerKeyMalformed) {
		t.Fatalf("enrol with a short key returned %v, want it refused", err)
	}
}

func TestEnrolmentNamesTheAgentTheMachineWillActAs(t *testing.T) {
	h := newHarness(t)
	h.expectAgent()

	d := newDevice(t)

	h.runners.EXPECT().
		Enrol(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, runner entity.Runner) (entity.Runner, error) {
			runner.ID = uuid.New()

			return runner, nil
		})

	enrolled, err := h.service.Enrol(h.asAgent(), enrolling(d))
	if err != nil {
		t.Fatalf("enrol: %v", err)
	}

	if enrolled.Runner.AgentName != h.agent.Name {
		t.Fatalf(
			"enrolment named the agent %q, want %q so the machine can report it without another call",
			enrolled.Runner.AgentName, h.agent.Name,
		)
	}
}
