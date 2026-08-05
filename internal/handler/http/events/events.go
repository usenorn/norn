package events

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/middleware"
	"github.com/usenorn/norn/internal/handler/http/v1/dashboard"
	"github.com/usenorn/norn/internal/service"
)

const Path = "/v1/workspaces/{workspaceId}/events"

type Edge struct {
	events service.Events
	cfg    config.Realtime
}

func New(events service.Events, cfg config.Realtime) *Edge {
	return &Edge{events: events, cfg: cfg}
}

// Serve streams a workspace's changes for as long as the client holds the connection. Everything
// that can fail is decided before the first byte: once the event-stream header is out, the panic
// and problem writers would append a JSON document into the middle of the stream rather than
// replace it.
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

	subscription, err := e.events.Subscribe(r.Context(), service.SubscribeInput{
		WorkspaceID:  workspaceID,
		Subscription: entity.ParseEventSubscription(r.URL.Query().Get("topics"), issueID),
		Cursor:       r.Header.Get("Last-Event-ID"),
	})
	if err != nil {
		status, detail := problem(err)
		middleware.WriteProblem(w, r, status, detail)

		return
	}

	defer subscription.Close()

	stream := http.NewResponseController(w)

	// The server sets a thirty second write deadline for ordinary responses; a stream that keeps
	// it would be cut mid-sentence on its first quiet minute.
	_ = stream.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// A comment primes the stream. Both Go's own buffer and an intermediary proxy hold a response
	// that has a header but no body, so without a first byte a client on a quiet workspace never
	// sees the connection open and sits reporting itself as still connecting.
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

// translate turns the entity the service published into the shape this API already returns, so a
// client patches an event with the same vocabulary it read the screen with.
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
