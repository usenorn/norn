package events

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/middleware"
	"github.com/usenorn/norn/internal/handler/http/v1/dashboard"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
)

const Path = "/v1/workspaces/{workspaceId}/events"

type Edge struct {
	events service.Events
	cfg    config.Realtime
	open   sync.Map
}

func New(events service.Events, cfg config.Realtime) *Edge {
	return &Edge{events: events, cfg: cfg}
}

func (e *Edge) admit(accountID uuid.UUID) (func(), bool) {
	counter, _ := e.open.LoadOrStore(accountID, &atomic.Int64{})
	streams := counter.(*atomic.Int64)

	if streams.Add(1) > int64(e.cfg.MaxPerAccount) {
		streams.Add(-1)

		return nil, false
	}

	return func() {
		if streams.Add(-1) == 0 {
			e.open.CompareAndDelete(accountID, streams)
		}
	}, true
}

func (e *Edge) Serve(w http.ResponseWriter, r *http.Request) {
	if !e.cfg.Enabled {
		middleware.WriteProblem(
			w, r, http.StatusNotFound, "live updates are not enabled on this instance",
		)

		return
	}

	workspaceID, err := uuid.Parse(r.PathValue("workspaceId"))
	if err != nil {
		middleware.WriteProblem(w, r, http.StatusBadRequest, "workspace id is not a uuid")

		return
	}

	issueID, _ := uuid.Parse(r.URL.Query().Get("issue"))

	actor, signedIn := identity.Actor(r.Context())
	if !signedIn {
		middleware.WriteProblem(w, r, http.StatusUnauthorized, "a valid session is required")

		return
	}

	release, admitted := e.admit(actor.Authority())
	if !admitted {
		middleware.WriteProblem(
			w, r, http.StatusTooManyRequests,
			"too many live update streams are already open for this account",
		)

		return
	}

	defer release()

	handshake, settled := context.WithTimeout(r.Context(), entity.EventHandshakeTimeout)
	started := time.Now()

	subscription, err := e.events.Subscribe(handshake, service.SubscribeInput{
		WorkspaceID:  workspaceID,
		Subscription: entity.ParseEventSubscription(r.URL.Query().Get("topics"), issueID),
		Cursor:       r.Header.Get("Last-Event-ID"),
	})

	settled()

	if err != nil {
		status, detail := problem(err)

		logging.From(r.Context()).ErrorContext(
			r.Context(), "opening a live update stream failed",
			"workspace_id", workspaceID,
			"status", status,
			"waited_ms", time.Since(started).Milliseconds(),
			"error", err,
		)

		middleware.WriteProblem(w, r, status, detail)

		return
	}

	logging.From(r.Context()).InfoContext(
		r.Context(), "live update stream opened",
		"workspace_id", workspaceID,
		"waited_ms", time.Since(started).Milliseconds(),
	)

	defer subscription.Close()

	stream := http.NewResponseController(w)

	_ = stream.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	if _, err := fmt.Fprint(w, ": connected\n\n"); err != nil {
		return
	}

	if err := stream.Flush(); err != nil {
		return
	}

	if subscription.Lapsed {
		if !write(w, stream, "resync", nil, "") {
			return
		}
	}

	for _, missed := range subscription.Missed {
		if !send(w, stream, missed) {
			return
		}
	}

	heartbeat := time.NewTicker(entity.EventHeartbeat)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, open := <-subscription.Events:
			if !open {
				_ = write(w, stream, "resync", nil, "")

				return
			}

			if !send(w, stream, event) {
				return
			}
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				return
			}

			if err := stream.Flush(); err != nil {
				return
			}
		}
	}
}

func send(w http.ResponseWriter, stream *http.ResponseController, event entity.Event) bool {
	payload := translate(event)

	body, err := json.Marshal(map[string]any{
		"kind":    string(event.Kind),
		"issueId": text(event.IssueID),
		"teamId":  text(event.TeamID),
		"actorId": text(event.ActorID),
		"subject": text(event.SubjectID),
		"payload": json.RawMessage(payload),
	})
	if err != nil {
		return true
	}

	return write(w, stream, string(event.Kind), body, event.ID)
}

func write(w http.ResponseWriter, stream *http.ResponseController, kind string, body []byte, id string) bool {
	if id != "" {
		if _, err := fmt.Fprintf(w, "id: %s\n", id); err != nil {
			return false
		}
	}

	if _, err := fmt.Fprintf(w, "event: %s\n", kind); err != nil {
		return false
	}

	if len(body) == 0 {
		body = []byte("{}")
	}

	if _, err := fmt.Fprintf(w, "data: %s\n\n", body); err != nil {
		return false
	}

	return stream.Flush() == nil
}

func translate(event entity.Event) json.RawMessage {
	if len(event.Payload) == 0 {
		return json.RawMessage("{}")
	}

	switch event.Kind {
	case entity.EventIssueCreated, entity.EventIssueUpdated, entity.EventIssueDeleted:
		var issue entity.Issue

		if err := json.Unmarshal(event.Payload, &issue); err != nil {
			return json.RawMessage("{}")
		}

		return marshal(dashboard.IssueEvent(issue))
	case entity.EventCommentPosted, entity.EventCommentEdited, entity.EventCommentDeleted:
		var comment entity.IssueComment

		if err := json.Unmarshal(event.Payload, &comment); err != nil {
			return json.RawMessage("{}")
		}

		return marshal(dashboard.CommentEvent(comment))
	case entity.EventExecutionUpdated:
		var execution entity.Execution

		if err := json.Unmarshal(event.Payload, &execution); err != nil {
			return json.RawMessage("{}")
		}

		return marshal(dashboard.ExecutionEventDTO(execution))
	case entity.EventExecutionEvent:
		var entry entity.ExecutionEvent

		if err := json.Unmarshal(event.Payload, &entry); err != nil {
			return json.RawMessage("{}")
		}

		return marshal(dashboard.ExecutionTimelineEvent(entry))
	default:
		return json.RawMessage("{}")
	}
}

func marshal(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("{}")
	}

	return encoded
}

func text(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}

	return id.String()
}
