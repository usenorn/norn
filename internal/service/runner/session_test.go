package runner_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
)

func (h *harness) expectRunnerByRefresh(runner entity.Runner, refresh string) {
	h.runners.EXPECT().
		GetByRefreshHash(gomock.Any(), entity.HashRunnerSecret(refresh)).
		Return(runner, nil).
		AnyTimes()
}

func (h *harness) expectFreshNonce() {
	h.sessions.EXPECT().ClaimNonce(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
}

func TestAnExchangeReturnsAnAccessTokenAndATicketAndMarksTheMachineSeen(t *testing.T) {
	h := newHarness(t)
	h.expectAgent()
	d := newDevice(t)

	const refresh = "nrr_secret"

	runner := h.enrolled(d, refresh)

	h.expectRunnerByRefresh(runner, refresh)
	h.expectFreshNonce()

	var granted, ticketed uuid.UUID

	h.sessions.EXPECT().
		Grant(gomock.Any(), gomock.Any(), gomock.Any(), 15*time.Minute).
		DoAndReturn(func(_ context.Context, _ []byte, id uuid.UUID, _ time.Duration) error {
			granted = id

			return nil
		})

	h.sessions.EXPECT().
		IssueTicket(gomock.Any(), gomock.Any(), gomock.Any(), time.Minute).
		DoAndReturn(func(_ context.Context, _ []byte, id uuid.UUID, _ time.Duration) error {
			ticketed = id

			return nil
		})

	h.runners.EXPECT().RecordSeen(gomock.Any(), runner.ID, gomock.Any()).Return(nil)

	session, err := h.service.Exchange(context.Background(), h.exchanging(runner, refresh, d))
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}

	if granted != runner.ID || ticketed != runner.ID {
		t.Fatalf("granted access to %s and a ticket to %s, want both for %s", granted, ticketed, runner.ID)
	}

	if !entity.LooksLikeRunnerToken(session.AccessToken) {
		t.Fatalf("access token %q does not carry the runner prefix the middleware dispatches on", session.AccessToken)
	}

	if session.Ticket == session.AccessToken {
		t.Fatalf("the ticket and the access token are the same secret; spending one would spend the other")
	}
}

func TestAnAssertionIsAcceptedOnlyOnce(t *testing.T) {
	h := newHarness(t)
	h.expectAgent()
	d := newDevice(t)

	const refresh = "nrr_secret"

	runner := h.enrolled(d, refresh)

	h.expectRunnerByRefresh(runner, refresh)

	h.sessions.EXPECT().ClaimNonce(gomock.Any(), runner.ID, gomock.Any()).Return(false, nil)

	_, err := h.service.Exchange(context.Background(), h.exchanging(runner, refresh, d))
	if !errors.Is(err, entity.ErrRunnerAssertionReplayed) {
		t.Fatalf("replaying an assertion returned %v, want it refused", err)
	}
}

func TestAnUnreachableNonceStoreRefusesTheExchangeRatherThanAdmittingIt(t *testing.T) {
	h := newHarness(t)
	h.expectAgent()
	d := newDevice(t)

	const refresh = "nrr_secret"

	runner := h.enrolled(d, refresh)

	h.expectRunnerByRefresh(runner, refresh)

	h.sessions.EXPECT().
		ClaimNonce(gomock.Any(), runner.ID, gomock.Any()).
		Return(false, errors.New("valkey is unreachable"))

	if _, err := h.service.Exchange(context.Background(), h.exchanging(runner, refresh, d)); err == nil {
		t.Fatalf(
			"the exchange succeeded while replay protection was unavailable; it must fail closed",
		)
	}
}

func TestAnAssertionOutsideTheSkewWindowIsRefused(t *testing.T) {
	h := newHarness(t)
	h.expectAgent()
	d := newDevice(t)

	const refresh = "nrr_secret"

	runner := h.enrolled(d, refresh)

	h.expectRunnerByRefresh(runner, refresh)

	for _, drift := range []time.Duration{testClockSkew + time.Minute, -testClockSkew - time.Minute} {
		input := h.exchanging(runner, refresh, d)
		input.IssuedAt = time.Now().UTC().Add(drift)
		input.Signature = d.sign(runner.ID, input.Nonce, input.IssuedAt)

		_, err := h.service.Exchange(context.Background(), input)
		if !errors.Is(err, entity.ErrRunnerAssertionStale) {
			t.Fatalf("an assertion %s out returned %v, want it refused", drift, err)
		}
	}
}

