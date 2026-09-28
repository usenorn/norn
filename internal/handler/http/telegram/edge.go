package telegram

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
	UpdatePath   = "/v1/telegram/bots/{botId}/updates"
	botParameter = "botId"
	secretHeader = "X-Telegram-Bot-Api-Secret-Token"
)

type Edge struct {
	updates service.TelegramUpdates
	limit   int64
}

func New(updates service.TelegramUpdates, cfg config.Telegram) *Edge {
	return &Edge{updates: updates, limit: cfg.MaxUpdateBytes}
}

func (e *Edge) Deliver(w http.ResponseWriter, r *http.Request) {
	botID, err := uuid.Parse(chi.URLParam(r, botParameter))
	if err != nil {
		w.WriteHeader(http.StatusOK)

		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, e.limit))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			logging.From(r.Context()).WarnContext(r.Context(), "a telegram update was larger than this instance reads")
			w.WriteHeader(http.StatusOK)

			return
		}

		http.Error(w, "the update could not be read", http.StatusBadRequest)

		return
	}

	e.settle(w, r, e.updates.Accept(r.Context(), botID, r.Header.Get(secretHeader), body))
}

// Telegram retries anything but a 2xx for up to a day, so an update for a bot that is gone is
// answered 200: repeating it would change nothing and would hold back every later update.
func (e *Edge) settle(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case err == nil:
		w.WriteHeader(http.StatusOK)

	case errors.Is(err, entity.ErrTelegramSecretInvalid):
		logging.From(r.Context()).WarnContext(r.Context(), "a telegram update did not carry its bot's secret")

		http.Error(w, "the update did not verify", http.StatusUnauthorized)

	case errors.Is(err, entity.ErrTelegramBotNotFound):
		w.WriteHeader(http.StatusOK)

	default:
		logging.From(r.Context()).ErrorContext(r.Context(), "accepting a telegram update failed", "error", err.Error())

		http.Error(w, "the update could not be stored", http.StatusInternalServerError)
	}
}
