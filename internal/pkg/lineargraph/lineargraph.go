package lineargraph

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
)

const (
	rateLimitedCode  = "RATELIMITED"
	resetHeader      = "X-RateLimit-Requests-Reset"
	retryAfterHeader = "Retry-After"
	authenticateCode = "AUTHENTICATION_ERROR"
	forbiddenCode    = "FORBIDDEN"
)

var ErrResponseTooLarge = errors.New("the source answered with more than this instance will read")

type Client struct {
	http     *http.Client
	endpoint string
	limit    int64
}

func New(cfg config.Linear) *Client {
	return &Client{
		http:     &http.Client{Timeout: cfg.RequestTimeout},
		endpoint: strings.TrimSpace(cfg.Endpoint),
		limit:    cfg.MaxResponseSize,
	}
}

type request struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type failure struct {
	Message    string         `json:"message"`
	Extensions map[string]any `json:"extensions"`
}

type envelope struct {
	Data   json.RawMessage `json:"data"`
	Errors []failure       `json:"errors"`
}

// Query runs one GraphQL operation and hands back the raw data node. The caller decodes
// it, because every resource shapes its own reply and a shared struct would be a second
// vocabulary alongside the import payloads.
func (c *Client) Query(
	ctx context.Context,
	key, query string,
	variables map[string]any,
	resource entity.ImportResource,
) (json.RawMessage, error) {
	body, err := json.Marshal(request{Query: query, Variables: variables})
	if err != nil {
		return nil, fmt.Errorf("encode query: %w", err)
	}

	call, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	call.Header.Set("Content-Type", "application/json")
	call.Header.Set("Accept", "application/json")
	call.Header.Set("Authorization", key)

	response, err := c.http.Do(call)
	if err != nil {
		return nil, entity.ImportSourceUnavailableError{Resource: resource, Reason: err.Error()}
	}

	defer func() { _ = response.Body.Close() }()

	read, err := c.read(response.Body)
	if err != nil {
		return nil, entity.ImportSourceUnavailableError{
			Resource: resource,
			Reason:   err.Error(),
			Cause:    err,
		}
	}

	if failed := c.status(response, read, resource); failed != nil {
		return nil, failed
	}

	var answer envelope

	if err := json.Unmarshal(read, &answer); err != nil {
		return nil, entity.ImportSourceUnavailableError{
			Resource: resource,
			Reason:   fmt.Sprintf("the source answered with something that is not GraphQL: %v", err),
		}
	}

	if len(answer.Errors) > 0 {
		return nil, c.reported(response, answer.Errors, resource)
	}

	return answer.Data, nil
}

// read refuses an oversized body rather than truncating it. The webhook client's capped
// transport silently ends the reader at its limit, which for JSON surfaces later as an
// unexpected end of input and points nowhere near the cause.
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

func (c *Client) status(response *http.Response, body []byte, resource entity.ImportResource) error {
	switch {
	case response.StatusCode == http.StatusTooManyRequests:
		return entity.ImportRateLimitedError{Resource: resource, RetryAfter: waitFrom(response)}

	case response.StatusCode == http.StatusUnauthorized,
		response.StatusCode == http.StatusForbidden:
		return entity.ImportSourceRefusedError{Resource: resource, Reason: excerpt(body)}

	case response.StatusCode >= http.StatusInternalServerError:
		return entity.ImportSourceUnavailableError{
			Resource: resource,
			Reason:   fmt.Sprintf("the source answered %d: %s", response.StatusCode, excerpt(body)),
		}

	case response.StatusCode != http.StatusOK:
		return entity.ImportSourceUnavailableError{
			Resource: resource,
			Reason:   fmt.Sprintf("the source answered %d: %s", response.StatusCode, excerpt(body)),
		}

	default:
		return nil
	}
}

// reported reads the errors array a GraphQL source returns alongside HTTP 200. A refusal
// must not be retried — twenty more attempts with the same rejected key change nothing —
// while a rate limit parks the cursor and resumes.
func (c *Client) reported(
	response *http.Response,
	failures []failure,
	resource entity.ImportResource,
) error {
	reasons := make([]string, 0, len(failures))
	limited, refused := false, false

	for _, reported := range failures {
		reasons = append(reasons, reported.Message)

		switch code(reported) {
		case rateLimitedCode:
			limited = true
		case authenticateCode, forbiddenCode:
			refused = true
		}
	}

	said := strings.Join(reasons, "; ")

	switch {
	case limited:
		return entity.ImportRateLimitedError{Resource: resource, RetryAfter: waitFrom(response)}
	case refused:
		return entity.ImportSourceRefusedError{Resource: resource, Reason: said}
	default:
		return entity.ImportSourceUnavailableError{Resource: resource, Reason: said}
	}
}

func code(reported failure) string {
	value, named := reported.Extensions["code"].(string)
	if !named {
		return ""
	}

	return strings.ToUpper(strings.TrimSpace(value))
}

func waitFrom(response *http.Response) time.Duration {
	if seconds := response.Header.Get(retryAfterHeader); seconds != "" {
		if parsed, err := strconv.Atoi(seconds); err == nil && parsed > 0 {
			return time.Duration(parsed) * time.Second
		}
	}

	reset := response.Header.Get(resetHeader)
	if reset == "" {
		return 0
	}

	epoch, err := strconv.ParseInt(reset, 10, 64)
	if err != nil {
		return 0
	}

	// The header is a millisecond epoch when it is long enough to be one, and a second
	// epoch otherwise. Reading it the wrong way round parks a run for fifty thousand years.
	if epoch > 1e11 {
		epoch /= 1000
	}

	until := time.Until(time.Unix(epoch, 0))
	if until < 0 {
		return 0
	}

	return until
}

func excerpt(body []byte) string {
	const most = 256

	trimmed := strings.TrimSpace(string(body))

	if len(trimmed) > most {
		return trimmed[:most]
	}

	return trimmed
}
