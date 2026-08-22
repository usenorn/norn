package runnerchannel_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
)

func TestOpeningAChannelSpendsTheTicketAndAnnouncesWhatTheServerBelieves(t *testing.T) {
	h := newHarness(t)

	h.opening("nrt_ticket")

	var announced entity.ChannelMessage

	h.channels.EXPECT().
		Append(gomock.Any(), h.runner.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, message entity.ChannelMessage) (string, error) {
			announced = message

			return "1-0", nil
		})

	session, err := h.service.Open(context.Background(), "nrt_ticket")
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	if session.Runner.ID != h.runner.ID || session.Epoch == "" {
		t.Fatalf("the session does not identify the connection: %+v", session)
	}

	if announced.Type != entity.ChannelSync {
		t.Fatalf("the server announced %q on connect, want channel.sync", announced.Type)
	}

	if announced.ID == "" {
		t.Fatalf("the sync carries no id, so the runner could never acknowledge it")
	}

	if string(announced.Payload) == "" {
		t.Fatalf("the sync carries no payload, so a runner has nothing to reconcile against")
	}
}

func TestATicketThatCannotBeSpentOpensNothing(t *testing.T) {
	h := newHarness(t)

	h.sessions.EXPECT().
		RedeemTicket(gomock.Any(), gomock.Any()).
		Return(uuid.Nil, entity.ErrRunnerCredentialInvalid)

	_, err := h.service.Open(context.Background(), "nrt_stale")
	if !errors.Is(err, entity.ErrChannelTicketInvalid) {
		t.Fatalf("a spent ticket returned %v, want it refused", err)
	}
}

func TestARevokedRunnerCannotOpenAChannelEvenWithAFreshTicket(t *testing.T) {
	h := newHarness(t)

	h.sessions.EXPECT().RedeemTicket(gomock.Any(), gomock.Any()).Return(h.runner.ID, nil)
	h.machines.EXPECT().
		ActorFor(gomock.Any(), h.runner.ID).
		Return(entity.Actor{}, entity.Runner{}, entity.ErrRunnerRevoked)

	_, err := h.service.Open(context.Background(), "nrt_ticket")
	if !errors.Is(err, entity.ErrRunnerRevoked) {
		t.Fatalf("a revoked runner opened a channel and got %v", err)
	}
}

func TestAHeartbeatHoldsTheChannelAndNoticesRevocation(t *testing.T) {
	h := newHarness(t)
	session := h.session()

	h.channels.EXPECT().
		Renew(gomock.Any(), h.runner.ID, session.Epoch, gomock.Any()).
		Return(nil).
		Times(2)

	h.runners.EXPECT().GetByID(gomock.Any(), h.runner.ID).Return(h.runner, nil)
	h.runners.EXPECT().RecordSeen(gomock.Any(), h.runner.ID, gomock.Any()).Return(nil)

	if err := h.service.Heartbeat(context.Background(), session); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}

	revoked := h.runner
	revoked.Status = entity.RunnerStatusRevoked

	h.runners.EXPECT().GetByID(gomock.Any(), h.runner.ID).Return(revoked, nil)

	if err := h.service.Heartbeat(context.Background(), session); !errors.Is(err, entity.ErrRunnerRevoked) {
		t.Fatalf(
			"a runner revoked while its socket was open kept heartbeating and got %v; revoking "+
				"must drop the connection",
			err,
		)
	}
}

func TestADisplacedConnectionStopsHoldingTheChannel(t *testing.T) {
	h := newHarness(t)
	session := h.session()

	h.channels.EXPECT().
		Renew(gomock.Any(), h.runner.ID, session.Epoch, gomock.Any()).
		Return(entity.ErrChannelDisplaced)

	if err := h.service.Heartbeat(context.Background(), session); !errors.Is(err, entity.ErrChannelDisplaced) {
		t.Fatalf("a displaced connection kept its lease and got %v", err)
	}
}

