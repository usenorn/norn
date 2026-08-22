package runnerchannel_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	edge "github.com/usenorn/norn/internal/handler/http/runnerchannel"
	"github.com/usenorn/norn/internal/service"
	channelsvc "github.com/usenorn/norn/internal/service/runnerchannel"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

type frame struct {
	V       int             `json:"v"`
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	TS      time.Time       `json:"ts"`
	Payload json.RawMessage `json:"payload,omitempty"`
	AckID   string          `json:"ack_id,omitempty"`
}

type stand struct {
	channels *channelsvc.MockRunnerChannels
	server   *httptest.Server
	session  service.ChannelSession
}

func newStand(t *testing.T) *stand {
	t.Helper()

	return newStandNeeding(t, "")
}

func newStandNeeding(t *testing.T, minimum string) *stand {
	t.Helper()

	ctrl := gomock.NewController(t)

	runnerID := uuid.New()
	channels := channelsvc.NewMockRunnerChannels(ctrl)

	held := &stand{
		channels: channels,
		session: service.ChannelSession{
			Runner: entity.Runner{ID: runnerID, Name: "vlad-mbp"},
			Epoch:  "epoch",
			Cursor: "0-0",
		},
	}

	held.server = httptest.NewServer(http.HandlerFunc(edge.New(channels, config.Runner{
		ChannelEnabled: true,
		MinimumVersion: minimum,
	}).Serve))

	t.Cleanup(held.server.Close)

	return held
}

func (s *stand) url() string {
	return "ws" + strings.TrimPrefix(s.server.URL, "http") + edge.Path
}

func TestAConnectedRunnerIsToldWhatTheServerBelievesAndIsAnsweredWhenItSpeaks(t *testing.T) {
	s := newStand(t)

	s.channels.EXPECT().Open(gomock.Any(), "nrt_good").Return(s.session, nil)
	s.channels.EXPECT().Close(gomock.Any(), gomock.Any()).AnyTimes()
	s.channels.EXPECT().Verify(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	var delivered atomic.Bool

	s.channels.EXPECT().
		Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, _ service.ChannelSession, cursor string) ([]entity.SpooledMessage, string, error) {
			if delivered.Swap(true) {
				<-ctx.Done()

				return nil, cursor, ctx.Err()
			}

			return []entity.SpooledMessage{{
				Cursor: "1-0",
				Message: entity.ChannelMessage{
					ID:       "01SYNC",
					Type:     entity.ChannelSync,
					IssuedAt: time.Now().UTC(),
					Payload:  []byte(`{"executions":[]}`),
				},
			}}, "1-0", nil
		}).
		AnyTimes()

	s.channels.EXPECT().
		Receive(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ service.ChannelSession, message entity.ChannelMessage) error {
			if message.Type != entity.ChannelRunnerHello {
				t.Errorf("the server was handed a %q, want runner.hello", message.Type)
			}

			return nil
		})

	s.channels.EXPECT().Acknowledge(gomock.Any(), gomock.Any(), "1-0").Return(nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	socket, _, err := websocket.Dial(ctx, s.url()+"?ticket=nrt_good", nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	defer func() { _ = socket.CloseNow() }()

	sync := read(t, ctx, socket)
	if sync.Type != string(entity.ChannelSync) {
		t.Fatalf("the first frame was %q, want channel.sync", sync.Type)
	}

	if sync.V != entity.ChannelVersion {
		t.Fatalf("the envelope carried version %d, want %d", sync.V, entity.ChannelVersion)
	}

	write(t, ctx, socket, frame{
		V: entity.ChannelVersion, Type: string(entity.ChannelAck), AckID: sync.ID,
	})

	write(t, ctx, socket, frame{
		V: entity.ChannelVersion, ID: "01HELLO", Type: string(entity.ChannelRunnerHello),
		TS: time.Now().UTC(),
	})

	answer := read(t, ctx, socket)
	if answer.Type != string(entity.ChannelAck) || answer.AckID != "01HELLO" {
		t.Fatalf("the server answered %+v, want an ack for 01HELLO", answer)
	}
}

func TestATicketTheServerRefusesNeverBecomesASocket(t *testing.T) {
	s := newStand(t)

	s.channels.EXPECT().
		Open(gomock.Any(), "nrt_stale").
		Return(service.ChannelSession{}, entity.ErrChannelTicketInvalid)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	socket, response, err := websocket.Dial(ctx, s.url()+"?ticket=nrt_stale", nil)
	if err == nil {
		_ = socket.CloseNow()

		t.Fatalf("a refused ticket still opened a socket")
	}

	if response == nil || response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("a refused ticket answered %v, want 401 before the upgrade", response)
	}
}

