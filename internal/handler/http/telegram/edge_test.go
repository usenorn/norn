package telegram_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/telegram"
	telegramsvc "github.com/usenorn/norn/internal/service/telegrambot"
)

func deliver(t *testing.T, updates *telegramsvc.MockTelegramUpdates, path, secret, body string) int {
	t.Helper()

	router := chi.NewRouter()
	router.Post(telegram.UpdatePath, telegram.New(updates, config.Telegram{MaxUpdateBytes: 1024}).Deliver)

	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("X-Telegram-Bot-Api-Secret-Token", secret)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	return recorder.Code
}

func TestOnlyAForgedUpdateIsRefusedAndAStoreFailureIsRetried(t *testing.T) {
	cases := map[string]struct {
		cause  error
		status int
	}{
		"accepted":                    {nil, http.StatusOK},
		"a secret that did not match": {entity.ErrTelegramSecretInvalid, http.StatusUnauthorized},
		"a bot that was disconnected": {entity.ErrTelegramBotNotFound, http.StatusOK},
		"the store is down":           {errors.New("connection refused"), http.StatusInternalServerError},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			updates := telegramsvc.NewMockTelegramUpdates(gomock.NewController(t))
			botID := uuid.New()

			updates.EXPECT().Accept(gomock.Any(), botID, "s3cret", []byte(`{"update_id":1}`)).Return(tc.cause)

			status := deliver(t, updates, "/v1/telegram/bots/"+botID.String()+"/updates", "s3cret", `{"update_id":1}`)
			if status != tc.status {
				t.Fatalf("status = %d, want %d", status, tc.status)
			}
		})
	}
}

func TestAnUpdateForAPathThatNamesNoBotIsDropped(t *testing.T) {
	updates := telegramsvc.NewMockTelegramUpdates(gomock.NewController(t))

	if status := deliver(t, updates, "/v1/telegram/bots/not-a-bot/updates", "s3cret", `{}`); status != http.StatusOK {
		t.Fatalf("status = %d, want 200 so Telegram stops retrying", status)
	}
}

func TestAnOversizedUpdateIsDroppedRatherThanRetriedForADay(t *testing.T) {
	updates := telegramsvc.NewMockTelegramUpdates(gomock.NewController(t))

	status := deliver(t, updates, "/v1/telegram/bots/"+uuid.NewString()+"/updates", "s3cret", strings.Repeat("x", 2048))
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
}
