package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/usenorn/norn/internal/config"
)

const (
	methodGetMe               = "getMe"
	methodSetWebhook          = "setWebhook"
	methodDeleteWebhook       = "deleteWebhook"
	methodSendMessage         = "sendMessage"
	methodEditMessageText     = "editMessageText"
	methodAnswerCallbackQuery = "answerCallbackQuery"
	methodSendChatAction      = "sendChatAction"
	parseModeHTML             = "HTML"
	descriptionLimit          = 256
)

var ErrResponseTooLarge = errors.New("telegram answered with more than this instance will read")

type Client struct {
	http     *http.Client
	endpoint string
	limit    int64
}

func New(cfg config.Telegram) *Client {
	dialer := &net.Dialer{Timeout: cfg.DialTimeout}

	return &Client{
		http: &http.Client{
			Timeout: cfg.RequestTimeout,
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				DialContext:           dialer.DialContext,
				TLSHandshakeTimeout:   cfg.DialTimeout,
				ResponseHeaderTimeout: cfg.RequestTimeout,
				ForceAttemptHTTP2:     true,
			},
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		endpoint: strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/"),
		limit:    cfg.MaxResponseSize,
	}
}

type Error struct {
	Method      string
	Code        int
	Description string
	RetryAfter  time.Duration
}

func (e *Error) Error() string {
	return fmt.Sprintf("telegram %s answered %d: %s", e.Method, e.Code, e.Description)
}

type Bot struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

type Webhook struct {
	URL            string
	SecretToken    string
	AllowedUpdates []string
}

type Button struct {
	Text         string
	CallbackData string
}

type Outgoing struct {
	ChatID   int64
	Text     string
	ReplyTo  int64
	Keyboard [][]Button
}

type Edit struct {
	ChatID    int64
	MessageID int64
	Text      string
}

type envelope struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	Description string          `json:"description"`
	ErrorCode   int             `json:"error_code"`
	Parameters  struct {
		RetryAfter int `json:"retry_after"`
	} `json:"parameters"`
}

type webhookRequest struct {
	URL            string   `json:"url"`
	SecretToken    string   `json:"secret_token"`
	AllowedUpdates []string `json:"allowed_updates"`
	DropPending    bool     `json:"drop_pending_updates"`
}

type inlineButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

type replyMarkup struct {
	InlineKeyboard [][]inlineButton `json:"inline_keyboard"`
}

type replyParameters struct {
	MessageID                int64 `json:"message_id"`
	AllowSendingWithoutReply bool  `json:"allow_sending_without_reply"`
}

type linkPreview struct {
	IsDisabled bool `json:"is_disabled"`
}

type sendRequest struct {
	ChatID      int64            `json:"chat_id"`
	Text        string           `json:"text"`
	ParseMode   string           `json:"parse_mode"`
	LinkPreview linkPreview      `json:"link_preview_options"`
	Reply       *replyParameters `json:"reply_parameters,omitempty"`
	Markup      *replyMarkup     `json:"reply_markup,omitempty"`
}

type editRequest struct {
	ChatID      int64       `json:"chat_id"`
	MessageID   int64       `json:"message_id"`
	Text        string      `json:"text"`
	ParseMode   string      `json:"parse_mode"`
	LinkPreview linkPreview `json:"link_preview_options"`
}

type sentResult struct {
	MessageID int64 `json:"message_id"`
}

type callbackAnswer struct {
	CallbackQueryID string `json:"callback_query_id"`
	Text            string `json:"text,omitempty"`
}

type chatAction struct {
	ChatID int64  `json:"chat_id"`
	Action string `json:"action"`
}

func (c *Client) GetMe(ctx context.Context, token string) (Bot, error) {
	var bot Bot
	if err := c.call(ctx, token, methodGetMe, nil, &bot); err != nil {
		return Bot{}, err
	}

	return bot, nil
}

func (c *Client) SetWebhook(ctx context.Context, token string, hook Webhook) error {
	return c.call(ctx, token, methodSetWebhook, webhookRequest{
		URL:            hook.URL,
		SecretToken:    hook.SecretToken,
		AllowedUpdates: hook.AllowedUpdates,
		DropPending:    true,
	}, nil)
}

