package preview_test

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/yamux"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	previewedge "github.com/usenorn/norn/internal/handler/http/preview"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/repository/nornapi"
	tunnelrepo "github.com/usenorn/norn/internal/repository/tunnel"
	"github.com/usenorn/norn/internal/service/previewproxy"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

const (
	testDomain    = "norn.ink"
	testExecution = "exec-01ABCDEF"
	testPreview   = "web"
	testGrant     = "a-grant-the-gateway-carried"
	settle        = 2 * time.Second
)

func testHost() string {
	return testPreview + "-" + "exec-01abcdef." + testDomain
}

type harness struct {
	t *testing.T

	norn     *nornapi.MockNorn
	runnerID uuid.UUID
	server   *httptest.Server
	tunnels  repository.Tunnel
	client   *http.Client
	served   func(http.ResponseWriter, *http.Request)
	streams  chan channelv1.StreamOpen

	mu     sync.Mutex
	host   string
	grant  string
	scheme string
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	return newHarnessOn(t, "http")
}

func newHarnessOn(t *testing.T, scheme string) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		t:        t,
		scheme:   scheme,
		norn:     nornapi.NewMockNorn(ctrl),
		runnerID: uuid.New(),
		streams:  make(chan channelv1.StreamOpen, 8),
		served: func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("served " + r.URL.Path + " for " + r.Host))
		},
	}

	cfg := config.Gateway{
		Secret:              "ngr_secret",
		Server:              "https://app.norn.test",
		MaxStreamsPerRunner: 16,
		RequestTimeout:      settle,
		StreamOpenTimeout:   settle,
		Heartbeat:           time.Second,
		RefreshLead:         time.Minute,
		RetryMin:            10 * time.Millisecond,
		RetryMax:            time.Second,
	}

	previews := config.Previews{BaseDomain: testDomain, Scheme: scheme}

	h.norn.EXPECT().
		Exchange(gomock.Any(), "ngr_secret").
		Return(entity.PreviewGatewayToken{
			Token:     "nga_token",
			ExpiresAt: time.Now().UTC().Add(time.Hour),
		}, nil).
		AnyTimes()

	tunnels := tunnelrepo.New(cfg)
	proxies := previewproxy.New(h.norn, tunnels, cfg)

	ctx, stop := context.WithCancel(context.Background())
	go proxies.Run(ctx)

	edge := previewedge.New(proxies, previews, cfg)

	h.server = httptest.NewServer(http.HandlerFunc(edge.Serve))
	h.client = &http.Client{
		Timeout: settle,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	ready(t, func() bool { return proxies.Ready() })

	t.Cleanup(func() {
		h.server.Close()
		stop()
	})

	h.tunnels = tunnels

	return h
}

func ready(t *testing.T, is func() bool) {
	t.Helper()

	deadline := time.Now().Add(settle)

	for time.Now().Before(deadline) {
		if is() {
			return
		}

		time.Sleep(time.Millisecond)
	}

	t.Fatal("the gateway never traded its secret for an access token")
}

func (h *harness) answers(reply entity.PreviewReply) {
	h.t.Helper()

	h.norn.EXPECT().
		Introspect(gomock.Any(), "nga_token", gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ string, ask entity.PreviewAsk,
		) (entity.PreviewReply, error) {
			h.mu.Lock()
			h.host = ask.Host
			h.grant = ask.Grant
			h.mu.Unlock()

			return reply, nil
		}).
		AnyTimes()
}

func (h *harness) asked() string {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.host
}

func (h *harness) allowed() entity.PreviewReply {
	return entity.PreviewReply{
		Verdict:     entity.PreviewAllowed,
		ExecutionID: testExecution,
		RunnerID:    h.runnerID,
		Preview:     testPreview,
		Mode:        entity.PreviewBySubdomain,
		ExpiresAt:   time.Now().UTC().Add(time.Hour),
	}
}

func (h *harness) machine() {
	h.t.Helper()

	here, there := net.Pipe()

	ctx, stop := context.WithCancel(context.Background())

	go func() { _ = h.tunnels.Hold(ctx, h.runnerID, here) }()

	client, err := yamux.Client(there, yamux.DefaultConfig())
	if err != nil {
		h.t.Fatalf("open the machine side of the tunnel: %v", err)
	}

	go h.accept(client)

	ready(h.t, func() bool { return h.tunnels.Live(h.runnerID) })

	h.t.Cleanup(func() {
		_ = client.Close()
		stop()
	})
}

func (h *harness) accept(session *yamux.Session) {
	for {
		stream, err := session.AcceptStream()
		if err != nil {
			return
		}

		go h.hand(stream)
	}
}

func (h *harness) hand(stream net.Conn) {
	reader := bufio.NewReader(stream)

	var open channelv1.StreamOpen

	if err := channelv1.ReadFrame(reader, &open); err != nil {
		_ = stream.Close()

		return
	}

	h.streams <- open

	if open.Execution != testExecution || open.Preview != testPreview {
		_ = channelv1.WriteFrame(stream, channelv1.StreamReady{Reason: "not mine"})
		_ = stream.Close()

		return
	}

	_ = channelv1.WriteFrame(stream, channelv1.StreamReady{Open: true})

	_ = http.Serve(&once{stream: &joined{Conn: stream, reader: reader}}, http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) { h.served(w, r) },
	))
}

type joined struct {
	net.Conn

	reader *bufio.Reader
}

func (c *joined) Read(into []byte) (int, error) { return c.reader.Read(into) }

type once struct {
	stream net.Conn
	spent  bool
}

func (l *once) Addr() net.Addr { return l.stream.LocalAddr() }

func (l *once) Close() error { return nil }

func (l *once) Accept() (net.Conn, error) {
	if l.spent {
		return nil, errors.New("this stream carries one request")
	}

	l.spent = true

	return l.stream, nil
}

func (h *harness) get(path string, carrying bool) *http.Response {
	h.t.Helper()

	request, err := http.NewRequest(http.MethodGet, h.server.URL+path, nil)
	if err != nil {
		h.t.Fatalf("build the request: %v", err)
	}

	request.Host = testHost()

	if carrying {
		request.AddCookie(&http.Cookie{
			Name: entity.PreviewGrantCookieName(h.scheme == "https"), Value: testGrant,
		})
	}

	response, err := h.client.Do(request)
	if err != nil {
		h.t.Fatalf("ask the gateway for %s: %v", path, err)
	}

	h.t.Cleanup(func() { _ = response.Body.Close() })

	return response
}

func (h *harness) grantAsked() string {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.grant
}

func (h *harness) opened() <-chan channelv1.StreamOpen {
	return h.streams
}
