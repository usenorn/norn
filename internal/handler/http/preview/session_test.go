package preview_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
)

func (h *harness) hands(reply entity.PreviewGrantReply, err error) {
	h.t.Helper()

	h.norn.EXPECT().
		Session(gomock.Any(), "nga_token", gomock.Any()).
		Return(reply, err).
		AnyTimes()
}

func (h *harness) redeems(reply entity.PreviewGrantReply, err error) {
	h.t.Helper()

	h.norn.EXPECT().
		Redeem(gomock.Any(), "nga_token", gomock.Any(), gomock.Any(), gomock.Any()).
		Return(reply, err).
		AnyTimes()
}

func granted() entity.PreviewGrantReply {
	return entity.PreviewGrantReply{
		Grant:     testGrant,
		Cookie:    entity.PreviewGrantCookie,
		Path:      "/app",
		ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
	}
}

func cookieOn(response *http.Response) *http.Cookie {
	for _, cookie := range response.Cookies() {
		if cookie.Name == entity.PreviewGrantCookie {
			return cookie
		}
	}

	return nil
}

func TestComingBackFromSignInLeavesTheBrowserHoldingItsPreviewSession(t *testing.T) {
	h := newHarness(t)
	h.hands(granted(), nil)

	response := h.get(entity.PreviewSessionPath+"?ticket=one-shot", false)

	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("the hand-off answered %d, want 303", response.StatusCode)
	}

	cookie := cookieOn(response)
	if cookie == nil {
		t.Fatal(
			"the hand-off set no preview cookie, so the browser would be sent straight back " +
				"into sign-in and loop",
		)
	}

	if cookie.Value != testGrant || !cookie.HttpOnly {
		t.Fatalf(
			"the preview cookie is %q httpOnly=%t; a session a page's own scripts can read is "+
				"a session the code norn did not write can take",
			cookie.Value, cookie.HttpOnly,
		)
	}

	if location := response.Header.Get("Location"); location != "/app" {
		t.Fatalf("the hand-off landed on %q, want the preview's own path", location)
	}
}

func TestAHandOffOnlyWorksOnce(t *testing.T) {
	h := newHarness(t)
	h.hands(entity.PreviewGrantReply{}, entity.ErrPreviewShareExpired)

	response := h.get(entity.PreviewSessionPath+"?ticket=already-spent", false)

	if response.StatusCode != http.StatusGone {
		t.Fatalf(
			"a spent hand-off answered %d, want 410; a ticket that works twice is a session "+
				"anybody who saw the url can mint",
			response.StatusCode,
		)
	}

	if cookieOn(response) != nil {
		t.Fatal("a refused hand-off still set a preview cookie")
	}
}

func TestAHandOffNeverLandsOnSomebodyElsesSite(t *testing.T) {
	h := newHarness(t)
	h.hands(granted(), nil)

	response := h.get(
		entity.PreviewSessionPath+"?ticket=one-shot&return="+
			url.QueryEscape("//evil.example.com/"),
		false,
	)

	if location := response.Header.Get("Location"); strings.HasPrefix(location, "//") {
		t.Fatalf(
			"the hand-off followed a return of %q off this host; an open redirect on the "+
				"preview origin is one a share link could aim anywhere",
			location,
		)
	}
}

func TestAShareLinkAsksForItsPasscodeWithoutSpendingAGuess(t *testing.T) {
	h := newHarness(t)
	h.redeems(entity.PreviewGrantReply{}, entity.ErrPreviewSharePasscodeNeeded)

	response := h.get(entity.PreviewSharePath+"nsl_token", false)

	if response.StatusCode != http.StatusOK {
		t.Fatalf("a link needing a passcode answered %d, want the form", response.StatusCode)
	}

	body := read(t, response)

	if !strings.Contains(body, `name="passcode"`) {
		t.Fatalf("the page for a passcode link carries no passcode field: %q", body)
	}
}

func TestARightPasscodeLetsAShareLinkIn(t *testing.T) {
	h := newHarness(t)
	h.redeems(granted(), nil)

	form := url.Values{"passcode": {"open-sesame"}}

	request, err := http.NewRequest(
		http.MethodPost,
		h.server.URL+entity.PreviewSharePath+"nsl_token",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		t.Fatalf("build the request: %v", err)
	}

	request.Host = testHost()
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := h.client.Do(request)
	if err != nil {
		t.Fatalf("submit the passcode: %v", err)
	}

	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusSeeOther || cookieOn(response) == nil {
		t.Fatalf(
			"the right passcode answered %d with cookie %v; a share link that never hands out "+
				"a session is a link that does not work",
			response.StatusCode, cookieOn(response),
		)
	}
}

func TestAWithdrawnShareLinkSaysSoRatherThanAskingAgain(t *testing.T) {
	h := newHarness(t)
	h.redeems(entity.PreviewGrantReply{}, entity.ErrPreviewShareExpired)

	response := h.get(entity.PreviewSharePath+"nsl_token", false)

	if response.StatusCode != http.StatusGone {
		t.Fatalf("a withdrawn link answered %d, want 410", response.StatusCode)
	}
}

func TestTooManyWrongPasscodesStopsTheLinkRatherThanTheViewer(t *testing.T) {
	h := newHarness(t)
	h.redeems(entity.PreviewGrantReply{}, entity.ErrPreviewShareGuessed)

	response := h.get(entity.PreviewSharePath+"nsl_token", false)

	if response.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("a link that has been guessed at answered %d, want 429", response.StatusCode)
	}
}
