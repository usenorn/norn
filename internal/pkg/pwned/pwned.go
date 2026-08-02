package pwned

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/usenorn/norn/internal/config"
)

const paddingHeader = "Add-Padding"

type Client struct {
	*http.Client

	endpoint string
	enabled  bool
}

func New(cfg config.Password) *Client {
	return &Client{
		Client:   &http.Client{Timeout: cfg.BreachCheckTimeout},
		endpoint: cfg.BreachCheckEndpoint,
		enabled:  cfg.BreachCheckEnabled,
	}
}

func (c *Client) Enabled() bool {
	return c.enabled
}

func (c *Client) Range(ctx context.Context, prefix string) (io.ReadCloser, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint+"/"+prefix, nil)
	if err != nil {
		return nil, fmt.Errorf("build breach range request: %w", err)
	}

	request.Header.Set(paddingHeader, "true")

	response, err := c.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request breach range: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()

		return nil, fmt.Errorf("breach range responded with %s", response.Status)
	}

	return response.Body, nil
}
