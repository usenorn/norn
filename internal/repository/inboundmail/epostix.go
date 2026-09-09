package inboundmail

import (
	"context"
	"fmt"
	"net/http"
	"time"
	"unicode/utf8"

	epostix "github.com/epostix/sdk-go"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
)

type epostixMail struct {
	client   *epostix.Client
	verifier *epostix.WebhookVerifier
	maxBody  int
}

func New(cfg config.Epostix) repository.InboundMail {
	options := []epostix.Option{
		epostix.WithHTTPClient(&http.Client{Timeout: cfg.RequestTimeout}),
		epostix.WithUserAgentSuffix("norn"),
	}

	if cfg.Endpoint != "" {
		options = append(options, epostix.WithBaseURL(cfg.Endpoint))
	}

	return &epostixMail{
		client:   epostix.New(cfg.APIKey, options...),
		verifier: epostix.NewWebhookVerifier([]string{cfg.WebhookSecret}),
		maxBody:  cfg.MaxBodyBytes,
	}
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

	content, err := m.client.Inbound.GetInboundEmailContent(ctx, messageID)
	if err != nil {
		return entity.InboundMessage{}, fmt.Errorf("read inbound message content: %w", err)
	}

	message := entity.InboundMessage{
		ExternalID: received.ID,
		Sender:     received.From,
		Recipients: received.To,
		Subject:    received.Subject,
		Text:       trim(text(content.Text), m.maxBody),
		HTML:       trim(text(content.HTML), m.maxBody),
		ReceivedAt: received.ReceivedAt.UTC(),
	}

	if message.ReceivedAt.IsZero() {
		message.ReceivedAt = time.Now().UTC()
	}

	for _, attachment := range content.Attachments {
		if attachment.Filename != nil && *attachment.Filename != "" {
			message.Attachments = append(message.Attachments, *attachment.Filename)
		}
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
