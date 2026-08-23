package preview_test

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

func read(t *testing.T, response *http.Response) string {
	t.Helper()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read what the gateway answered: %v", err)
	}

	return string(body)
}

func TestAPreviewIsCarriedToTheMachineRunningIt(t *testing.T) {
	h := newHarness(t)
	h.answers(h.allowed())
	h.machine()

	response := h.get("/dashboard", true)

	if response.StatusCode != http.StatusOK {
		t.Fatalf("the gateway answered %d for an open preview, want 200", response.StatusCode)
	}

	body := read(t, response)

	if !strings.Contains(body, "served /dashboard") {
		t.Fatalf(
			"the machine answered %q; the browser has to reach the service itself, not a page "+
				"the gateway wrote",
			body,
		)
	}

	if !strings.Contains(body, testHost()) {
		t.Fatalf(
			"the service was asked for host %q; a preview that arrives under the gateway's own "+
				"hostname breaks every absolute url and cookie the app sets",
			body,
		)
	}
}

func TestAHostNoMachineEverRegisteredNeverReachesATunnel(t *testing.T) {
	h := newHarness(t)
	h.answers(entity.PreviewReply{
		Verdict: entity.PreviewRefused,
		Reason:  entity.ErrPreviewNotFound.Error(),
	})
	h.machine()

	response := h.get("/", true)

	if response.StatusCode != http.StatusNotFound {
		t.Fatalf(
			"an unregistered host answered %d, want 404; a hostname that reaches a dial is a "+
				"path from a guessed name to somebody's laptop",
			response.StatusCode,
		)
	}

	select {
	case open := <-h.opened():
		t.Fatalf(
			"the gateway opened a stream for %s/%s before norn had said the pair exists",
			open.Execution, open.Preview,
		)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestAClosedPreviewSaysSoRatherThanFailingToConnect(t *testing.T) {
	h := newHarness(t)
	h.answers(entity.PreviewReply{
		Verdict:     entity.PreviewRefused,
		ExecutionID: testExecution,
		Preview:     testPreview,
		Reason:      entity.ErrPreviewClosed.Error(),
	})
	h.machine()

	response := h.get("/", true)

	if response.StatusCode != http.StatusGone {
		t.Fatalf("a closed preview answered %d, want 410", response.StatusCode)
	}

	if body := read(t, response); !strings.Contains(body, "closed") {
		t.Fatalf("the page for a closed preview does not say it is closed: %q", body)
	}
}

func TestAViewerWithNoSessionIsSentIntoSignInRatherThanProxied(t *testing.T) {
	h := newHarness(t)
	h.answers(entity.PreviewReply{
		Verdict:     entity.PreviewSignIn,
		ExecutionID: testExecution,
		Preview:     testPreview,
		Redirect:    "https://app.norn.test/sign-in?return=%2Fv1%2Fpreviews%2Fauthorize",
	})
	h.machine()

	response := h.get("/", false)

	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("a signed-out browser answered %d, want 303", response.StatusCode)
	}

	if location := response.Header.Get("Location"); !strings.HasPrefix(
		location, "https://app.norn.test/sign-in",
	) {
		t.Fatalf(
			"a signed-out browser was sent to %q; it has to land in norn's own sign-in or it "+
				"can never come back with a session",
			location,
		)
	}
}

func TestAMachineThatIsNotConnectedSaysSoInsteadOfHanging(t *testing.T) {
	h := newHarness(t)
	h.answers(h.allowed())

	response := h.get("/", true)

	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("an offline machine answered %d, want 502", response.StatusCode)
	}

	if body := read(t, response); !strings.Contains(body, "offline") {
		t.Fatalf(
			"the page for an offline machine reads %q; a dead tunnel has to say the machine is "+
				"offline rather than look like the app is broken",
			body,
		)
	}
}

func TestAPathModeHostIsRefusedRatherThanRoutedToTheWrongPreview(t *testing.T) {
	h := newHarness(t)

	reply := h.allowed()
	reply.Mode = entity.PreviewByPath

	h.answers(reply)
	h.machine()

	response := h.get("/", true)

	if response.StatusCode != http.StatusNotImplemented {
		t.Fatalf(
			"a path-mode host answered %d, want 501; one address standing for every preview of "+
				"a run cannot pick between them",
			response.StatusCode,
		)
	}
}

func TestAHostOutsideThePreviewDomainIsRefusedWithoutAskingNorn(t *testing.T) {
	h := newHarness(t)

	request, err := http.NewRequest(http.MethodGet, h.server.URL+"/", nil)
	if err != nil {
		t.Fatalf("build the request: %v", err)
	}

	request.Host = "anything.example.com"

	response, err := h.client.Do(request)
	if err != nil {
		t.Fatalf("ask the gateway: %v", err)
	}

	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusNotFound {
		t.Fatalf(
			"a host under nobody's preview domain answered %d, want 404",
			response.StatusCode,
		)
	}
}

func TestAPreviewDomainThatCarriesAPortStillRoutes(t *testing.T) {
	h := newHarness(t)
	h.answers(h.allowed())
	h.machine()

	request, err := http.NewRequest(http.MethodGet, h.server.URL+"/", nil)
	if err != nil {
		t.Fatalf("build the request: %v", err)
	}

	request.Host = testHost() + ":8090"
	request.AddCookie(&http.Cookie{Name: entity.PreviewGrantCookie, Value: testGrant})

	response, err := h.client.Do(request)
	if err != nil {
		t.Fatalf("ask the gateway: %v", err)
	}

	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		t.Fatalf(
			"a host carrying an explicit port answered %d; a gateway that only matches the "+
				"default port cannot be run anywhere but behind a proxy on 443",
			response.StatusCode,
		)
	}

	if asked := h.asked(); asked != testHost()+":8090" {
		t.Fatalf(
			"norn was asked about %q; the host has to reach norn exactly as the browser wrote "+
				"it, because that is what the registration was stored under",
			asked,
		)
	}
}
