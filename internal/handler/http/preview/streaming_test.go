package preview_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestAWebSocketUpgradeSurvivesTheWholeWayToTheMachine(t *testing.T) {
	h := newHarness(t)
	h.answers(h.allowed())

	h.served = func(w http.ResponseWriter, r *http.Request) {
		socket, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			return
		}

		defer func() { _ = socket.CloseNow() }()

		kind, payload, err := socket.Read(r.Context())
		if err != nil {
			return
		}

		_ = socket.Write(r.Context(), kind, append([]byte("echo "), payload...))
	}

	h.machine()

	ctx, cancel := context.WithTimeout(context.Background(), settle)
	defer cancel()

	socket, _, err := websocket.Dial(ctx, wsURL(h.server.URL)+"/live", &websocket.DialOptions{
		HTTPClient: &http.Client{Timeout: settle},
		Host:       testHost(),
	})
	if err != nil {
		t.Fatalf(
			"a websocket could not be opened through the gateway (%v); without upgrades there "+
				"is no hot reload and no live app behind a preview",
			err,
		)
	}

	defer func() { _ = socket.CloseNow() }()

	if err := socket.Write(ctx, websocket.MessageText, []byte("hello")); err != nil {
		t.Fatalf("write through the tunnel: %v", err)
	}

	_, payload, err := socket.Read(ctx)
	if err != nil {
		t.Fatalf("read back through the tunnel: %v", err)
	}

	if string(payload) != "echo hello" {
		t.Fatalf("the machine answered %q, want %q", payload, "echo hello")
	}
}

func TestAStreamedResponseReachesTheBrowserBeforeItEnds(t *testing.T) {
	h := newHarness(t)
	h.answers(h.allowed())

	holding := make(chan struct{})

	h.served = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}

		_, _ = w.Write([]byte("data: first\n\n"))
		flusher.Flush()

		select {
		case <-holding:
		case <-r.Context().Done():
		}
	}

	h.machine()
	defer close(holding)

	response := h.get("/events", true)

	if response.StatusCode != http.StatusOK {
		t.Fatalf("a stream answered %d, want 200", response.StatusCode)
	}

	first := make([]byte, len("data: first\n\n"))

	done := make(chan error, 1)

	go func() {
		_, err := response.Body.Read(first)
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("read the first event: %v", err)
		}
	case <-time.After(settle):
		t.Fatal(
			"nothing arrived until the response ended, so the gateway is buffering; a live " +
				"reload channel or an event stream would look hung behind it",
		)
	}

	if !strings.Contains(string(first), "first") {
		t.Fatalf("the first event read back as %q", first)
	}
}

func wsURL(address string) string {
	return "ws" + strings.TrimPrefix(address, "http")
}
