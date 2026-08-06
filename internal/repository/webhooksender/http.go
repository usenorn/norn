package webhooksender

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/outbound"
	"github.com/usenorn/norn/internal/repository"
)

type webhookSender struct {
	client *outbound.Client
}

func New(client *outbound.Client) repository.WebhookSender {
	return &webhookSender{client: client}
}

func (s *webhookSender) Check(ctx context.Context, url string) error {
	if err := s.client.Check(ctx, url); err != nil {
		if errors.Is(err, outbound.ErrDestinationRefused) {
			return entity.ErrWebhookDestinationRefused
		}

		return err
	}

	return nil
}

func (s *webhookSender) Send(ctx context.Context, request entity.WebhookRequest) entity.WebhookResponse {
	started := time.Now().UTC()

	result := entity.WebhookResponse{
		ResolvedAddress: s.client.Resolve(ctx, request.URL),
		StartedAt:       started,
	}

	outgoing, err := http.NewRequestWithContext(
		ctx, http.MethodPost, request.URL, bytes.NewReader(request.Body),
	)
	if err != nil {
		return s.settle(result, entity.WebhookAttemptRefused, err)
	}

	for name, value := range request.Headers {
		outgoing.Header.Set(name, value)
	}

	response, err := s.client.Do(outgoing)
	if err != nil {
		return s.settle(result, classify(ctx, err), err)
	}

	defer func() { _ = response.Body.Close() }()

	body, _ := io.ReadAll(response.Body)

	result.StatusCode = response.StatusCode
	result.Excerpt = excerpt(body)

	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		return s.settle(result, entity.WebhookAttemptSucceeded, nil)
	}

	if location := response.Header.Get("Location"); location != "" && result.Excerpt == "" {
		result.Excerpt = "Location: " + location
	}

	return s.settle(result, entity.WebhookAttemptRejected, nil)
}

func (s *webhookSender) settle(
	response entity.WebhookResponse,
	outcome entity.WebhookAttemptOutcome,
	cause error,
) entity.WebhookResponse {
	response.Outcome = outcome
	response.FinishedAt = time.Now().UTC()

	if cause != nil {
		response.Error = cause.Error()
	}

	return response
}

func classify(ctx context.Context, err error) entity.WebhookAttemptOutcome {
	switch {
	case errors.Is(err, outbound.ErrDestinationRefused):
		return entity.WebhookAttemptRefused
	case errors.Is(err, context.DeadlineExceeded), errors.Is(ctx.Err(), context.DeadlineExceeded):
		return entity.WebhookAttemptTimedOut
	default:
		var timeout interface{ Timeout() bool }
		if errors.As(err, &timeout) && timeout.Timeout() {
			return entity.WebhookAttemptTimedOut
		}

		return entity.WebhookAttemptFailed
	}
}

func excerpt(body []byte) string {
	trimmed := strings.TrimSpace(string(body))

	if len(trimmed) > entity.WebhookResponseExcerpt {
		return trimmed[:entity.WebhookResponseExcerpt]
	}

	return trimmed
}
