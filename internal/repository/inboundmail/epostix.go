package inboundmail

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	epostix "github.com/epostix/sdk-go"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
)

type epostixMail struct {
	client     *epostix.Client
	verifier   *epostix.WebhookVerifier
	transport  *http.Client
	endpoint   string
	apiKey     string
	maxBody    int
	maxMessage int64
}

func New(cfg config.Epostix) repository.InboundMail {
	transport := &http.Client{Timeout: cfg.RequestTimeout}

	options := []epostix.Option{
		epostix.WithHTTPClient(transport),
		epostix.WithUserAgentSuffix("norn"),
	}

	endpoint := epostix.ProductionBaseURL

	if cfg.Endpoint != "" {
		endpoint = cfg.Endpoint
		options = append(options, epostix.WithBaseURL(endpoint))
	}

	return &epostixMail{
		client:     epostix.New(cfg.APIKey, options...),
		verifier:   epostix.NewWebhookVerifier([]string{cfg.WebhookSecret}),
		transport:  transport,
		endpoint:   strings.TrimSuffix(endpoint, "/"),
		apiKey:     cfg.APIKey,
		maxBody:    cfg.MaxBodyBytes,
		maxMessage: cfg.MaxMessageBytes,
	}
}

// raw reads the message as it was received. The SDK names an attachment but offers no way to
// read one, and the original message is the only place the bytes are, so this one call is made
// without it rather than leaving every file behind.
func (m *epostixMail) raw(ctx context.Context, messageID string) ([]byte, error) {
	request, err := http.NewRequestWithContext(
		ctx, http.MethodGet, m.endpoint+"/inbound/"+url.PathEscape(messageID)+"/raw", nil,
	)
	if err != nil {
		return nil, fmt.Errorf("build the raw message request: %w", err)
	}

	request.Header.Set("Authorization", "Bearer "+m.apiKey)
	request.Header.Set("Accept", "message/rfc822")

	response, err := m.transport.Do(request)
	if err != nil {
		return nil, fmt.Errorf("read the raw message: %w", err)
	}

	defer func() {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("read the raw message: the provider answered %d", response.StatusCode)
	}

	// A message over the cap is read no further. What was written is still filed; the files it
	// carried are reported as not kept rather than pulled into memory whole.
	held, err := io.ReadAll(io.LimitReader(response.Body, m.maxMessage+1))
	if err != nil {
		return nil, fmt.Errorf("read the raw message: %w", err)
	}

	if int64(len(held)) > m.maxMessage {
		return nil, entity.ErrIntakeMessageTooLarge
	}

	return held, nil
}

func (m *epostixMail) Verify(header http.Header, body []byte) (entity.InboundNotice, error) {
	event, delivery, err := m.verifier.VerifyAndDecode(body, header)
	if err != nil {
		return entity.InboundNotice{}, entity.ErrIntakeSignatureInvalid
	}

	notice := entity.InboundNotice{DeliveryID: delivery.DeliveryID}

	inbound, received := event.InboundEmail()
	if !received {
		return notice, nil
	}

	notice.MessageID = inbound.InboundID

	return notice, nil
}

func (m *epostixMail) Fetch(ctx context.Context, messageID string) (entity.InboundMessage, error) {
	received, err := m.client.Inbound.GetInboundEmail(ctx, messageID)
	if err != nil {
		return entity.InboundMessage{}, fmt.Errorf("read inbound message: %w", err)
	}

	message := entity.InboundMessage{
		ExternalID: received.ID,
		Sender:     received.From,
		Recipients: received.To,
		Subject:    received.Subject,
		ReceivedAt: received.ReceivedAt.UTC(),
	}

	if message.ReceivedAt.IsZero() {
		message.ReceivedAt = time.Now().UTC()
	}

	raw, err := m.raw(ctx, messageID)
	if err != nil {
		if !errors.Is(err, entity.ErrIntakeMessageTooLarge) {
			return entity.InboundMessage{}, err
		}

		// The words are worth filing even when the message is too big to take apart, so the
		// provider's own reading of it stands in and the files are reported as not kept.
		return m.content(ctx, message, err)
	}

	parsed, err := readMessage(raw)
	if err != nil {
		return m.content(ctx, message, err)
	}

	message.Text = trim(parsed.text, m.maxBody)
	message.HTML = trim(parsed.html, m.maxBody)
	message.Attachments = parsed.attachments

	return message, nil
}

// content falls back to the provider's parsed reading of the message. It carries the words and
// the names of the files, never their bytes, so what it returns is a message whose attachments
// are known about and cannot be stored.
func (m *epostixMail) content(
	ctx context.Context,
	message entity.InboundMessage,
	because error,
) (entity.InboundMessage, error) {
	content, err := m.client.Inbound.GetInboundEmailContent(ctx, message.ExternalID)
	if err != nil {
		return entity.InboundMessage{}, errors.Join(
			because, fmt.Errorf("read inbound message content: %w", err),
		)
	}

	message.Text = trim(text(content.Text), m.maxBody)
	message.HTML = trim(text(content.HTML), m.maxBody)

	for _, attachment := range content.Attachments {
		if attachment.Filename == nil || *attachment.Filename == "" {
			continue
		}

		message.Attachments = append(message.Attachments, entity.InboundAttachment{
			FileName:    *attachment.Filename,
			ContentType: text(attachment.ContentType),
		})
	}

	return message, nil
}

func text(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

func trim(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}

	cut := limit
	for cut > 0 && !utf8.RuneStart(value[cut]) {
		cut--
	}

	return value[:cut]
}
