package toolingclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/pkg/outbound"
)

var (
	ErrNotFound      = errors.New("the remote answered that nothing is there")
	ErrTooLarge      = errors.New("the remote answered with more than this instance will read")
	ErrUnexpected    = errors.New("the remote answered with something this instance does not understand")
	ErrUnreachable   = errors.New("the remote could not be reached")
	ErrRefused       = errors.New("this instance will not open a connection to that address")
	ErrRateLimited   = errors.New("the remote is rate limiting this instance")
	ErrUnauthorized  = errors.New("the remote refused the request's credentials")
	errStatusUnknown = errors.New("unexpected status")
)

type Client struct {
	public   *http.Client
	private  *http.Client
	github   string
	raw      string
	token    string
	registry string
	limit    int64
}

func New(cfg config.AgentTooling) (*Client, error) {
	allowed, err := outbound.ParsePrefixes(cfg.AllowedDestinations)
	if err != nil {
		return nil, err
	}

	return &Client{
		public:   guarded(cfg, outbound.Control(allowed)),
		private:  guarded(cfg, outbound.ControlAllowingPrivate(allowed)),
		github:   strings.TrimRight(strings.TrimSpace(cfg.GitHubEndpoint), "/"),
		raw:      strings.TrimRight(strings.TrimSpace(cfg.GitHubRawEndpoint), "/"),
		token:    strings.TrimSpace(cfg.GitHubToken),
		registry: strings.TrimRight(strings.TrimSpace(cfg.RegistryEndpoint), "/"),
		limit:    cfg.MaxResponseSize,
	}, nil
}

func guarded(cfg config.AgentTooling, control func(string, string, syscall.RawConn) error) *http.Client {
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

func (c *Client) HTTP(allowPrivate bool) *http.Client {
	if allowPrivate {
		return c.private
	}

	return c.public
}

func (c *Client) GitHubURL(path string) string {
	return c.github + path
}

func (c *Client) GitHubRawURL(path string) string {
	return c.raw + path
}

func (c *Client) GitHubHeader() http.Header {
	header := http.Header{}
	header.Set("Accept", "application/vnd.github+json")
	header.Set("X-GitHub-Api-Version", "2022-11-28")

	if c.token != "" {
		header.Set("Authorization", "Bearer "+c.token)
	}

	return header
}

func (c *Client) RegistryURL(path string) string {
	return c.registry + path
}

func (c *Client) Get(ctx context.Context, target string, header http.Header, allowPrivate bool) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnexpected, err)
	}

	for name, values := range header {
		for _, value := range values {
			request.Header.Add(name, value)
		}
	}

	response, err := c.HTTP(allowPrivate).Do(request)
	if err != nil {
		if errors.Is(err, outbound.ErrDestinationRefused) {
			return nil, fmt.Errorf("%w: %v", ErrRefused, err)
		}

		return nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	defer func() { _ = response.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(response.Body, c.limit+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}

	if int64(len(body)) > c.limit {
		return nil, ErrTooLarge
	}

	if err := statusError(response.StatusCode); err != nil {
		return nil, err
	}

	return body, nil
}

func (c *Client) GetJSON(ctx context.Context, target string, header http.Header, out any) error {
	body, err := c.Get(ctx, target, header, false)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%w: %v", ErrUnexpected, err)
	}

	return nil
}

func statusError(status int) error {
	switch {
	case status >= http.StatusOK && status < http.StatusMultipleChoices:
		return nil
	case status == http.StatusNotFound, status == http.StatusGone,
		status == http.StatusMovedPermanently, status == http.StatusFound:
		return ErrNotFound
	case status == http.StatusUnauthorized:
		return ErrUnauthorized
	case status == http.StatusTooManyRequests, status == http.StatusForbidden:
		return ErrRateLimited
	case status >= http.StatusInternalServerError:
		return fmt.Errorf("%w: status %d", ErrUnreachable, status)
	default:
		return fmt.Errorf("%w: %w %d", ErrUnexpected, errStatusUnknown, status)
	}
}
