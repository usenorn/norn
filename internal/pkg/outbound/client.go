package outbound

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"github.com/usenorn/norn/internal/config"
)

type Client struct {
	http     *http.Client
	resolver *net.Resolver
	allowed  []netip.Prefix
}

func New(cfg config.Webhooks) (*Client, error) {
	allowed, err := parsePrefixes(cfg.AllowedDestinations)
	if err != nil {
		return nil, err
	}

	dialer := &net.Dialer{Timeout: cfg.DialTimeout, Control: control(allowed)}

	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   cfg.DialTimeout,
		ResponseHeaderTimeout: cfg.RequestTimeout,
		DisableKeepAlives:     true,
		ForceAttemptHTTP2:     true,
	}

	return &Client{
		http: &http.Client{
			Timeout:   cfg.RequestTimeout,
			Transport: &cappedTransport{inner: transport, limit: cfg.MaxResponseSize},
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		resolver: net.DefaultResolver,
		allowed:  allowed,
	}, nil
}

type cappedTransport struct {
	inner http.RoundTripper
	limit int64
}

func (t *cappedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.inner.RoundTrip(request)
	if err != nil {
		return nil, err
	}

	response.Body = struct {
		io.Reader
		io.Closer
	}{io.LimitReader(response.Body, t.limit), response.Body}

	return response, nil
}

func (c *Client) Do(request *http.Request) (*http.Response, error) {
	return c.http.Do(request)
}

func (c *Client) Check(ctx context.Context, raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("%w: %s", ErrDestinationRefused, raw)
	}

	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("%w: %s names no host", ErrDestinationRefused, raw)
	}

	if literal, err := netip.ParseAddr(host); err == nil {
		if !permitted(literal, c.allowed) {
			return refuse(literal)
		}

		return nil
	}

	addresses, err := c.resolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("%w: %s does not resolve", ErrDestinationRefused, host)
	}

	for _, address := range addresses {
		if !permitted(address, c.allowed) {
			return refuse(address)
		}
	}

	return nil
}

func (c *Client) Resolve(ctx context.Context, raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}

	host := parsed.Hostname()
	if host == "" {
		return ""
	}

	if literal, err := netip.ParseAddr(host); err == nil {
		return literal.Unmap().String()
	}

	addresses, err := c.resolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(addresses) == 0 {
		return ""
	}

	return addresses[0].Unmap().String()
}
