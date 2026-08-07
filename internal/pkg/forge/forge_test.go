package forge_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/forge"
)

func settings() config.SourceControl {
	return config.SourceControl{
		RequestTimeout:  5 * time.Second,
		DialTimeout:     time.Second,
		MaxResponseSize: 1 << 16,
		// The test server answers on loopback, which the guard refuses by design, so the
		// tests punch the same hole an operator running a forge on a private network does.
		AllowedDestinations: []string{"127.0.0.0/8", "::1/128"},
	}
}

func client(t *testing.T) *forge.Client {
	t.Helper()

	built, err := forge.New(settings())
	if err != nil {
		t.Fatalf("forge.New: %v", err)
	}

	return built
}

func call(t *testing.T, handler http.HandlerFunc) (forge.Response, error) {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return client(t).Do(context.Background(), forge.Request{
		Provider: entity.SCMProviderGitHub,
		Method:   http.MethodGet,
		URL:      server.URL,
	})
}

func TestAForbiddenAnswerIsReadAsAnExhaustedQuotaWheneverItSaysSo(t *testing.T) {
	cases := []struct {
		name    string
		headers map[string]string
		limited bool
	}{
		{
			name:    "a primary limit reports nothing remaining",
			headers: map[string]string{"X-RateLimit-Remaining": "0", "X-RateLimit-Reset": ""},
			limited: true,
		},
		{
			name:    "a secondary limit sends only a retry-after",
			headers: map[string]string{"Retry-After": "60"},
			limited: true,
		},
		{
			name:    "gitlab spells the same header without a prefix",
			headers: map[string]string{"RateLimit-Remaining": "0"},
			limited: true,
		},
		{
			name:    "a plain refusal carries neither signal",
			headers: map[string]string{},
			limited: false,
		},
		{
			name:    "quota left means the refusal is about permission",
			headers: map[string]string{"X-RateLimit-Remaining": "4999"},
			limited: false,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := call(t, func(w http.ResponseWriter, _ *http.Request) {
				for name, value := range testCase.headers {
					if value != "" {
						w.Header().Set(name, value)
					}
				}

				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"message":"Forbidden"}`))
			})

			var limited entity.SCMRateLimitedError
			var rejected entity.SCMCredentialsRejectedError

			switch {
			case testCase.limited && !errors.As(err, &limited):
				t.Fatalf(
					"a rate-limited 403 was classified as %T. Treated as a refusal it breaks the "+
						"connection, so a busy workspace loses its integration every hour",
					err,
				)
			case !testCase.limited && !errors.As(err, &rejected):
				t.Fatalf(
					"a permission 403 was classified as %T. Treated as a rate limit it is retried "+
						"forever and the person is never told to fix the token's scopes",
					err,
				)
			}
		})
	}
}

func TestARateLimitCarriesHowLongToWaitHoweverTheForgeSaysIt(t *testing.T) {
	reset := strconv.FormatInt(time.Now().Add(90*time.Second).Unix(), 10)

	cases := []struct {
		name    string
		headers map[string]string
		atLeast time.Duration
		atMost  time.Duration
	}{
		{
			name:    "retry-after in seconds",
			headers: map[string]string{"Retry-After": "60"},
			atLeast: 60 * time.Second,
			atMost:  60 * time.Second,
		},
		{
			name:    "a reset given as a second epoch",
			headers: map[string]string{"X-RateLimit-Reset": reset},
			atLeast: 80 * time.Second,
			atMost:  90 * time.Second,
		},
		{
			name:    "a reset already in the past asks for no wait at all",
			headers: map[string]string{"X-RateLimit-Reset": "1"},
			atLeast: 0,
			atMost:  0,
		},
		{
			name:    "a reset far enough out to be a mistake is bounded",
			headers: map[string]string{"X-RateLimit-Reset": "99999999999"},
			atLeast: 24 * time.Hour,
			atMost:  24 * time.Hour,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := call(t, func(w http.ResponseWriter, _ *http.Request) {
				for name, value := range testCase.headers {
					w.Header().Set(name, value)
				}

				w.WriteHeader(http.StatusTooManyRequests)
			})

			var limited entity.SCMRateLimitedError
			if !errors.As(err, &limited) {
				t.Fatalf("429 was classified as %T, want a rate limit", err)
			}

			if limited.RetryAfter < testCase.atLeast || limited.RetryAfter > testCase.atMost {
				t.Fatalf(
					"RetryAfter = %s, want between %s and %s",
					limited.RetryAfter, testCase.atLeast, testCase.atMost,
				)
			}
		})
	}
}

func TestAnOversizedAnswerIsRefusedRatherThanCutShort(t *testing.T) {
	_, err := call(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", (1<<16)+1)))
	})

	if !errors.Is(err, forge.ErrResponseTooLarge) {
		t.Fatalf(
			"reading past the cap gave %v, want a refusal. A body cut off at the limit comes back "+
				"as broken json at an offset that names neither the limit nor the call that hit it",
			err,
		)
	}
}

func TestAnAnswerInsideTheCapIsHandedBackWhole(t *testing.T) {
	body := strings.Repeat("x", 1<<16)

	answer, err := call(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	if len(answer.Body) != len(body) {
		t.Fatalf("read %d bytes of a %d-byte answer; the cap is inclusive", len(answer.Body), len(body))
	}
}

func TestAnUnauthorisedAnswerIsACredentialProblemAndAServerErrorIsNot(t *testing.T) {
	_, err := call(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	var rejected entity.SCMCredentialsRejectedError
	if !errors.As(err, &rejected) {
		t.Fatalf("401 was classified as %T, want a rejected credential", err)
	}

	_, err = call(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})

	var unavailable entity.SCMUnavailableError
	if !errors.As(err, &unavailable) {
		t.Fatalf(
			"502 was classified as %T. A forge outage is not a credential problem, and breaking "+
				"the connection over one would need a person to repair something that was never broken",
			err,
		)
	}
}

func TestAnAnswerTheAdapterHasToReadForItselfComesBackRatherThanAsAnError(t *testing.T) {
	answer, err := call(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found"}`))
	})
	if err != nil {
		t.Fatalf("Do returned %v; a 404 means different things per call and belongs to the adapter", err)
	}

	if answer.Status != http.StatusNotFound || len(answer.Body) == 0 {
		t.Fatalf("Do = %d with %d bytes, want the answer intact", answer.Status, len(answer.Body))
	}
}

