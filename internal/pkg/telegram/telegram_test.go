package telegram_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/pkg/telegram"
)

const token = "123456:AAHsecretsecretsecretsecretsecret"

func build(endpoint string, limit int64) *telegram.Client {
	return telegram.New(config.Telegram{
		Endpoint:        endpoint,
		RequestTimeout:  2 * time.Second,
		DialTimeout:     time.Second,
		MaxResponseSize: limit,
	})
}

func answering(t *testing.T, handler http.HandlerFunc) *telegram.Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return build(server.URL+"/", 1<<20)
}

func TestGetMeCallsTheBotMethodPathWithTheToken(t *testing.T) {
	var path string

	client := answering(t, func(writer http.ResponseWriter, request *http.Request) {
		path = request.URL.Path
		_, _ = writer.Write([]byte(`{"ok":true,"result":{"id":42,"is_bot":true,"first_name":"Ada","username":"ada_bot"}}`))
	})

	bot, err := client.GetMe(context.Background(), token)
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}

	if path != "/bot"+token+"/getMe" {
		t.Errorf("path = %q", path)
	}

	if bot.ID != 42 || bot.Username != "ada_bot" || bot.FirstName != "Ada" {
		t.Errorf("bot = %+v", bot)
	}
}

func TestSendMessageSendsHTMLWithoutPreviewAndTheKeyboard(t *testing.T) {
	var sent map[string]any

	client := answering(t, func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		_ = json.Unmarshal(body, &sent)
		_, _ = writer.Write([]byte(`{"ok":true,"result":{"message_id":77}}`))
	})

	id, err := client.SendMessage(context.Background(), token, telegram.Outgoing{
		ChatID:   -1001234567890123,
		Text:     "<b>Ship it?</b>",
		ReplyTo:  5,
		Keyboard: [][]telegram.Button{{{Text: "Yes", CallbackData: "a:0"}}},
	})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	if id != 77 {
		t.Errorf("message id = %d, want 77", id)
	}

	if sent["parse_mode"] != "HTML" {
		t.Errorf("parse_mode = %v", sent["parse_mode"])
	}

	if chat, _ := sent["chat_id"].(float64); int64(chat) != -1001234567890123 {
		t.Errorf("chat_id = %v", sent["chat_id"])
	}

	preview, _ := sent["link_preview_options"].(map[string]any)
	if preview["is_disabled"] != true {
		t.Errorf("link_preview_options = %v", sent["link_preview_options"])
	}

	markup, _ := sent["reply_markup"].(map[string]any)
	rows, _ := markup["inline_keyboard"].([]any)
	if len(rows) != 1 {
		t.Fatalf("inline_keyboard = %v", markup)
	}

	reply, _ := sent["reply_parameters"].(map[string]any)
	if reply["message_id"] != float64(5) {
		t.Errorf("reply_parameters = %v", sent["reply_parameters"])
	}
}

func TestAnUnsuccessfulAnswerCarriesItsCodeAndRetryAfter(t *testing.T) {
	client := answering(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusTooManyRequests)
		_, _ = writer.Write([]byte(`{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 7","parameters":{"retry_after":7}}`))
	})

	err := client.DeleteWebhook(context.Background(), token)

	var refused *telegram.Error
	if !errors.As(err, &refused) {
		t.Fatalf("err = %v, want *telegram.Error", err)
	}

	if refused.Code != http.StatusTooManyRequests || refused.RetryAfter != 7*time.Second {
		t.Errorf("refused = %+v", refused)
	}
}

func TestAFailedConnectionNeverRepeatsTheToken(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	endpoint := server.URL
	server.Close()

	_, err := build(endpoint, 1<<20).GetMe(context.Background(), token)
	if err == nil {
		t.Fatal("GetMe against a closed server succeeded")
	}

	if strings.Contains(err.Error(), token) || strings.Contains(err.Error(), "AAHsecret") {
		t.Fatalf("error leaks the token: %v", err)
	}
}

func TestAnAnswerLargerThanTheLimitIsRefused(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"ok":true,"result":{"id":1,"first_name":"` + strings.Repeat("x", 2048) + `"}}`))
	}))
	t.Cleanup(server.Close)

	_, err := build(server.URL, 1024).GetMe(context.Background(), token)
	if !errors.Is(err, telegram.ErrResponseTooLarge) {
		t.Fatalf("err = %v, want ErrResponseTooLarge", err)
	}
}

func TestDecodeKeepsSupergroupIdentifiersWhole(t *testing.T) {
	update, err := telegram.Decode([]byte(`{"update_id":9007199254740,"message":{"message_id":3,
		"from":{"id":7000000001,"is_bot":false,"username":"rae"},
		"chat":{"id":-1009876543210987,"type":"supergroup","title":"Release"},
		"text":"/start@ada_bot abc","entities":[{"type":"bot_command","offset":0,"length":14}]}}`))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if update.UpdateID != 9007199254740 || update.Message.Chat.ID != -1009876543210987 ||
		update.Message.From.ID != 7000000001 {
		t.Errorf("update = %+v", update)
	}
}
