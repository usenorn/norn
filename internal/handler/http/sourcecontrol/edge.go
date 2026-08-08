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

const (
	DeliveryPath = "/v1/source-control/{provider}/{repositoryId}"

	AppDeliveryBase = "/v1/source-control/github-app"
	AppDeliveryPath = AppDeliveryBase + "/deliveries"
)

type Edge struct {
	sync  service.SourceControlSync
	limit int64
}

func New(sync service.SourceControlSync, cfg config.SourceControl) *Edge {
	return &Edge{sync: sync, limit: cfg.MaxDeliveryBytes}
}

func (e *Edge) Deliver(w http.ResponseWriter, r *http.Request) {
	provider := entity.SCMProvider(chi.URLParam(r, "provider"))

	repositoryID, err := uuid.Parse(chi.URLParam(r, "repositoryId"))
	if err != nil {
		e.refuse(w)

		return
	}

	body, ok := e.read(w, r)
	if !ok {
		return
	}

	deliveryID, err := e.sync.Accept(r.Context(), repositoryID, provider, r.Header, body)

	e.settle(w, r, repositoryID.String(), deliveryID, err)
}

func (e *Edge) DeliverToApp(w http.ResponseWriter, r *http.Request) {
	body, ok := e.read(w, r)
	if !ok {
		return
	}

	deliveryID, err := e.sync.AcceptFromApp(
		r.Context(), entity.SCMProviderGitHub, r.Header, body,
	)

	e.settle(w, r, "", deliveryID, err)
}

func (e *Edge) read(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, e.limit))
	if err == nil {
		return body, true
	}

	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		http.Error(
			w,
			"the delivery is larger than this instance accepts",
			http.StatusRequestEntityTooLarge,
		)

		return nil, false
	}

	e.refuse(w)

	return nil, false
}

func (e *Edge) settle(
	w http.ResponseWriter,
	r *http.Request,
	repository string,
	deliveryID uuid.UUID,
	err error,
) {
	switch {
	case errors.Is(err, entity.ErrSCMSignatureInvalid),
		errors.Is(err, entity.ErrSCMConnectionNotFound),
		errors.Is(err, entity.ErrSCMRepositoryNotFound),
		errors.Is(err, entity.ErrSCMAppNotFound):
		e.refuse(w)

	case errors.Is(err, entity.ErrSCMDeliveryDuplicate):
		w.WriteHeader(http.StatusOK)

	case err != nil:
		logging.From(r.Context()).ErrorContext(
			r.Context(),
			"accepting a source control delivery failed",
			"repository_id", repository,
			"error", err.Error(),
		)

		http.Error(w, "the delivery could not be stored", http.StatusInternalServerError)

	default:
		logging.From(r.Context()).InfoContext(
			r.Context(),
			"source control delivery accepted",
			"repository_id", repository,
			"delivery_id", deliveryID.String(),
		)

		w.WriteHeader(http.StatusAccepted)
	}
}

func (e *Edge) refuse(w http.ResponseWriter) {
	http.Error(w, "the delivery did not verify", http.StatusUnauthorized)
}
