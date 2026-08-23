package preview

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/middleware"
	"github.com/usenorn/norn/internal/service"
)

const HealthPath = "/__norn/healthz"

type routeKey struct{}

type Edge struct {
	proxies  service.PreviewProxy
	previews config.Previews
	cfg      config.Gateway
	proxy    *httputil.ReverseProxy
}

func New(proxies service.PreviewProxy, previews config.Previews, cfg config.Gateway) *Edge {
	edge := &Edge{proxies: proxies, previews: previews, cfg: cfg}

	edge.proxy = &httputil.ReverseProxy{
		Rewrite:       rewrite,
		FlushInterval: -1,
		Transport: &http.Transport{
			DialContext:           edge.dial,
			MaxIdleConnsPerHost:   8,
			IdleConnTimeout:       30 * time.Second,
			ResponseHeaderTimeout: cfg.RequestTimeout,
			DisableCompression:    true,
		},
		ErrorHandler: edge.failed,
	}

	return edge
}

func (e *Edge) Serve(w http.ResponseWriter, r *http.Request) {
	host := named(r.Host)

	switch {
	case r.URL.Path == HealthPath:
		e.health(w)

		return
	case r.URL.Path == entity.PreviewSessionPath:
		e.session(w, r, host)

		return
	case strings.HasPrefix(r.URL.Path, entity.PreviewSharePath):
		e.share(w, r, host)

		return
	}

	reply, ok := e.route(w, r, host)
	if !ok {
		return
	}

	if reply.Mode == entity.PreviewByPath {
		e.unsupported(w, r, host)

		return
	}

	e.proxy.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), routeKey{}, reply)))
}

func (e *Edge) route(
	w http.ResponseWriter,
	r *http.Request,
	host string,
) (entity.PreviewReply, bool) {
	if host == "" || !e.serving(host) {
		e.unknown(w, r, host)

		return entity.PreviewReply{}, false
	}

	client := middleware.ClientFrom(r.Context())

	reply, err := e.proxies.Route(r.Context(), entity.PreviewAsk{
		Host:      host,
		Grant:     grantOf(r),
		IP:        client.IP,
		UserAgent: client.UserAgent,
	})
	if err != nil {
		e.refused(w, r, host, err)

		return entity.PreviewReply{}, false
	}

	switch reply.Verdict {
	case entity.PreviewAllowed:
		return reply, true
	case entity.PreviewSignIn:
		e.away(w, r, reply.Redirect)
	default:
		if reply.Preview == "" {
			e.unknown(w, r, host)
		} else {
			e.closed(w, r, host)
		}
	}

	return entity.PreviewReply{}, false
}

func (e *Edge) serving(host string) bool {
	domain := bare(e.previews.BaseDomain)
	if domain == "" {
		return false
	}

	return strings.HasSuffix(bare(host), "."+domain)
}

func (e *Edge) health(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")

	if !e.proxies.Ready() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("waiting for norn\n"))

		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (e *Edge) away(w http.ResponseWriter, r *http.Request, to string) {
	if to == "" {
		e.unknown(w, r, named(r.Host))

		return
	}

	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, to, http.StatusSeeOther)
}

func (e *Edge) refused(w http.ResponseWriter, r *http.Request, host string, err error) {
	switch {
	case errors.Is(err, entity.ErrGatewayUnready),
		errors.Is(err, entity.ErrPreviewGatewayCredentialInvalid):
		e.unready(w, r)
	case errors.Is(err, entity.ErrTunnelCrowded):
		e.crowded(w, r, host)
	case errors.Is(err, entity.ErrTunnelMissing):
		e.offline(w, r, host)
	case errors.Is(err, entity.ErrTunnelRefused):
		e.gone(w, r, host)
	default:
		e.broken(w, r, err)
	}
}

func (e *Edge) dial(ctx context.Context, _, _ string) (net.Conn, error) {
	reply, carried := ctx.Value(routeKey{}).(entity.PreviewReply)
	if !carried {
		return nil, entity.ErrTunnelRefused
	}

	return e.proxies.Dial(ctx, reply)
}

