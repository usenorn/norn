package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/middleware"
	"github.com/usenorn/norn/internal/pkg/identity"
)

type countingThrottle struct {
	keys []string
}

func (c *countingThrottle) Record(_ context.Context, key string) (int, error) {
	c.keys = append(c.keys, key)

	return 1, nil
}

func throttled(t *testing.T, actor entity.Actor) (int, []string) {
	t.Helper()

	throttle := &countingThrottle{}
	handler := middleware.MCPRateLimit(throttle, config.MCP{RequestsPerWindow: 10, RateWindow: time.Minute})(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }),
	)

	request := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	request = request.WithContext(identity.WithActor(request.Context(), actor))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	return recorder.Code, throttle.keys
}

func TestARunnersSessionReachesNornsToolsAndIsThrottledAsThatRunner(t *testing.T) {
	runnerID := uuid.New()

	code, keys := throttled(t, entity.Actor{
		Kind: entity.ActorKindAgent, AccountID: uuid.New(), RunnerID: &runnerID,
	})
	if code != http.StatusOK {
		t.Fatalf(
			"a runner's session was answered %d at /mcp; every run needs norn's tools before its "+
				"coding agent can start",
			code,
		)
	}

	if len(keys) != 1 || keys[0] != "mcp-runner:"+runnerID.String() {
		t.Fatalf("a runner was throttled under %v", keys)
	}
}

func TestAnAPITokenIsThrottledAsThatToken(t *testing.T) {
	tokenID := uuid.New()

	code, keys := throttled(t, entity.Actor{
		Kind: entity.ActorKindAgent, AccountID: uuid.New(), TokenID: &tokenID,
	})
	if code != http.StatusOK || len(keys) != 1 || keys[0] != "mcp-token:"+tokenID.String() {
		t.Fatalf("an api token came back %d under %v", code, keys)
	}
}
