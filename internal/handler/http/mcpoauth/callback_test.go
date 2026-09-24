package mcpoauth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/mcpoauth"
	"github.com/usenorn/norn/internal/service"
	agentcapabilitysvc "github.com/usenorn/norn/internal/service/agentcapability"
)

func TestTheCallbackReturnsTheBrowserToWhereTheSignInStarted(t *testing.T) {
	cases := []struct {
		name     string
		query    string
		code     string
		returnTo string
		err      error
		want     string
	}{
		{"signed in", "?state=s&code=c", "c", "/acme/settings/agents/1?tab=capabilities", nil, "/acme/settings/agents/1?tab=capabilities&connection=connected"},
		{"refused by the person", "?state=s&error=access_denied", "", "/acme/settings/agent-library", entity.ErrAgentMCPOAuthRefused, "/acme/settings/agent-library?connection=refused"},
		{"replayed", "?state=s&code=c", "c", "", entity.ErrAgentMCPOAuthStateNotFound, "/?connection=expired"},
		{"an off-site return", "?state=s&code=c", "c", "//evil.example/", nil, "/?connection=connected"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			capabilities := agentcapabilitysvc.NewMockAgentCapabilities(ctrl)
			capabilities.EXPECT().
				CompleteMCPConnect(gomock.Any(), "s", tc.code, "https://norn.test/v1/agent-mcp/oauth/callback").
				Return(service.CompletedMCPConnect{WorkspaceID: uuid.New(), ReturnTo: tc.returnTo}, tc.err)

			callback := mcpoauth.NewCallback(capabilities, config.App{BaseURL: "https://norn.test/"})

			recorder := httptest.NewRecorder()
			callback.Handle(recorder, httptest.NewRequest(http.MethodGet, mcpoauth.CallbackPath+tc.query, nil))

			if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != tc.want {
				t.Errorf("redirect %d to %q, want 303 to %q", recorder.Code, recorder.Header().Get("Location"), tc.want)
			}
		})
	}
}
