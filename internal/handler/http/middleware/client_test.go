package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/handler/http/middleware"
)

const forwardedHeader = "X-Forwarded-For"

func capturedIP(t *testing.T, cfg config.HTTP, remoteAddr, forwarded string) netip.Addr {
	t.Helper()

	var captured netip.Addr

	handler := middleware.ClientCapture(cfg)(
		http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			captured = middleware.ClientFrom(r.Context()).IP
		}),
	)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = remoteAddr

	if forwarded != "" {
		request.Header.Set(forwardedHeader, forwarded)
	}

	handler.ServeHTTP(httptest.NewRecorder(), request)

	return captured
}

func TestForwardedAddressesAreOnlyTrustedFromAProxyOnTheAllowList(t *testing.T) {
	cases := []struct {
		name      string
		header    string
		proxies   []string
		remote    string
		forwarded string
		want      string
	}{
		{
			name:      "no header configured leaves the peer address in place",
			proxies:   []string{"127.0.0.1"},
			remote:    "127.0.0.1:41234",
			forwarded: "203.0.113.7",
			want:      "127.0.0.1",
		},
		{
			name:      "a forged header from an untrusted peer is ignored",
			header:    forwardedHeader,
			proxies:   []string{"127.0.0.1"},
			remote:    "198.51.100.4:41234",
			forwarded: "203.0.113.7",
			want:      "198.51.100.4",
		},
		{
			name:      "a trusted proxy hands over the client address",
			header:    forwardedHeader,
			proxies:   []string{"127.0.0.1"},
			remote:    "127.0.0.1:41234",
			forwarded: "203.0.113.7",
			want:      "203.0.113.7",
		},
		{
			name:      "an address prepended by the client is never reached",
			header:    forwardedHeader,
			proxies:   []string{"127.0.0.1"},
			remote:    "127.0.0.1:41234",
			forwarded: "9.9.9.9, 203.0.113.7",
			want:      "203.0.113.7",
		},
		{
			name:      "trusted hops are walked past from the right",
			header:    forwardedHeader,
			proxies:   []string{"127.0.0.1", "10.0.0.0/8"},
			remote:    "127.0.0.1:41234",
			forwarded: "203.0.113.7, 10.0.0.5",
			want:      "203.0.113.7",
		},
		{
			name:      "a chain of nothing but trusted hops falls back to the peer",
			header:    forwardedHeader,
			proxies:   []string{"127.0.0.1", "10.0.0.0/8"},
			remote:    "127.0.0.1:41234",
			forwarded: "10.0.0.9, 10.0.0.5",
			want:      "127.0.0.1",
		},
		{
			name:      "an unparseable hop stops the walk instead of exposing what is left of it",
			header:    forwardedHeader,
			proxies:   []string{"127.0.0.1"},
			remote:    "127.0.0.1:41234",
			forwarded: "203.0.113.7, unknown",
			want:      "127.0.0.1",
		},
		{
			name:      "a mapped address arrives as its IPv4 form",
			header:    forwardedHeader,
			proxies:   []string{"127.0.0.1"},
			remote:    "127.0.0.1:41234",
			forwarded: "::ffff:203.0.113.7",
			want:      "203.0.113.7",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := config.HTTP{ClientIPHeader: c.header, TrustedProxies: c.proxies}

			got := capturedIP(t, cfg, c.remote, c.forwarded)

			if got.String() != c.want {
				t.Fatalf("client IP from %s with %s %q = %s, want %s",
					c.remote, forwardedHeader, c.forwarded, got, c.want)
			}
		})
	}
}

func TestAnUnconfiguredInstanceStillRecordsTheConnectingAddress(t *testing.T) {
	got := capturedIP(t, config.HTTP{}, "198.51.100.4:41234", "203.0.113.7")

	if got.String() != "198.51.100.4" {
		t.Fatalf("client IP = %s, want the peer address 198.51.100.4", got)
	}
}