func TestATamperedSignatureIsRefused(t *testing.T) {
	h := newHarness(t)
	h.expectAgent()
	d := newDevice(t)

	const refresh = "nrr_secret"

	runner := h.enrolled(d, refresh)

	h.expectRunnerByRefresh(runner, refresh)

	input := h.exchanging(runner, refresh, d)
	input.Signature = d.sign(runner.ID, "a-different-nonce-entirely", input.IssuedAt)

	if _, err := h.service.Exchange(context.Background(), input); !errors.Is(err, entity.ErrRunnerAssertionForged) {
		t.Fatalf("a signature over different content returned %v, want it refused", err)
	}
}

func TestAnAssertionSignedByAnotherMachinesKeyIsRefused(t *testing.T) {
	h := newHarness(t)
	h.expectAgent()
	d := newDevice(t)

	const refresh = "nrr_secret"

	runner := h.enrolled(d, refresh)

	h.expectRunnerByRefresh(runner, refresh)

	input := h.exchanging(runner, refresh, newDevice(t))

	if _, err := h.service.Exchange(context.Background(), input); !errors.Is(err, entity.ErrRunnerAssertionForged) {
		t.Fatalf("an assertion signed by a different device returned %v, want it refused", err)
	}
}

func TestAnAssertionNamingAnotherRunnerIsRefused(t *testing.T) {
	h := newHarness(t)
	h.expectAgent()
	d := newDevice(t)

	const refresh = "nrr_secret"

	runner := h.enrolled(d, refresh)

	h.expectRunnerByRefresh(runner, refresh)

	input := h.exchanging(runner, refresh, d)
	input.RunnerID = uuid.New()

	if _, err := h.service.Exchange(context.Background(), input); !errors.Is(err, entity.ErrRunnerAssertionMismatch) {
		t.Fatalf("an assertion naming a different runner returned %v, want it refused", err)
	}
}

func TestAnUnknownRefreshSecretDoesNotSayWhetherAnyRunnerHasIt(t *testing.T) {
	h := newHarness(t)
	d := newDevice(t)

	const refresh = "nrr_secret"

	runner := h.enrolled(d, refresh)

	h.runners.EXPECT().
		GetByRefreshHash(gomock.Any(), gomock.Any()).
		Return(entity.Runner{}, entity.ErrRunnerNotFound)

	_, err := h.service.Exchange(context.Background(), h.exchanging(runner, refresh, d))
	if !errors.Is(err, entity.ErrRunnerCredentialInvalid) {
		t.Fatalf(
			"an unknown secret returned %v, want the credential refused without confirming "+
				"whether a runner exists",
			err,
		)
	}
}

func TestARevokedRunnerCannotExchangeAgain(t *testing.T) {
	h := newHarness(t)
	d := newDevice(t)

	const refresh = "nrr_secret"

	runner := h.enrolled(d, refresh)
	runner.Status = entity.RunnerStatusRevoked

	h.expectRunnerByRefresh(runner, refresh)

	_, err := h.service.Exchange(context.Background(), h.exchanging(runner, refresh, d))
	if !errors.Is(err, entity.ErrRunnerRevoked) {
		t.Fatalf("a revoked runner exchanged and got %v, want it refused", err)
	}
}

func TestAnAuthenticatedRunnerActsAsItsAgentAndNamesItsMachine(t *testing.T) {
	h := newHarness(t)
	d := newDevice(t)

	runner := h.enrolled(d, "nrr_secret")

	h.expectAgent()

	h.sessions.EXPECT().
		Resolve(gomock.Any(), entity.HashRunnerSecret("nrs_access")).
		Return(runner.ID, nil)

	h.runners.EXPECT().GetByID(gomock.Any(), runner.ID).Return(runner, nil)

	actor, err := h.service.Authenticate(context.Background(), "nrs_access")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}

	if actor.Kind != entity.ActorKindAgent {
		t.Fatalf("runner resolved to a %q actor, want it to act as its agent", actor.Kind)
	}

	if actor.AgentID == nil || *actor.AgentID != h.agent.ID {
		t.Fatalf("runner resolved to agent %v, want %s", actor.AgentID, h.agent.ID)
	}

	if actor.RunnerID == nil || *actor.RunnerID != runner.ID {
		t.Fatalf("runner resolved to runner %v, want %s", actor.RunnerID, runner.ID)
	}

	if actor.AccountID != h.agent.AccountID || actor.OwnerAccountID != h.agent.OwnerAccountID {
		t.Fatalf("runner acts as account %s owned by %s, want the agent's own",
			actor.AccountID, actor.OwnerAccountID)
	}

	if !actor.ConfinedTo(h.workspaceID) {
		t.Fatalf("runner actor is not confined to its workspace, so it could reach others")
	}

	if actor.Holds(entity.NewPermission(entity.ResourceLabel, entity.ActionManage)) {
		t.Fatalf("runner actor holds a scope its agent was never granted")
	}
}

