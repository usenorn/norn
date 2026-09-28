package telegrammessenger

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf16"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/telegram"
	"github.com/usenorn/norn/internal/repository"
)

const (
	mentionEntity   = "mention"
	unmodified      = "message is not modified"
	memberLeft      = "left"
	memberKicked    = "kicked"
	callbackRefused = "query is too old"
)

type httpMessenger struct {
	client *telegram.Client
}

func New(client *telegram.Client) repository.TelegramMessenger {
	return &httpMessenger{client: client}
}

func (r *httpMessenger) Identify(ctx context.Context, token string) (entity.TelegramIdentity, error) {
	bot, err := r.client.GetMe(ctx, token)
	if err != nil {
		return entity.TelegramIdentity{}, translate(err)
	}

	return entity.TelegramIdentity{BotUserID: bot.ID, Username: bot.Username, Name: bot.FirstName}, nil
}

func (r *httpMessenger) Register(ctx context.Context, token, url, secret string) error {
	return translate(r.client.SetWebhook(ctx, token, telegram.Webhook{
		URL:            url,
		SecretToken:    secret,
		AllowedUpdates: entity.TelegramAllowedUpdates,
	}))
}

func (r *httpMessenger) Unregister(ctx context.Context, token string) error {
	return translate(r.client.DeleteWebhook(ctx, token))
}

func (r *httpMessenger) Send(ctx context.Context, token string, message entity.TelegramOutgoing) (int64, error) {
	id, err := r.client.SendMessage(ctx, token, telegram.Outgoing{
		ChatID:   message.ChatID,
		Text:     message.Text,
		ReplyTo:  message.ReplyTo,
		Keyboard: keyboard(message.Options),
	})
	if err != nil {
		return 0, translateChat(err)
	}

	return id, nil
}

func (r *httpMessenger) Edit(ctx context.Context, token string, chatID, messageID int64, text string) error {
	err := r.client.EditMessageText(ctx, token, telegram.Edit{ChatID: chatID, MessageID: messageID, Text: text})

	var refused *telegram.Error
	if errors.As(err, &refused) && strings.Contains(refused.Description, unmodified) {
		return nil
	}

	return translateChat(err)
}

func (r *httpMessenger) AnswerCallback(ctx context.Context, token, callbackID, text string) error {
	err := r.client.AnswerCallbackQuery(ctx, token, callbackID, text)

	var refused *telegram.Error
	if errors.As(err, &refused) && strings.Contains(refused.Description, callbackRefused) {
		return nil
	}

	return translateChat(err)
}

func (r *httpMessenger) Typing(ctx context.Context, token string, chatID int64) error {
	return translateChat(r.client.SendChatAction(ctx, token, chatID, entity.TelegramTypingAction))
}

func (r *httpMessenger) Decode(payload []byte) (entity.TelegramIncoming, error) {
	update, err := telegram.Decode(payload)
	if err != nil {
		return entity.TelegramIncoming{}, err
	}

	incoming := entity.TelegramIncoming{UpdateID: update.UpdateID}

	if update.Message != nil {
		message := toMessage(*update.Message)
		incoming.Message = &message
	}

	if update.CallbackQuery != nil {
		callback := toCallback(*update.CallbackQuery)
		incoming.Callback = &callback
	}

	if update.MyChatMember != nil {
		incoming.Membership = &entity.TelegramMembership{
			ChatID:   update.MyChatMember.Chat.ID,
			ChatType: entity.TelegramChatType(update.MyChatMember.Chat.Type),
			Removed: update.MyChatMember.NewChatMember.Status == memberLeft ||
				update.MyChatMember.NewChatMember.Status == memberKicked,
		}
	}

	return incoming, nil
}

func toMessage(message telegram.Message) entity.TelegramMessage {
	decoded := entity.TelegramMessage{
		ChatID:     message.Chat.ID,
		ChatType:   entity.TelegramChatType(message.Chat.Type),
		ChatTitle:  message.Chat.Title,
		MessageID:  message.MessageID,
		Sender:     toSender(message.From, message.SenderChat != nil),
		Text:       message.Text,
		Mentions:   mentions(message.Text, message.Entities),
		MigratedTo: message.MigrateToChatID,
	}

	if reply := message.ReplyToMessage; reply != nil {
		decoded.ReplyToID = reply.MessageID
		if reply.From != nil {
			decoded.ReplyToSenderID = reply.From.ID
		}
	}

	return decoded
}

func toCallback(callback telegram.CallbackQuery) entity.TelegramCallback {
	decoded := entity.TelegramCallback{
		ID:     callback.ID,
		Sender: toSender(&callback.From, false),
		Data:   callback.Data,
	}

	if callback.Message != nil {
		decoded.ChatID = callback.Message.Chat.ID
		decoded.MessageID = callback.Message.MessageID
	}

	return decoded
}

func toSender(user *telegram.User, anonymous bool) entity.TelegramSender {
	if user == nil {
		return entity.TelegramSender{Anonymous: anonymous}
	}

	return entity.TelegramSender{ID: user.ID, IsBot: user.IsBot, Anonymous: anonymous, Username: user.Username}
}

func mentions(text string, entities []telegram.Entity) []string {
	units := utf16.Encode([]rune(text))

	var found []string

	for _, marked := range entities {
		end := marked.Offset + marked.Length
		if marked.Type != mentionEntity || marked.Offset < 0 || end > len(units) {
			continue
		}

		found = append(found, strings.TrimPrefix(string(utf16.Decode(units[marked.Offset:end])), "@"))
	}

	return found
}

func keyboard(options []string) [][]telegram.Button {
	if len(options) == 0 {
		return nil
	}

	rows := make([][]telegram.Button, 0, len(options))

	for index, option := range options {
		rows = append(rows, []telegram.Button{{Text: option, CallbackData: entity.TelegramOptionData(index)}})
	}

	return rows
}

func translate(err error) error {
	return translated(err, entity.ErrTelegramUnreachable)
}

func translateChat(err error) error {
	return translated(err, entity.ErrTelegramChatUnavailable)
}

func translated(err, refusedRequest error) error {
	if err == nil {
		return nil
	}

	var refused *telegram.Error
	if !errors.As(err, &refused) {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}

		return fmt.Errorf("%w: %w", entity.ErrTelegramUnreachable, err)
	}

	switch refused.Code {
	case http.StatusUnauthorized, http.StatusNotFound:
		return fmt.Errorf("%w: %w", entity.ErrTelegramTokenRejected, err)
	case http.StatusForbidden, http.StatusBadRequest:
		return fmt.Errorf("%w: %w", refusedRequest, err)
	default:
		return fmt.Errorf("%w: %w", entity.ErrTelegramUnreachable, err)
	}
}
