package tunnel_test

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/yamux"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	tunnelrepo "github.com/usenorn/norn/internal/repository/tunnel"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

const settle = 2 * time.Second

func settings(streams int) config.Gateway {
	return config.Gateway{
		MaxStreamsPerRunner: streams,
		Heartbeat:           time.Second,
		RequestTimeout:      settle,
		StreamOpenTimeout:   settle,
	}
}

type machine struct {
	session *yamux.Session
	opened  chan channelv1.StreamOpen
}

func attach(
	t *testing.T,
	cfg config.Gateway,
	runnerID uuid.UUID,
	answer func(channelv1.StreamOpen) bool,
) (repository.Tunnel, *machine) {
	t.Helper()

	tunnels := tunnelrepo.New(cfg)
	here, there := net.Pipe()

	ctx, stop := context.WithCancel(context.Background())
	held := make(chan struct{})

	go func() {
		defer close(held)

		_ = tunnels.Hold(ctx, runnerID, here)
	}()

	client, err := yamux.Client(there, yamux.DefaultConfig())
	if err != nil {
		t.Fatalf("open the machine side of the tunnel: %v", err)
	}

	holder := &machine{session: client, opened: make(chan channelv1.StreamOpen, 8)}

	go holder.accept(answer)

	t.Cleanup(func() {
		_ = client.Close()
		stop()
		<-held
	})

	waitFor(t, func() bool { return tunnels.Live(runnerID) })

	return tunnels, holder
}

func (m *machine) accept(answer func(channelv1.StreamOpen) bool) {
	for {
		stream, err := m.session.AcceptStream()
		if err != nil {
			return
		}

		go m.serve(stream, answer)
	}
}

func (m *machine) serve(stream net.Conn, answer func(channelv1.StreamOpen) bool) {
	reader := bufio.NewReader(stream)

	var open channelv1.StreamOpen

	if err := channelv1.ReadFrame(reader, &open); err != nil {
		_ = stream.Close()

		return
	}

	m.opened <- open

	if !answer(open) {
		_ = channelv1.WriteFrame(stream, channelv1.StreamReady{Reason: "not here"})
		_ = stream.Close()

		return
	}

	_ = channelv1.WriteFrame(stream, channelv1.StreamReady{Open: true})

	_ = http.Serve(&single{stream: stream}, http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "served "+r.URL.Path)
		},
	))
}

type single struct {
	stream net.Conn
	spent  bool
}

func (l *single) Addr() net.Addr { return l.stream.LocalAddr() }

func (l *single) Close() error { return nil }

func (l *single) Accept() (net.Conn, error) {
	if l.spent {
		return nil, errors.New("spent")
	}

	l.spent = true

	return l.stream, nil
}

func waitFor(t *testing.T, ready func() bool) {
	t.Helper()

	deadline := time.Now().Add(settle)

	for time.Now().Before(deadline) {
		if ready() {
			return
		}

		time.Sleep(time.Millisecond)
	}

	t.Fatal("the tunnel never became live")
}

func TestAStreamCarriesThePairItWasOpenedFor(t *testing.T) {
	runnerID := uuid.New()

	tunnels, holder := attach(t, settings(4), runnerID, func(channelv1.StreamOpen) bool {
		return true
	})

	stream, err := tunnels.Open(context.Background(), runnerID, channelv1.StreamOpen{
		Execution: "exec-01ABC",
		Preview:   "web",
	})
	if err != nil {
		t.Fatalf("open a stream: %v", err)
	}

	defer func() { _ = stream.Close() }()

	opened := <-holder.opened

	if opened.Execution != "exec-01ABC" || opened.Preview != "web" {
		t.Fatalf(
			"the machine was asked for %s/%s; a stream that does not name its pair is a stream "+
				"the machine cannot refuse",
			opened.Execution, opened.Preview,
		)
	}
}

func TestAMachineThatRefusesAPairIsNotProxiedTo(t *testing.T) {
	runnerID := uuid.New()

	tunnels, _ := attach(t, settings(4), runnerID, func(channelv1.StreamOpen) bool {
		return false
	})

	_, err := tunnels.Open(context.Background(), runnerID, channelv1.StreamOpen{
		Execution: "exec-01ABC",
		Preview:   "web",
	})

	if !errors.Is(err, entity.ErrTunnelRefused) {
		t.Fatalf(
			"opening a stream the machine refused answered %v, want %v; a refusal the gateway "+
				"treats as success would proxy a browser into nothing",
			err, entity.ErrTunnelRefused,
		)
	}
}

func TestAMachineHoldingNoTunnelIsSaidToBeOfflineRatherThanWaitedOn(t *testing.T) {
	tunnels := tunnelrepo.New(settings(4))

	_, err := tunnels.Open(context.Background(), uuid.New(), channelv1.StreamOpen{
		Execution: "exec-01ABC",
		Preview:   "web",
	})

	if !errors.Is(err, entity.ErrTunnelMissing) {
		t.Fatalf(
			"asking a machine that is not connected answered %v, want %v; anything else leaves "+
				"a browser waiting on a machine that will never answer",
			err, entity.ErrTunnelMissing,
		)
	}
}

func TestOneMachineCarriesOnlyAsManyStreamsAsItMay(t *testing.T) {
	runnerID := uuid.New()

	tunnels, _ := attach(t, settings(1), runnerID, func(channelv1.StreamOpen) bool {
		return true
	})

	open := channelv1.StreamOpen{Execution: "exec-01ABC", Preview: "web"}

	first, err := tunnels.Open(context.Background(), runnerID, open)
	if err != nil {
		t.Fatalf("open the first stream: %v", err)
	}

	if _, err := tunnels.Open(context.Background(), runnerID, open); !errors.Is(
		err, entity.ErrTunnelCrowded,
	) {
		t.Fatalf(
			"a second stream past the cap answered %v, want %v; without a cap one page of "+
				"images could take a machine's whole tunnel",
			err, entity.ErrTunnelCrowded,
		)
	}

	if err := first.Close(); err != nil {
		t.Fatalf("close the first stream: %v", err)
	}

	again, err := tunnels.Open(context.Background(), runnerID, open)
	if err != nil {
		t.Fatalf(
			"a stream closed and gave its place back, but the next one was still refused: %v",
			err,
		)
	}

	_ = again.Close()
}

func TestASecondTunnelFromOneMachineDisplacesTheFirst(t *testing.T) {
	runnerID := uuid.New()
	cfg := settings(4)
	tunnels := tunnelrepo.New(cfg)

	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	first, firstThere := net.Pipe()
	second, secondThere := net.Pipe()

	closed := make(chan struct{})

	go func() {
		defer close(closed)

		_ = tunnels.Hold(ctx, runnerID, first)
	}()

	waitFor(t, func() bool { return tunnels.Live(runnerID) })

	go func() { _ = tunnels.Hold(ctx, runnerID, second) }()

	select {
	case <-closed:
	case <-time.After(settle):
		t.Fatal(
			"a machine that reconnected left its old tunnel held, so requests would keep going " +
				"to a connection nothing is reading",
		)
	}

	_ = firstThere.Close()
	_ = secondThere.Close()
}
