package inboundmail

import (
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/service"
)

const DeliveryPath = "/v1/inbound-mail/deliveries"

type Edge struct {
	intakes service.Intakes
	limit   int64
	enabled bool
}

func New(intakes service.Intakes, intake config.Intake, provider config.Epostix) *Edge {
	return &Edge{
		intakes: intakes,
		limit:   provider.MaxDeliverySize,
		enabled: intake.Domain != "" && provider.Configured(),
	}
}

func (e *Edge) Deliver(w http.ResponseWriter, r *http.Request) {
	if !e.enabled {
		http.Error(w, "this instance does not take mail", http.StatusNotFound)

		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, e.limit))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
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

	deliveryID, err := e.intakes.Accept(r.Context(), r.Header, body)

	e.settle(w, r, deliveryID, err)
}

// A provider signs every event it sends, so only a failed signature is worth refusing.
// Mail for an address nobody claims is ordinary traffic on a shared inbound domain, and
// refusing it would paint the provider's delivery history red for something Norn chose.
func (e *Edge) settle(w http.ResponseWriter, r *http.Request, deliveryID uuid.UUID, err error) {
	switch {
	case errors.Is(err, entity.ErrIntakeSignatureInvalid):
		logging.From(r.Context()).WarnContext(r.Context(), "an inbound mail delivery did not verify")

		e.refuse(w)

	case errors.Is(err, entity.ErrIntakeAddressUnknown),
		errors.Is(err, entity.ErrIntakeUnroutable),
		errors.Is(err, entity.ErrIntakeDeliveryDuplicate):
		logging.From(r.Context()).InfoContext(
			r.Context(),
			"an inbound mail delivery was accepted and ignored",
			"reason", err.Error(),
		)

		w.WriteHeader(http.StatusOK)

	case err != nil:
		logging.From(r.Context()).ErrorContext(
			r.Context(),
			"accepting an inbound mail delivery failed",
			"error", err.Error(),
		)

		http.Error(w, "the delivery could not be stored", http.StatusInternalServerError)

	case deliveryID == uuid.Nil:
		w.WriteHeader(http.StatusOK)

	default:
		logging.From(r.Context()).InfoContext(
			r.Context(),
			"inbound mail delivery accepted",
			"delivery_id", deliveryID.String(),
		)

		w.WriteHeader(http.StatusAccepted)
	}
}

func (e *Edge) refuse(w http.ResponseWriter) {
	http.Error(w, "the delivery did not verify", http.StatusUnauthorized)
}
