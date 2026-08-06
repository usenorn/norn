package lineargraph_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/lineargraph"
)

func answering(t *testing.T, handler http.HandlerFunc) (*lineargraph.Client, *httptest.Server) {
	t.Helper()

	source := httptest.NewServer(handler)
	t.Cleanup(source.Close)

	return lineargraph.New(config.Linear{
		Endpoint:        source.URL,
		RequestTimeout:  5 * time.Second,
		MaxResponseSize: 1 << 20,
		PageSize:        100,
	}), source
}

func TestAPageOfDataComesBackAsItselfWithTheKeyAttached(t *testing.T) {
	var offered string

	client, _ := answering(t, func(writer http.ResponseWriter, request *http.Request) {
		offered = request.Header.Get("Authorization")

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":{"issues":{"nodes":[{"id":"abc"}]}}}`))
	})

	data, err := client.Query(context.Background(), "lin_api_key", "query{issues{id}}", nil, entity.ImportIssue)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	if !strings.Contains(string(data), `"abc"`) {
		t.Errorf("data = %s, want the node the source sent", data)
	}

	if offered != "lin_api_key" {
		t.Errorf(
			"the source was called with authorization %q. Linear takes a personal key as the bare "+
				"header value rather than a bearer token, and a mis-shaped one reads as anonymous.",
			offered,
		)
	}
}

func TestBeingThrottledParksRatherThanFails(t *testing.T) {
	reset := time.Now().Add(90 * time.Second).Unix()

	client, _ := answering(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("X-RateLimit-Requests-Reset", strconv.FormatInt(reset, 10))
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"errors":[{"message":"slow down","extensions":{"code":"RATELIMITED"}}]}`))
	})

	_, err := client.Query(context.Background(), "k", "query{}", nil, entity.ImportIssue)

	var limited entity.ImportRateLimitedError

	if !errors.As(err, &limited) {
		t.Fatalf(
			"Query returned %v, want a rate-limit error. A throttled source must park the cursor "+
				"and resume; anything else spends the run's attempts and abandons the import.",
			err,
		)
	}

	if limited.RetryAfter < 30*time.Second || limited.RetryAfter > 2*time.Minute {
		t.Errorf(
			"retry after = %s, want roughly the ninety seconds the source asked for. Ignoring the "+
				"header means either hammering a source that just refused us or parking far longer "+
				"than it wanted.",
			limited.RetryAfter,
		)
	}
}

func TestAMillisecondResetIsNotReadAsSeconds(t *testing.T) {
	reset := time.Now().Add(60 * time.Second).UnixMilli()

	client, _ := answering(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("X-RateLimit-Requests-Reset", strconv.FormatInt(reset, 10))
		writer.WriteHeader(http.StatusTooManyRequests)
		_, _ = writer.Write([]byte(`{}`))
	})

	_, err := client.Query(context.Background(), "k", "query{}", nil, entity.ImportIssue)

	var limited entity.ImportRateLimitedError

	if !errors.As(err, &limited) {
		t.Fatalf("Query returned %v, want a rate-limit error", err)
	}

	if limited.RetryAfter > 5*time.Minute {
		t.Fatalf(
			"retry after = %s. A millisecond epoch read as seconds parks the run somewhere around "+
				"the year fifty thousand, and nothing ever wakes it.",
			limited.RetryAfter,
		)
	}
}

func TestARejectedKeyStopsTheRunRatherThanRetrying(t *testing.T) {
	for _, refusal := range []struct {
		name    string
		respond http.HandlerFunc
	}{
		{
			"the transport says no",
			func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(http.StatusUnauthorized)
				_, _ = writer.Write([]byte(`{"errors":[{"message":"invalid api key"}]}`))
			},
		},
		{
			"the body says no",
			func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = writer.Write([]byte(
					`{"errors":[{"message":"not permitted","extensions":{"code":"AUTHENTICATION_ERROR"}}]}`))
			},
		},
	} {
		t.Run(refusal.name, func(t *testing.T) {
			client, _ := answering(t, refusal.respond)

			_, err := client.Query(context.Background(), "k", "query{}", nil, entity.ImportTeam)

			var refused entity.ImportSourceRefusedError

			if !errors.As(err, &refused) {
				t.Fatalf(
					"Query returned %v, want a refusal. A key the source has rejected will be "+
						"rejected the same way twenty more times, so retrying only delays telling "+
						"the person who can fix it.",
					err,
				)
			}
		})
	}
}

func TestAnAnswerTooBigToReadIsNamedRatherThanTruncated(t *testing.T) {
	client, _ := answering(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(writer, `{"data":{"padding":%q}}`, strings.Repeat("x", 2<<20))
	})

	_, err := client.Query(context.Background(), "k", "query{}", nil, entity.ImportIssue)

	if err == nil {
		t.Fatal("an oversized answer was accepted")
	}

	if !errors.Is(err, lineargraph.ErrResponseTooLarge) {
		t.Fatalf(
			"Query returned %v, want something that names the size. The webhook client caps a body "+
				"by silently ending the reader, which for JSON surfaces later as an unexpected end "+
				"of input and sends whoever reads the log looking at the wrong thing entirely.",
			err,
		)
	}
}

func TestASourceHavingAnOutageIsRetriedRatherThanGivenUpOn(t *testing.T) {
	client, _ := answering(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadGateway)
		_, _ = writer.Write([]byte("upstream is down"))
	})

	_, err := client.Query(context.Background(), "k", "query{}", nil, entity.ImportComment)

	var unavailable entity.ImportSourceUnavailableError

	if !errors.As(err, &unavailable) {
		t.Fatalf(
			"Query returned %v, want an unavailability. An outage is temporary and the framework "+
				"will come back to it; classifying it as a refusal would abandon the run over "+
				"something that fixes itself.",
			err,
		)
	}

	if !strings.Contains(unavailable.Reason, "upstream is down") {
		t.Errorf("reason = %q, want what the source actually said", unavailable.Reason)
	}
}

func TestSomethingThatIsNotGraphQLIsReportedAsSuch(t *testing.T) {
	client, _ := answering(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("<html>you are behind a captive portal</html>"))
	})

	_, err := client.Query(context.Background(), "k", "query{}", nil, entity.ImportTeam)

	var unavailable entity.ImportSourceUnavailableError

	if !errors.As(err, &unavailable) {
		t.Fatalf("Query returned %v, want an unavailability naming the shape of the answer", err)
	}
}