func TestOpeningTheChannelWithoutATicketIsRefused(t *testing.T) {
	s := newStand(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	socket, response, err := websocket.Dial(ctx, s.url(), nil)
	if err == nil {
		_ = socket.CloseNow()

		t.Fatalf("a connection with no ticket was upgraded")
	}

	if response == nil || response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("a ticketless connection answered %v, want 401", response)
	}
}

func TestARunnerBelowTheServersMinimumIsTurnedAwayBeforeItsTicketIsSpent(t *testing.T) {
	s := newStandNeeding(t, "1.2.0")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	socket, response, err := websocket.Dial(ctx, s.url()+"?ticket=nrt_good&version=0.9.0", nil)
	if err == nil {
		_ = socket.CloseNow()

		t.Fatalf("a runner older than this server needs still opened a socket")
	}

	if response == nil || response.StatusCode != http.StatusUpgradeRequired {
		t.Fatalf("an outdated runner answered %v, want 426 before the upgrade", response)
	}

	var problem api.RunnerProblem

	if err := json.NewDecoder(response.Body).Decode(&problem); err != nil {
		t.Fatalf("the refusal was not a problem document: %v", err)
	}

	if problem.Code != api.RunnerProblemCodeRunnerOutdated {
		t.Fatalf("the refusal carried code %q, want runner_outdated", problem.Code)
	}

	if problem.Detail == nil || !strings.Contains(*problem.Detail, "1.2.0") {
		t.Fatalf("the refusal does not name the version to upgrade to: %v", problem.Detail)
	}

	if !strings.Contains(*problem.Detail, channelv1.InstallRunner) {
		t.Fatalf("the refusal does not say how to take the new build: %v", *problem.Detail)
	}
}

func TestABuildWithNoReleasedVersionIsNeverTurnedAwayForIt(t *testing.T) {
	s := newStandNeeding(t, "1.2.0")

	s.channels.EXPECT().Open(gomock.Any(), "nrt_good").Return(s.session, nil)
	s.channels.EXPECT().Close(gomock.Any(), gomock.Any()).AnyTimes()
	s.channels.EXPECT().Verify(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.channels.EXPECT().
		Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, _ service.ChannelSession, cursor string) ([]entity.SpooledMessage, string, error) {
			<-ctx.Done()

			return nil, cursor, ctx.Err()
		}).
		AnyTimes()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	socket, _, err := websocket.Dial(ctx, s.url()+"?ticket=nrt_good&version=dev", nil)
	if err != nil {
		t.Fatalf("a development build was turned away by the version gate: %v", err)
	}

	_ = socket.CloseNow()
}

func TestAnEnvelopeFromAnotherProtocolVersionEndsTheChannel(t *testing.T) {
	s := newStand(t)

	s.channels.EXPECT().Open(gomock.Any(), "nrt_good").Return(s.session, nil)
	s.channels.EXPECT().Close(gomock.Any(), gomock.Any()).AnyTimes()
	s.channels.EXPECT().Verify(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.channels.EXPECT().
		Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, _ service.ChannelSession, cursor string) ([]entity.SpooledMessage, string, error) {
			<-ctx.Done()

			return nil, cursor, ctx.Err()
		}).
		AnyTimes()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	socket, _, err := websocket.Dial(ctx, s.url()+"?ticket=nrt_good", nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	defer func() { _ = socket.CloseNow() }()

	write(t, ctx, socket, frame{
		V: entity.ChannelVersion + 1, ID: "01HELLO",
		Type: string(entity.ChannelRunnerHello), TS: time.Now().UTC(),
	})

	if _, _, err := socket.Read(ctx); websocket.CloseStatus(err) != websocket.StatusUnsupportedData {
		t.Fatalf("an envelope from another protocol version ended with %v, want 1003", err)
	}
}

func read(t *testing.T, ctx context.Context, socket *websocket.Conn) frame {
	t.Helper()

	kind, raw, err := socket.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if kind != websocket.MessageText {
		t.Fatalf("the server sent a %v frame, want text", kind)
	}

	var decoded frame

	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("the server sent something that is not an envelope: %v", err)
	}

	return decoded
}

func write(t *testing.T, ctx context.Context, socket *websocket.Conn, body frame) {
	t.Helper()

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if err := socket.Write(ctx, websocket.MessageText, raw); err != nil {
		t.Fatalf("write: %v", err)
	}
}