func (e *Edge) failed(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, context.Canceled) {
		return
	}

	e.refused(w, r, named(r.Host), err)
}

func rewrite(request *httputil.ProxyRequest) {
	reply, carried := request.In.Context().Value(routeKey{}).(entity.PreviewReply)
	if !carried {
		return
	}

	request.Out.URL.Scheme = "http"
	request.Out.URL.Host = reply.Preview + "." + strings.ToLower(reply.ExecutionID)
	request.Out.Host = request.In.Host

	request.SetXForwarded()
}

func named(host string) string {
	return strings.ToLower(strings.TrimSpace(host))
}

func bare(host string) string {
	host = named(host)

	if stripped, _, err := net.SplitHostPort(host); err == nil {
		return stripped
	}

	return strings.Trim(host, "[]")
}

func grantOf(r *http.Request) string {
	cookie, err := r.Cookie(entity.PreviewGrantCookie)
	if err != nil {
		return ""
	}

	return cookie.Value
}

func landing(reply entity.PreviewGrantReply, asked string) string {
	if strings.HasPrefix(asked, "/") && !strings.HasPrefix(asked, "//") {
		return asked
	}

	if reply.Path != "" {
		return reply.Path
	}

	return "/"
}

func (e *Edge) admit(w http.ResponseWriter, r *http.Request, reply entity.PreviewGrantReply) {
	http.SetCookie(w, &http.Cookie{
		Name:     entity.PreviewGrantCookie,
		Value:    reply.Grant,
		Path:     "/",
		Expires:  reply.ExpiresAt,
		HttpOnly: true,
		Secure:   e.previews.Scheme == "https",
		SameSite: http.SameSiteLaxMode,
	})

	e.away(w, r, landing(reply, r.URL.Query().Get("return")))
}

func (e *Edge) session(w http.ResponseWriter, r *http.Request, host string) {
	ticket := strings.TrimSpace(r.URL.Query().Get("ticket"))
	if ticket == "" {
		e.expired(w, r)

		return
	}

	reply, err := e.proxies.Session(r.Context(), ticket)
	if err != nil {
		switch {
		case errors.Is(err, entity.ErrPreviewShareExpired),
			errors.Is(err, entity.ErrPreviewShareNotFound):
			e.expired(w, r)
		default:
			e.refused(w, r, host, err)
		}

		return
	}

	e.admit(w, r, reply)
}

func (e *Edge) share(w http.ResponseWriter, r *http.Request, host string) {
	token := strings.TrimPrefix(r.URL.Path, entity.PreviewSharePath)
	if token == "" || strings.Contains(token, "/") {
		e.missing(w, r)

		return
	}

	passcode := ""

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			e.passcode(w, r, http.StatusBadRequest, "That form did not arrive whole.")

			return
		}

		passcode = r.PostForm.Get("passcode")
	}

	reply, err := e.proxies.Redeem(r.Context(), host, token, passcode)
	if err != nil {
		e.declined(w, r, host, err)

		return
	}

	e.admit(w, r, reply)
}

func (e *Edge) declined(w http.ResponseWriter, r *http.Request, host string, err error) {
	switch {
	case errors.Is(err, entity.ErrPreviewSharePasscodeNeeded):
		e.passcode(w, r, http.StatusOK, "")
	case errors.Is(err, entity.ErrPreviewSharePasscode):
		e.passcode(w, r, http.StatusForbidden, "That is not the passcode on this link.")
	case errors.Is(err, entity.ErrPreviewShareGuessed):
		e.guessed(w, r)
	case errors.Is(err, entity.ErrPreviewShareNotFound):
		e.missing(w, r)
	case errors.Is(err, entity.ErrPreviewShareExpired):
		e.withdrawn(w, r)
	default:
		e.refused(w, r, host, err)
	}
}
