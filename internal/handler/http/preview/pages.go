package preview

import (
	"bytes"
	"embed"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/usenorn/norn/internal/observability/logging"
)

//go:embed pages/*.html
var pages embed.FS

var page = template.Must(template.ParseFS(pages, "pages/*.html"))

type view struct {
	Title string
	Lines []string
	Host  string
	Form  string
	Wrong string
}

func (e *Edge) render(w http.ResponseWriter, r *http.Request, status int, shown view) {
	var body bytes.Buffer

	if err := page.ExecuteTemplate(&body, "page", shown); err != nil {
		logging.From(r.Context()).ErrorContext(
			r.Context(), "a gateway page would not render", slog.String("error", err.Error()),
		)

		http.Error(w, shown.Title, status)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.WriteHeader(status)

	_, _ = w.Write(body.Bytes())
}

func (e *Edge) unknown(w http.ResponseWriter, r *http.Request, host string) {
	e.render(w, r, http.StatusNotFound, view{
		Title: "Nothing is served here",
		Lines: []string{
			"norn has no preview at this address. A preview only exists while a machine is " +
				"running the service behind it, and the address stops answering as soon as it is " +
				"closed.",
		},
		Host: host,
	})
}

func (e *Edge) closed(w http.ResponseWriter, r *http.Request, host string) {
	e.render(w, r, http.StatusGone, view{
		Title: "This preview has been closed",
		Lines: []string{
			"The run that was serving it has finished or given its workspace back, so there is " +
				"nothing left to reach. What the run did is still on its branches, and what " +
				"happened is still on its timeline in norn.",
		},
		Host: host,
	})
}

func (e *Edge) offline(w http.ResponseWriter, r *http.Request, host string) {
	e.render(w, r, http.StatusBadGateway, view{
		Title: "That machine is offline",
		Lines: []string{
			"The preview is still open, but the machine running it is not holding a tunnel to " +
				"norn right now. It reconnects on its own; this address starts answering again " +
				"the moment it does.",
		},
		Host: host,
	})
}

func (e *Edge) gone(w http.ResponseWriter, r *http.Request, host string) {
	e.render(w, r, http.StatusBadGateway, view{
		Title: "That machine is no longer running this preview",
		Lines: []string{
			"The machine is connected, but it refused the request: the service behind this " +
				"preview has stopped, or the run has been given back.",
		},
		Host: host,
	})
}

func (e *Edge) crowded(w http.ResponseWriter, r *http.Request, host string) {
	e.render(w, r, http.StatusServiceUnavailable, view{
		Title: "Too much at once",
		Lines: []string{
			"That machine is already carrying as many preview requests as it may. Try again in " +
				"a moment.",
		},
		Host: host,
	})
}

func (e *Edge) unready(w http.ResponseWriter, r *http.Request) {
	e.render(w, r, http.StatusServiceUnavailable, view{
		Title: "This gateway is not ready",
		Lines: []string{
			"It has not been able to trade its credential with norn for an access token, so it " +
				"cannot ask who you are. It keeps trying.",
		},
	})
}

func (e *Edge) unsupported(w http.ResponseWriter, r *http.Request, host string) {
	e.render(w, r, http.StatusNotImplemented, view{
		Title: "Path-mode previews are not served yet",
		Lines: []string{
			"This address names a run rather than one of its previews. Only a preview with its " +
				"own address is routable today.",
		},
		Host: host,
	})
}

func (e *Edge) broken(w http.ResponseWriter, r *http.Request, err error) {
	logging.From(r.Context()).ErrorContext(
		r.Context(), "the gateway could not answer a preview request", slog.String("error", err.Error()),
	)

	e.render(w, r, http.StatusBadGateway, view{
		Title: "Something went wrong",
		Lines: []string{"norn could not answer for this preview. Try again in a moment."},
	})
}

func (e *Edge) expired(w http.ResponseWriter, r *http.Request) {
	e.render(w, r, http.StatusGone, view{
		Title: "That link has already been used",
		Lines: []string{
			"A sign-in hand-off works once. Open the preview again and norn will send you " +
				"through with a fresh one.",
		},
	})
}

func (e *Edge) missing(w http.ResponseWriter, r *http.Request) {
	e.render(w, r, http.StatusNotFound, view{
		Title: "That share link does not work",
		Lines: []string{
			"It may have been withdrawn, or it may never have belonged to this preview.",
		},
	})
}

func (e *Edge) withdrawn(w http.ResponseWriter, r *http.Request) {
	e.render(w, r, http.StatusGone, view{
		Title: "That share link has run out",
		Lines: []string{"It has either expired or been withdrawn by somebody in the workspace."},
	})
}

func (e *Edge) guessed(w http.ResponseWriter, r *http.Request) {
	e.render(w, r, http.StatusTooManyRequests, view{
		Title: "Too many tries",
		Lines: []string{
			"This link has had too many wrong passcodes. Wait a quarter of an hour and try again.",
		},
	})
}

func (e *Edge) passcode(w http.ResponseWriter, r *http.Request, status int, wrong string) {
	e.render(w, r, status, view{
		Title: "This preview is shared with a passcode",
		Lines: []string{"Whoever sent you the link has the passcode that opens it."},
		Form:  r.URL.EscapedPath(),
		Wrong: wrong,
	})
}