func TestARedeliveredMessageIsNotActedOnTwice(t *testing.T) {
	h := newHarness(t)
	session := h.session()

	message := h.freshMessage("01ABC", entity.ChannelRunnerHeartbeat)

	h.channels.EXPECT().Seen(gomock.Any(), h.runner.ID, message.ID).Return(false, nil)
	h.channels.EXPECT().Renew(gomock.Any(), h.runner.ID, session.Epoch, gomock.Any()).Return(nil)
	h.runners.EXPECT().GetByID(gomock.Any(), h.runner.ID).Return(h.runner, nil)
	h.runners.EXPECT().RecordSeen(gomock.Any(), h.runner.ID, gomock.Any()).Return(nil)

	if err := h.service.Receive(context.Background(), session, message); err != nil {
		t.Fatalf("first delivery: %v", err)
	}

	h.channels.EXPECT().Seen(gomock.Any(), h.runner.ID, message.ID).Return(true, nil)

	if err := h.service.Receive(context.Background(), session, message); err != nil {
		t.Fatalf("second delivery: %v", err)
	}
}

func TestSayingHelloTellsTheServerWhichBuildIsNowOnTheMachine(t *testing.T) {
	h := newHarness(t)
	session := h.session()

	message := h.freshMessage("01HELLO", entity.ChannelRunnerHello)
	message.Payload = []byte(`{"version":"1.4.0","protocol":1,"capacity":2}`)

	h.channels.EXPECT().Seen(gomock.Any(), h.runner.ID, message.ID).Return(false, nil)
	h.channels.EXPECT().Renew(gomock.Any(), h.runner.ID, session.Epoch, gomock.Any()).Return(nil)
	h.runners.EXPECT().GetByID(gomock.Any(), h.runner.ID).Return(h.runner, nil)
	h.runners.EXPECT().RecordSeen(gomock.Any(), h.runner.ID, gomock.Any()).Return(nil)
	h.runners.EXPECT().RecordVersion(gomock.Any(), h.runner.ID, "1.4.0").Return(nil)

	if err := h.service.Receive(context.Background(), session, message); err != nil {
		t.Fatalf("saying hello: %v", err)
	}
}

func TestAHelloThatNamesTheVersionAlreadyOnRecordChangesNothing(t *testing.T) {
	h := newHarness(t)
	h.runner.Host.Version = "1.4.0"
	session := h.session()

	message := h.freshMessage("01HELLO", entity.ChannelRunnerHello)
	message.Payload = []byte(`{"version":"1.4.0","protocol":1}`)

	h.channels.EXPECT().Seen(gomock.Any(), h.runner.ID, message.ID).Return(false, nil)
	h.channels.EXPECT().Renew(gomock.Any(), h.runner.ID, session.Epoch, gomock.Any()).Return(nil)
	h.runners.EXPECT().GetByID(gomock.Any(), h.runner.ID).Return(h.runner, nil)
	h.runners.EXPECT().RecordSeen(gomock.Any(), h.runner.ID, gomock.Any()).Return(nil)

	if err := h.service.Receive(context.Background(), session, message); err != nil {
		t.Fatalf("saying hello: %v", err)
	}
}

func TestAMessageForSomethingTheServerCannotDoYetIsStillAccepted(t *testing.T) {
	h := newHarness(t)

	message := h.freshMessage("01DEF", entity.ChannelChangeSetUpdated)

	h.channels.EXPECT().Seen(gomock.Any(), h.runner.ID, message.ID).Return(false, nil)

	if err := h.service.Receive(context.Background(), h.session(), message); err != nil {
		t.Fatalf(
			"a message whose feature has not been built refused the whole connection: %v; the "+
				"runner would reconnect forever",
			err,
		)
	}
}

