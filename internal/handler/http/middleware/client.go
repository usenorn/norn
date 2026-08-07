package middleware

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
)

func ClientCapture(cfg config.HTTP) func(http.Handler) http.Handler {
	trusted, _ := cfg.TrustedPrefixes()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			client := entity.SessionClient{
				UserAgent: entity.TruncateUserAgent(r.UserAgent()),
				IP:        clientIP(r, cfg.ClientIPHeader, trusted),
			}

			next.ServeHTTP(w, r.WithContext(identity.WithClient(r.Context(), client)))
		})
	}
}

func ClientFrom(ctx context.Context) entity.SessionClient {
	return identity.Client(ctx)
}

func clientIP(r *http.Request, header string, trusted []netip.Prefix) netip.Addr {
	peer := peerIP(r)

	if header == "" || !within(peer, trusted) {
		return peer
	}

	hops := strings.Split(strings.Join(r.Header.Values(header), ","), ",")

	for i := len(hops) - 1; i >= 0; i-- {
		hop, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			return peer
		}

		if forwarded := normalizeIP(hop); !within(forwarded, trusted) {
			return forwarded
		}
	}

	return peer
}

func within(ip netip.Addr, prefixes []netip.Prefix) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(ip) {
			return true
		}
	}

	return false
}

func peerIP(r *http.Request) netip.Addr {
	if addrPort, err := netip.ParseAddrPort(r.RemoteAddr); err == nil {
		return normalizeIP(addrPort.Addr())
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	ip, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}

	return normalizeIP(ip)
}

func normalizeIP(ip netip.Addr) netip.Addr {
	return ip.Unmap().WithZone("")
}
