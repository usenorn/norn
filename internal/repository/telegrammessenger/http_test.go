package telegrammessenger_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/telegram"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/repository/telegrammessenger"
)

const token = "123456:AAHsecretsecretsecretsecretsecret"

func answering(t *testing.T, status int, body string) repository.TelegramMessenger {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return telegrammessenger.New(telegram.New(config.Telegram{
		Endpoint:        server.URL,
		RequestTimeout:  2 * time.Second,
		DialTimeout:     time.Second,
		MaxResponseSize: 1 << 20,
	}))
}

func TestARevokedTokenIsRejected(t *testing.T) {
	messenger := answering(t, http.StatusUnauthorized, `{"ok":false,"error_code":401,"description":"Unauthorized"}`)

	_, err := messenger.Identify(context.Background(), token)
	if !errors.Is(err, entity.ErrTelegramTokenRejected) {
		t.Fatalf("err = %v, want ErrTelegramTokenRejected", err)
	}
}

func TestABlockedChatIsUnavailableRatherThanUnreachable(t *testing.T) {
	messenger := answering(t, http.StatusForbidden, `{"ok":false,"error_code":403,"description":"Forbidden: bot was blocked by the user"}`)

	_, err := messenger.Send(context.Background(), token, entity.TelegramOutgoing{ChatID: 1, Text: "hi"})
	if !errors.Is(err, entity.ErrTelegramChatUnavailable) {
		t.Fatalf("err = %v, want ErrTelegramChatUnavailable", err)
	}
}

func TestEditingToTheSameTextIsNotAFailure(t *testing.T) {
	messenger := answering(t, http.StatusBadRequest,
		`{"ok":false,"error_code":400,"description":"Bad Request: message is not modified: specified new message content and reply markup are exactly the same"}`)

	if err := messenger.Edit(context.Background(), token, 1, 2, "same"); err != nil {
		t.Fatalf("Edit: %v", err)
	}
}

func TestARateLimitIsTransient(t *testing.T) {
	messenger := answering(t, http.StatusTooManyRequests, `{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":3}}`)

	_, err := messenger.Send(context.Background(), token, entity.TelegramOutgoing{ChatID: 1, Text: "hi"})
	if !errors.Is(err, entity.ErrTelegramUnreachable) {
		t.Fatalf("err = %v, want ErrTelegramUnreachable", err)
	}
}

func TestDecodeReadsMentionsByUTF16Offsets(t *testing.T) {
	messenger := telegrammessenger.New(nil)

	incoming, err := messenger.Decode([]byte(`{"update_id":5,"message":{"message_id":9,
		"from":{"id":7,"is_bot":false,"username":"rae"},
		"chat":{"id":-100123,"type":"supergroup","title":"Release"},
		"text":"🚀 @ada_bot status?",
		"entities":[{"type":"mention","offset":3,"length":8}],
		"reply_to_message":{"message_id":4,"from":{"id":42,"is_bot":true},"chat":{"id":-100123,"type":"supergroup"}}}}`))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	message := incoming.Message
	if message == nil {
		t.Fatal("no message decoded")
	}

	if !slices.Equal(message.Mentions, []string{"ada_bot"}) {
		t.Errorf("mentions = %q", message.Mentions)
	}

	if message.ChatType != entity.TelegramChatSupergroup || message.ReplyToID != 4 || message.ReplyToSenderID != 42 {
		t.Errorf("message = %+v", message)
	}

	if !message.Sender.Person() {
		t.Errorf("sender = %+v, want a person", message.Sender)
	}
}

func TestDecodeMarksAMessageSentAsTheChatAnonymous(t *testing.T) {
	incoming, err := telegrammessenger.New(nil).Decode([]byte(`{"update_id":6,"message":{"message_id":1,
		"from":{"id":1087968824,"is_bot":true,"username":"GroupAnonymousBot"},
		"sender_chat":{"id":-100123,"type":"supergroup"},
		"chat":{"id":-100123,"type":"supergroup"},"text":"@ada_bot hi"}}`))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if incoming.Message.Sender.Person() {
		t.Error("an anonymous admin was treated as a person")
	}
}

func TestDecodeSeesTheBotRemovedFromAGroup(t *testing.T) {
	incoming, err := telegrammessenger.New(nil).Decode([]byte(`{"update_id":7,"my_chat_member":{
		"chat":{"id":-100123,"type":"supergroup"},"from":{"id":7,"is_bot":false},
		"new_chat_member":{"status":"kicked"}}}`))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if incoming.Membership == nil || !incoming.Membership.Removed || incoming.Membership.ChatID != -100123 {
		t.Errorf("membership = %+v", incoming.Membership)
	}
}