func TestARevokedRunnerCannotAuthenticateEvenWithALiveAccessToken(t *testing.T) {
	h := newHarness(t)
	d := newDevice(t)

	runner := h.enrolled(d, "nrr_secret")
	runner.Status = entity.RunnerStatusRevoked

	h.sessions.EXPECT().Resolve(gomock.Any(), gomock.Any()).Return(runner.ID, nil)
	h.runners.EXPECT().GetByID(gomock.Any(), runner.ID).Return(runner, nil)

	if _, err := h.service.Authenticate(context.Background(), "nrs_access"); !errors.Is(err, entity.ErrRunnerRevoked) {
		t.Fatalf("a revoked runner authenticated and got %v, want it refused", err)
	}
}

func TestARunnerFromAnotherWorkspaceCannotBeRevokedThroughThisOne(t *testing.T) {
	h := newHarness(t)
	d := newDevice(t)

	runner := h.enrolled(d, "nrr_secret")
	runner.WorkspaceID = uuid.New()

	h.runners.EXPECT().GetByID(gomock.Any(), runner.ID).Return(runner, nil)

	if err := h.service.Revoke(context.Background(), h.workspaceID, runner.ID); !errors.Is(err, entity.ErrRunnerNotFound) {
		t.Fatalf("revoking another workspace's runner returned %v, want it refused", err)
	}
}

func TestRevokingAMachineRecordsItAndLeavesTheAgentAlone(t *testing.T) {
	h := newHarness(t)
	d := newDevice(t)

	runner := h.enrolled(d, "nrr_secret")

	h.runners.EXPECT().GetByID(gomock.Any(), runner.ID).Return(runner, nil)
	h.runners.EXPECT().Revoke(gomock.Any(), h.workspaceID, runner.ID, gomock.Any()).Return(nil)

	if err := h.service.Revoke(context.Background(), h.workspaceID, runner.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
}

func TestSelfIsRefusedWithoutARunnerOnTheActor(t *testing.T) {
	h := newHarness(t)

	if _, err := h.service.Self(context.Background()); !errors.Is(err, entity.ErrRunnerCredentialInvalid) {
		t.Fatalf("self without a runner actor returned %v, want it refused", err)
	}
}

func TestADisabledAgentCannotRenewOnAnyOfItsMachines(t *testing.T) {
	h := newHarness(t)
	d := newDevice(t)

	const refresh = "nrr_secret"

	runner := h.enrolled(d, refresh)

	disabled := h.agent
	disabled.Status = entity.AgentStatusDisabled

	h.expectRunnerByRefresh(runner, refresh)
	h.agents.EXPECT().GetByID(gomock.Any(), h.workspaceID, h.agent.ID).Return(disabled, nil)

	_, err := h.service.Exchange(context.Background(), h.exchanging(runner, refresh, d))
	if !errors.Is(err, entity.ErrAgentDisabled) {
		t.Fatalf(
			"a machine belonging to a disabled agent was handed a fresh token and got %v, want it refused",
			err,
		)
	}
}

func TestAnExchangeNamesTheAgentSoTheMachineNeedsNoSecondCall(t *testing.T) {
	h := newHarness(t)
	h.expectAgent()

	d := newDevice(t)

	const refresh = "nrr_secret"

	runner := h.enrolled(d, refresh)

	h.expectRunnerByRefresh(runner, refresh)
	h.expectFreshNonce()

	h.sessions.EXPECT().Grant(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	h.sessions.EXPECT().IssueTicket(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	h.runners.EXPECT().RecordSeen(gomock.Any(), runner.ID, gomock.Any()).Return(nil)

	session, err := h.service.Exchange(context.Background(), h.exchanging(runner, refresh, d))
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}

	if session.Runner.AgentName != h.agent.Name {
		t.Fatalf(
			"the session named the agent %q, want %q so a runner can say who it acts as",
			session.Runner.AgentName, h.agent.Name,
		)
	}
}
