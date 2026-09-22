package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"strings"
	"syscall"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/outbound"
)

const (
	modelsPath          = "/models"
	responsesPath       = "/responses"
	probeInput          = "ping"
	probeTokens         = 16
	insufficientQuota   = "insufficient_quota"
	modelNotFound       = "model_not_found"
	providerReasonLimit = 256
)

var (
	ErrResponseTooLarge = errors.New("the provider answered with more than this instance will read")

	nonTextModelParts = []string{
		"embedding", "moderation", "whisper", "transcribe", "tts", "audio", "realtime",
		"dall-e", "image", "sora", "search", "computer-use", "babbage", "davinci",
	}
)

type Client struct {
	public   *http.Client
	private  *http.Client
	endpoint string
	limit    int64
}

func New(cfg config.OpenAI) (*Client, error) {
	allowed, err := outbound.ParsePrefixes(cfg.AllowedDestinations)
	if err != nil {
		return nil, err
	}

	return &Client{
		public:   guarded(cfg, outbound.Control(allowed)),
		private:  guarded(cfg, outbound.ControlAllowingPrivate(allowed)),
		endpoint: strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/"),
		limit:    cfg.MaxResponseSize,
	}, nil
}

func guarded(cfg config.OpenAI, control func(string, string, syscall.RawConn) error) *http.Client {
	dialer := &net.Dialer{Timeout: cfg.DialTimeout, Control: control}

	return &http.Client{
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
	}
}

type modelList struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

type probeRequest struct {
	Model           string `json:"model"`
	Input           string `json:"input"`
	MaxOutputTokens int    `json:"max_output_tokens"`
}

type failure struct {
	Error struct {
		Message string `json:"message"`
		Code    string `json:"code"`
		Type    string `json:"type"`
	} `json:"error"`
}

func (c *Client) TextModels(
	ctx context.Context,
	endpoint entity.AIProviderEndpoint,
	apiKey string,
) ([]string, error) {
	body, err := c.call(ctx, endpoint, http.MethodGet, modelsPath, apiKey, nil)
	if err != nil {
		return nil, err
	}

	var listed modelList
	if err := json.Unmarshal(body, &listed); err != nil {
		return nil, fmt.Errorf("%w: the model list was not JSON: %v", entity.ErrAIProviderUnreachable, err)
	}

	models := make([]string, 0, len(listed.Data))

	for _, model := range listed.Data {
		if generatesText(model.ID) {
			models = append(models, model.ID)
		}
	}

	slices.Sort(models)

	return models, nil
}

func (c *Client) Probe(
	ctx context.Context,
	endpoint entity.AIProviderEndpoint,
	apiKey, model string,
) error {
	payload, err := json.Marshal(probeRequest{
		Model:           model,
		Input:           probeInput,
		MaxOutputTokens: probeTokens,
	})
	if err != nil {
		return fmt.Errorf("encode probe: %w", err)
	}

	_, err = c.call(ctx, endpoint, http.MethodPost, responsesPath, apiKey, payload)

	return err
}

func generatesText(model string) bool {
	if strings.TrimSpace(model) == "" {
		return false
	}

	for _, part := range nonTextModelParts {
		if strings.Contains(model, part) {
			return false
		}
	}

	return true
}

func (c *Client) call(
	ctx context.Context,
	endpoint entity.AIProviderEndpoint,
	method, path, apiKey string,
	payload []byte,
) ([]byte, error) {
	base := c.endpoint
	if endpoint.BaseURL != "" {
		base = strings.TrimRight(endpoint.BaseURL, "/")
	}

	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}

	request, err := http.NewRequestWithContext(ctx, method, base+path, body)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", entity.ErrAIProviderUnreachable, err)
	}

	request.Header.Set("Authorization", "Bearer "+apiKey)
	request.Header.Set("Accept", "application/json")

	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	caller := c.public
	if endpoint.AllowPrivateAddress {
		caller = c.private
	}

	response, err := caller.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if errors.Is(err, outbound.ErrDestinationRefused) {
			return nil, fmt.Errorf("%w: %w", entity.ErrAIProviderDestinationRefused, err)
		}

		return nil, fmt.Errorf("%w: %v", entity.ErrAIProviderUnreachable, err)
	}

	defer func() { _ = response.Body.Close() }()

	read, err := c.read(response.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", entity.ErrAIProviderUnreachable, err)
	}

	if response.StatusCode != http.StatusOK {
		return nil, refusal(response.StatusCode, read)
	}

	return read, nil
}

func (c *Client) read(body io.Reader) ([]byte, error) {
	read, err := io.ReadAll(io.LimitReader(body, c.limit+1))
	if err != nil {
		return nil, err
	}

	if int64(len(read)) > c.limit {
		return nil, fmt.Errorf("%w (%d bytes)", ErrResponseTooLarge, c.limit)
	}

	return read, nil
}

func refusal(status int, body []byte) error {
	var said failure
	_ = json.Unmarshal(body, &said)

	reason := reasonOf(said.Error.Message, status)

	switch {
	case status == http.StatusUnauthorized, status == http.StatusForbidden:
		return fmt.Errorf("%w: %s", entity.ErrAIProviderKeyRejected, reason)
	case status == http.StatusTooManyRequests &&
		(said.Error.Code == insufficientQuota || said.Error.Type == insufficientQuota):
		return fmt.Errorf("%w: %s", entity.ErrAIProviderQuotaExceeded, reason)
	case status == http.StatusTooManyRequests:
		return fmt.Errorf("%w: %s", entity.ErrAIProviderRateLimited, reason)
	case status == http.StatusNotFound, status == http.StatusBadRequest, said.Error.Code == modelNotFound:
		return fmt.Errorf("%w: %s", entity.ErrAIProviderModelUnavailable, reason)
	default:
		return fmt.Errorf("%w: %s", entity.ErrAIProviderUnreachable, reason)
	}
}

func reasonOf(message string, status int) string {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return fmt.Sprintf("the provider answered %d", status)
	}

	if len(trimmed) > providerReasonLimit {
		return trimmed[:providerReasonLimit]
	}

	return trimmed
}