func (c *Client) DeleteWebhook(ctx context.Context, token string) error {
	return c.call(ctx, token, methodDeleteWebhook, nil, nil)
}

func (c *Client) SendMessage(ctx context.Context, token string, message Outgoing) (int64, error) {
	request := sendRequest{
		ChatID:      message.ChatID,
		Text:        message.Text,
		ParseMode:   parseModeHTML,
		LinkPreview: linkPreview{IsDisabled: true},
	}

	if message.ReplyTo != 0 {
		request.Reply = &replyParameters{MessageID: message.ReplyTo, AllowSendingWithoutReply: true}
	}

	if len(message.Keyboard) > 0 {
		request.Markup = keyboard(message.Keyboard)
	}

	var sent sentResult
	if err := c.call(ctx, token, methodSendMessage, request, &sent); err != nil {
		return 0, err
	}

	return sent.MessageID, nil
}

func (c *Client) EditMessageText(ctx context.Context, token string, edit Edit) error {
	return c.call(ctx, token, methodEditMessageText, editRequest{
		ChatID:      edit.ChatID,
		MessageID:   edit.MessageID,
		Text:        edit.Text,
		ParseMode:   parseModeHTML,
		LinkPreview: linkPreview{IsDisabled: true},
	}, nil)
}

func (c *Client) AnswerCallbackQuery(ctx context.Context, token, callbackID, text string) error {
	return c.call(ctx, token, methodAnswerCallbackQuery, callbackAnswer{
		CallbackQueryID: callbackID,
		Text:            text,
	}, nil)
}

func (c *Client) SendChatAction(ctx context.Context, token string, chatID int64, action string) error {
	return c.call(ctx, token, methodSendChatAction, chatAction{ChatID: chatID, Action: action}, nil)
}

func keyboard(rows [][]Button) *replyMarkup {
	markup := &replyMarkup{InlineKeyboard: make([][]inlineButton, 0, len(rows))}

	for _, row := range rows {
		buttons := make([]inlineButton, 0, len(row))
		for _, button := range row {
			buttons = append(buttons, inlineButton(button))
		}

		markup.InlineKeyboard = append(markup.InlineKeyboard, buttons)
	}

	return markup
}

func (c *Client) call(ctx context.Context, token, method string, payload, result any) error {
	body := []byte("{}")

	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("telegram %s: encode: %w", method, err)
		}

		body = encoded
	}

	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.endpoint+"/bot"+token+"/"+method, bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("telegram %s: %w", method, withoutURL(err))
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := c.http.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		return fmt.Errorf("telegram %s: %w", method, withoutURL(err))
	}

	defer func() { _ = response.Body.Close() }()

	read, err := c.read(response.Body)
	if err != nil {
		return fmt.Errorf("telegram %s: %w", method, err)
	}

	var answer envelope
	if err := json.Unmarshal(read, &answer); err != nil {
		return &Error{Method: method, Code: response.StatusCode, Description: "the answer was not JSON"}
	}

	if !answer.OK {
		return &Error{
			Method:      method,
			Code:        codeOf(answer.ErrorCode, response.StatusCode),
			Description: truncated(answer.Description),
			RetryAfter:  time.Duration(answer.Parameters.RetryAfter) * time.Second,
		}
	}

	if result == nil {
		return nil
	}

	if err := json.Unmarshal(answer.Result, result); err != nil {
		return fmt.Errorf("telegram %s: decode result: %w", method, err)
	}

	return nil
}

func (c *Client) read(body io.Reader) ([]byte, error) {
	read, err := io.ReadAll(io.LimitReader(body, c.limit+1))
	if err != nil {
		return nil, withoutURL(err)
	}

	if int64(len(read)) > c.limit {
		return nil, fmt.Errorf("%w (%d bytes)", ErrResponseTooLarge, c.limit)
	}

	return read, nil
}

func withoutURL(err error) error {
	var located *url.Error
	if errors.As(err, &located) {
		return fmt.Errorf("%s: %w", located.Op, located.Err)
	}

	return err
}

func codeOf(said, status int) int {
	if said != 0 {
		return said
	}

	return status
}

func truncated(description string) string {
	trimmed := strings.TrimSpace(description)
	if len(trimmed) > descriptionLimit {
		return trimmed[:descriptionLimit]
	}

	return trimmed
}
