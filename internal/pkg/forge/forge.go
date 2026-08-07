package forge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"

	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"net/netip"
	"strings"
	"sync"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/outbound"
)

var ErrResponseTooLarge = errors.New("the forge answered with more than this instance will read")

type Client struct {
	http    *http.Client
	cfg     config.SourceControl
	allowed []netip.Prefix
	limit   int64

	mu      sync.Mutex
	granted map[string]*http.Client
}

// New builds a client of its own rather than reusing the webhook one. That client caps a
// body by silently ending the reader, which for an API surfaces later as broken JSON at an
// offset naming nothing; it refuses redirects, which turns a renamed repository into a dead
// connection; and it closes every connection, which a sweep making dozens of calls to one
// host pays for on each. The dial guard is the one part worth sharing, because a second
// copy of an allow-list is a hole waiting for one copy to be patched.
func New(cfg config.SourceControl) (*Client, error) {
	allowed, err := outbound.ParsePrefixes(cfg.AllowedDestinations)
	if err != nil {
		return nil, err
	}

	dialer := &net.Dialer{Timeout: cfg.DialTimeout, Control: outbound.Control(allowed)}

	return &Client{
		cfg:     cfg,
		allowed: allowed,
		granted: make(map[string]*http.Client, 2),
		http: &http.Client{
			Timeout: cfg.RequestTimeout,
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				DialContext:           dialer.DialContext,
				TLSHandshakeTimeout:   cfg.DialTimeout,
				ResponseHeaderTimeout: cfg.RequestTimeout,
				ForceAttemptHTTP2:     true,
				MaxIdleConnsPerHost:   4,
			},
		},
		limit: cfg.MaxResponseSize,
	}, nil
}

type Request struct {
	Provider entity.SCMProvider
	Method   string
	URL      string
	Header   http.Header
	Body     []byte
	Trust    entity.SCMTrust
}

type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

func (r Response) Link(rel string) string {
	return linkFor(r.Header.Get("Link"), rel)
}

// Do classifies everything a forge can say about a call except which resource it was about.
// A 2xx and the 4xx an adapter has to read for itself come back as a Response; everything
// that means the same thing whatever was being asked for comes back as a typed error, so a
// caller decides whether to park, retry or stop rather than the transport deciding for it.
func (c *Client) Do(ctx context.Context, request Request) (Response, error) {
	var body *bytes.Reader
	if len(request.Body) > 0 {
		body = bytes.NewReader(request.Body)
	} else {
		body = bytes.NewReader(nil)
	}

	call, err := http.NewRequestWithContext(ctx, request.Method, request.URL, body)
	if err != nil {
		return Response{}, fmt.Errorf("build request: %w", err)
	}

	for name, values := range request.Header {
		for _, value := range values {
			call.Header.Add(name, value)
		}
	}

	caller, err := c.clientFor(request.Trust)
	if err != nil {
		return Response{}, entity.SCMDestinationRefusedError{
			Provider: request.Provider,
			Reason:   err.Error(),
			Cause:    err,
		}
	}

	response, err := caller.Do(call)
	if err != nil {
		// Refused by our own dial guard, so nothing was ever asked of the forge. Reported as
		// an unreachable repository this reads as "check the token", and somebody goes and
		// rotates a credential that was never involved.
		if errors.Is(err, outbound.ErrDestinationRefused) {
			return Response{}, entity.SCMDestinationRefusedError{
				Provider: request.Provider,
				Reason:   err.Error(),
				Cause:    err,
			}
		}

		return Response{}, entity.SCMUnavailableError{
			Provider: request.Provider,
			Reason:   err.Error(),
			Cause:    err,
		}
	}

	defer func() { _ = response.Body.Close() }()

	read, err := c.read(response.Body)
	if err != nil {
		return Response{}, entity.SCMUnavailableError{
			Provider: request.Provider,
			Reason:   err.Error(),
			Cause:    err,
		}
	}

	answer := Response{Status: response.StatusCode, Header: response.Header, Body: read}

	if failed := classify(request.Provider, response, read); failed != nil {
		return answer, failed
	}

	return answer, nil
}

func (c *Client) read(body io.Reader) ([]byte, error) {
	return readCapped(body, c.limit)
}

func (c *Client) clientFor(trust entity.SCMTrust) (*http.Client, error) {
	if !trust.Custom() {
		return c.http, nil
	}

	key := trustKey(trust)

	c.mu.Lock()
	defer c.mu.Unlock()

	if held, found := c.granted[key]; found {
		return held, nil
	}

	transport, err := c.transportFor(trust)
	if err != nil {
		return nil, err
	}

	built := &http.Client{Timeout: c.cfg.RequestTimeout, Transport: transport}
	c.granted[key] = built

	return built, nil
}

func (c *Client) transportFor(trust entity.SCMTrust) (*http.Transport, error) {
	control := outbound.Control(c.allowed)
	if trust.AllowPrivateAddress {
		control = outbound.ControlAllowingPrivate(c.allowed)
	}

	dialer := &net.Dialer{Timeout: c.cfg.DialTimeout, Control: control}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   c.cfg.DialTimeout,
		ResponseHeaderTimeout: c.cfg.RequestTimeout,
		ForceAttemptHTTP2:     true,
		MaxIdleConnsPerHost:   4,
	}

	if strings.TrimSpace(trust.CACertificate) == "" {
		return transport, nil
	}

	certificates, err := entity.ParseSCMCertificates(trust.CACertificate)
	if err != nil {
		return nil, err
	}

	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}

	for _, certificate := range certificates {
		pool.AddCert(certificate)
	}

	transport.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}

	return transport, nil
}

func trustKey(trust entity.SCMTrust) string {
	sum := sha256.Sum256([]byte(trust.CACertificate))

	if trust.AllowPrivateAddress {
		return "private:" + hex.EncodeToString(sum[:])
	}

	return "public:" + hex.EncodeToString(sum[:])
}