func TestAHostThatResolvesInsideTheNetworkIsRefused(t *testing.T) {
	guarded, err := forge.New(config.SourceControl{
		RequestTimeout:  time.Second,
		DialTimeout:     time.Second,
		MaxResponseSize: 1 << 16,
	})
	if err != nil {
		t.Fatalf("forge.New: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	_, err = guarded.Do(context.Background(), forge.Request{
		Provider: entity.SCMProviderGitLab,
		Method:   http.MethodGet,
		URL:      server.URL,
	})

	var refused entity.SCMDestinationRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf(
			"a connection naming a loopback host was allowed, or was reported as %T. A base url "+
				"is supplied by whoever connects, so it is exactly the input the destination guard "+
				"exists for — and being refused by our own guard is not the forge saying no",
			err,
		)
	}

	var unreachable entity.SCMRepositoryUnreachableError
	if errors.As(err, &unreachable) {
		t.Fatal(
			"our own dial guard was reported as an unreachable repository. Those read the same " +
				"on screen, so somebody told to check the token would rotate a credential that " +
				"never left this instance",
		)
	}
}

func TestPaginationIsReadOutOfTheLinkHeaderTheForgeSends(t *testing.T) {
	answer, err := call(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(
			"Link",
			`<https://api.example/repos?page=1>; rel="prev", `+
				`<https://api.example/repos?page=3>; rel="next", `+
				`<https://api.example/repos?page=9>; rel="last"`,
		)
		w.WriteHeader(http.StatusOK)
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	if got := answer.Link("next"); got != "https://api.example/repos?page=3" {
		t.Fatalf("Link(next) = %q, want the next page", got)
	}

	if got := answer.Link("first"); got != "" {
		t.Fatalf("Link(first) = %q, want empty when the forge did not send that relation", got)
	}
}
