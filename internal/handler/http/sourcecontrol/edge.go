package sourcecontrol

import (
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/service"
)

const DeliveryPath = "/v1/source-control/{provider}/{connectionId}"

type Edge struct {
	sync  service.SourceControlSync
	limit int64
}

func New(sync service.SourceControlSync, cfg config.SourceControl) *Edge {
	return &Edge{sync: sync, limit: cfg.MaxDeliveryBytes}
}

// Deliver reads the body once, whole, before anything else looks at it. GitHub signs the
// exact bytes it sent, so a payload that has been through a decoder is no longer the payload
// that was signed and the mismatch would read as a wrong secret. The cap is this route's
// own: the dashboard's is sized for a form, and a forge sends what it likes.
func (e *Edge) Deliver(w http.ResponseWriter, r *http.Request) {
	provider := entity.SCMProvider(chi.URLParam(r, "provider"))

	connectionID, err := uuid.Parse(chi.URLParam(r, "connectionId"))
	if err != nil {
		e.refuse(w)

		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, e.limit))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			// Saying so plainly matters: refused as a bad signature this reads as a broken
			// secret, and somebody would rotate a credential that was never the problem.
			http.Error(
				w,
				"the delivery is larger than this instance accepts",
				http.StatusRequestEntityTooLarge,
			)

			return
		}

		e.refuse(w)

		return
	}

	deliveryID, err := e.sync.Accept(r.Context(), connectionID, provider, r.Header, body)

	switch {
	case errors.Is(err, entity.ErrSCMSignatureInvalid),
		errors.Is(err, entity.ErrSCMConnectionNotFound):
		e.refuse(w)

	case errors.Is(err, entity.ErrSCMDeliveryDuplicate):
		// The forge is redelivering something already held. Answering success is what stops
		// it retrying, and the stored copy is what stops it being applied twice.
		w.WriteHeader(http.StatusOK)

	case err != nil:
		logging.From(r.Context()).ErrorContext(
			r.Context(),
			"accepting a source control delivery failed",
			"connection_id", connectionID.String(),
			"error", err.Error(),
		)

		http.Error(w, "the delivery could not be stored", http.StatusInternalServerError)

	default:
		logging.From(r.Context()).InfoContext(
			r.Context(),
			"source control delivery accepted",
			"connection_id", connectionID.String(),
			"delivery_id", deliveryID.String(),
		)

		w.WriteHeader(http.StatusAccepted)
	}
}

// refuse answers a connection that does not exist and a delivery that did not verify with
// exactly the same words, so this endpoint cannot be asked which connections an instance
// holds.
func (e *Edge) refuse(w http.ResponseWriter) {
	http.Error(w, "the delivery did not verify", http.StatusUnauthorized)
}
