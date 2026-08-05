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
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			client := entity.SessionClient{
				UserAgent: entity.TruncateUserAgent(r.UserAgent()),
				IP:        clientIP(r, cfg.ClientIPHeader),
			}

			next.ServeHTTP(w, r.WithContext(identity.WithClient(r.Context(), client)))
		})
	}
}

func ClientFrom(ctx context.Context) entity.SessionClient {
	return identity.Client(ctx)
}

func clientIP(r *http.Request, header string) netip.Addr {
	if header != "" {
		if forwarded := r.Header.Get(header); forwarded != "" {
			first, _, _ := strings.Cut(forwarded, ",")

			if ip, err := netip.ParseAddr(strings.TrimSpace(first)); err == nil {
				return normalizeIP(ip)
			}
		}
	}

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