func TestWhatARunnerReportsAboutAnExecutionReachesTheExecution(t *testing.T) {
	cases := map[entity.ChannelMessageType]func(*harness) *gomock.Call{
		entity.ChannelExecutionAccepted: func(h *harness) *gomock.Call {
			return h.executions.EXPECT().Accepted(gomock.Any(), h.runner, gomock.Any())
		},
		entity.ChannelExecutionDeclined: func(h *harness) *gomock.Call {
			return h.executions.EXPECT().Declined(gomock.Any(), h.runner, gomock.Any())
		},
		entity.ChannelExecutionState: func(h *harness) *gomock.Call {
			return h.executions.EXPECT().Reported(gomock.Any(), h.runner, gomock.Any())
		},
		entity.ChannelExecutionEvent: func(h *harness) *gomock.Call {
			return h.executions.EXPECT().Observed(gomock.Any(), h.runner, gomock.Any())
		},
	}

	for kind, expect := range cases {
		t.Run(string(kind), func(t *testing.T) {
			h := newHarness(t)
			message := h.freshMessage("01MNO", kind)

			h.channels.EXPECT().Seen(gomock.Any(), h.runner.ID, message.ID).Return(false, nil)
			expect(h).Return(nil)

			if err := h.service.Receive(context.Background(), h.session(), message); err != nil {
				t.Fatalf("%s was not acted on: %v", kind, err)
			}
		})
	}
}

func TestAReconnectingRunnerIsToldWhatItIsStillHolding(t *testing.T) {
	h := newHarness(t)

	h.opening("ticket", "exec-01ABC")
	h.channels.EXPECT().
		Append(gomock.Any(), h.runner.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, message entity.ChannelMessage) (string, error) {
			if message.Type != entity.ChannelSync {
				t.Fatalf("the first message on a fresh channel is %q, want %q",
					message.Type, entity.ChannelSync)
			}

			if !strings.Contains(string(message.Payload), "exec-01ABC") {
				t.Fatalf("the sync carries %s, which does not name the leased execution",
					message.Payload)
			}

			return "1-0", nil
		})

	if _, err := h.service.Open(context.Background(), "ticket"); err != nil {
		t.Fatalf("open a channel: %v", err)
	}
}

func TestAMessageTheProtocolDoesNotDefineIsRefused(t *testing.T) {
	h := newHarness(t)

	cases := map[string]entity.ChannelMessage{
		"an invented type":          {ID: "01GHI", Type: "execution.teleport"},
		"one only the server sends": {ID: "01JKL", Type: entity.ChannelExecutionStart},
		"no id at all":              {Type: entity.ChannelRunnerHello},
	}

	for name, message := range cases {
		t.Run(name, func(t *testing.T) {
			if err := h.service.Receive(context.Background(), h.session(), message); err == nil {
				t.Fatalf("%s was accepted", name)
			}
		})
	}
}

func TestAConnectionThatLostItsPlaceStopsBeingVerified(t *testing.T) {
	h := newHarness(t)
	session := h.session()

	h.channels.EXPECT().
		Presence(gomock.Any(), h.runner.ID).
		Return(entity.RunnerPresence{RunnerID: h.runner.ID, Epoch: "somebody-else"}, nil)

	if err := h.service.Verify(context.Background(), session); !errors.Is(err, entity.ErrChannelDisplaced) {
		t.Fatalf("a displaced connection verified successfully and got %v", err)
	}
}

func TestARunnerThatWentSilentLosesItsChannel(t *testing.T) {
	h := newHarness(t)

	h.channels.EXPECT().
		Presence(gomock.Any(), h.runner.ID).
		Return(entity.RunnerPresence{RunnerID: h.runner.ID}, nil)

	err := h.service.Verify(context.Background(), h.session())
	if !errors.Is(err, entity.ErrChannelDisplaced) {
		t.Fatalf(
			"a runner whose lease expired kept its socket and got %v; a silent machine must not "+
				"hold a connection open forever",
			err,
		)
	}
}

func TestARunnerRevokedMidConnectionFailsVerification(t *testing.T) {
	h := newHarness(t)
	session := h.session()

	revoked := h.runner
	revoked.Status = entity.RunnerStatusRevoked

	h.channels.EXPECT().
		Presence(gomock.Any(), h.runner.ID).
		Return(entity.RunnerPresence{RunnerID: h.runner.ID, Epoch: session.Epoch}, nil)

	h.runners.EXPECT().GetByID(gomock.Any(), h.runner.ID).Return(revoked, nil)

	if err := h.service.Verify(context.Background(), session); !errors.Is(err, entity.ErrRunnerRevoked) {
		t.Fatalf("a revoked runner passed verification and got %v", err)
	}
}
