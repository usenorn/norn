package dashboard_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	telegrambotsvc "github.com/usenorn/norn/internal/service/telegrambot"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

type telegramHarness struct {
	bots      *telegrambotsvc.MockTelegramBots
	routes    http.Handler
	workspace uuid.UUID
	agent     uuid.UUID
}

func newTelegramHarness(t *testing.T) *telegramHarness {
	t.Helper()

	ctrl := gomock.NewController(t)
	h := &telegramHarness{bots: telegrambotsvc.NewMockTelegramBots(ctrl), workspace: uuid.New(), agent: uuid.New()}
	h.routes = newEdge(ctrl, edgeServices{telegramBots: h.bots})

	return h
}

func (h *telegramHarness) call(t *testing.T, method, suffix string, payload any) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			t.Fatalf("encode request: %v", err)
		}
	}

	request := httptest.NewRequest(
		method, "/workspaces/"+h.workspace.String()+"/agents/"+h.agent.String()+"/telegram"+suffix, &body,
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	h.routes.ServeHTTP(recorder, request)

	return recorder
}

func TestTheBotTokenNeverLeavesTheBackend(t *testing.T) {
	h := newTelegramHarness(t)
	token := "7000000001:AAHhandlerhandlerhandlerhandler9876"

	h.bots.EXPECT().Connect(gomock.Any(), h.workspace, h.agent, token).Return(service.TelegramBotView{
		Bot:    entity.TelegramBot{ID: uuid.New(), Username: "ada_bot", Name: "Ada", TokenHint: "9876"},
		Hosted: true,
		Groups: []entity.TelegramGroup{{ID: uuid.New(), Title: "Release"}},
	}, nil)

	recorder := h.call(t, http.MethodPut, "", api.ConnectAgentTelegramRequest{Token: token})
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", recorder.Code, recorder.Body)
	}

	got := decodeBody[api.AgentTelegramBot](t, recorder)
	if got.Username != "ada_bot" || got.TokenHint != "9876" || len(got.Groups) != 1 || got.Members == nil {
		t.Errorf("bot = %+v", got)
	}

	if strings.Contains(recorder.Body.String(), "AAHhandler") {
		t.Errorf("the response carries the token: %s", recorder.Body)
	}
}

func TestATelegramRefusalCarriesItsReason(t *testing.T) {
	cases := map[string]struct {
		cause  error
		status int
		code   string
	}{
		"a revoked token":           {entity.ErrTelegramTokenRejected, http.StatusUnprocessableEntity, "token_rejected"},
		"a bot another agent uses":  {entity.ErrTelegramBotTaken, http.StatusUnprocessableEntity, "bot_taken"},
		"an instance without https": {entity.ErrTelegramOriginInsecure, http.StatusUnprocessableEntity, "origin_insecure"},
		"telegram is down":          {entity.ErrTelegramUnreachable, http.StatusUnprocessableEntity, "unreachable"},
		"no encryption key":         {entity.ErrTelegramEncryptionKeyMissing, http.StatusServiceUnavailable, "telegram_sealing_unavailable"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := newTelegramHarness(t)
			h.bots.EXPECT().Connect(gomock.Any(), h.workspace, h.agent, gomock.Any()).Return(service.TelegramBotView{}, tc.cause)

			recorder := h.call(t, http.MethodPut, "", api.ConnectAgentTelegramRequest{Token: "x"})
			if recorder.Code != tc.status {
				t.Fatalf("status = %d, want %d; body %s", recorder.Code, tc.status, recorder.Body)
			}

			var problem struct {
				Code string `json:"code"`
			}

			if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil || problem.Code != tc.code {
				t.Fatalf("code = %q (%v), want %q", problem.Code, err, tc.code)
			}
		})
	}
}

func TestAnAgentWithoutABotReadsAsNotFound(t *testing.T) {
	h := newTelegramHarness(t)
	h.bots.EXPECT().Get(gomock.Any(), h.workspace, h.agent).Return(service.TelegramBotView{}, entity.ErrTelegramBotNotFound)

	if recorder := h.call(t, http.MethodGet, "", nil); recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
}

func TestALinkIsIssuedForThePurposeAsked(t *testing.T) {
	h := newTelegramHarness(t)
	h.bots.EXPECT().
		IssueLink(gomock.Any(), h.workspace, h.agent, entity.TelegramLinkGroup).
		Return(service.TelegramLinkInvite{URL: "https://t.me/ada_bot?startgroup=c0de"}, nil)

	recorder := h.call(t, http.MethodPost, "/links", api.CreateAgentTelegramLinkRequest{Purpose: api.TelegramLinkPurposeGroup})
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", recorder.Code, recorder.Body)
	}

	if got := decodeBody[api.TelegramLinkInvite](t, recorder); got.Url != "https://t.me/ada_bot?startgroup=c0de" {
		t.Errorf("invite = %+v", got)
	}
}
