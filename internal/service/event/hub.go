package event

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/repository"
)

type subscriber struct {
	accountID    uuid.UUID
	subscription entity.EventSubscription
	scope        atomic.Pointer[entity.TeamScope]
	events       chan entity.Event
	refresh      chan struct{}
	done         chan struct{}
	closeOnce    sync.Once
}

// deliver hands the subscriber one event, or reports that it could not. A subscriber whose buffer
// is full is not waited for: blocking here would let one stalled reader hold up every other reader
// on the workspace, so the caller closes it instead and lets the client resume from its own cursor.
func (s *subscriber) deliver(event entity.Event) bool {
	scope := s.scope.Load()

	if scope == nil {
		return true
	}

	if !event.Reaches(s.accountID, *scope) || !s.subscription.Wants(event) {
		return true
	}

	select {
	case s.events <- event:
		return true
	default:
		return false
	}
}

// rescope drops the cached scope so nothing is delivered until it has been resolved again, then
// asks for that resolution. A membership change can only ever narrow what somebody may see, so the
// gap has to fail closed rather than keep using the scope the change just invalidated.
func (s *subscriber) rescope() {
	s.scope.Store(nil)

	select {
	case s.refresh <- struct{}{}:
	default:
	}
}

func (s *subscriber) close() {
	s.closeOnce.Do(func() {
		close(s.done)
		close(s.events)
	})
}

type hub struct {
	workspaceID uuid.UUID
	stream      repository.EventStream

	mu          sync.Mutex
	subscribers map[*subscriber]struct{}
	stop        context.CancelFunc
	stopped     chan struct{}
}

func newHub(workspaceID uuid.UUID, stream repository.EventStream) *hub {
	return &hub{
		workspaceID: workspaceID,
		stream:      stream,
		subscribers: map[*subscriber]struct{}{},
		stopped:     make(chan struct{}),
	}
}

func (h *hub) start(ctx context.Context, cursor string) {
	reading, cancel := context.WithCancel(context.WithoutCancel(ctx))
	h.stop = cancel

	go h.read(reading, cursor)
}

func (h *hub) read(ctx context.Context, cursor string) {
	defer close(h.stopped)

	for {
		if ctx.Err() != nil {
			return
		}

		events, next, err := h.stream.Read(ctx, h.workspaceID, cursor)
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			logging.From(ctx).ErrorContext(
				ctx,
				"reading the workspace event stream failed",
				"workspace_id", h.workspaceID.String(),
				"error", err.Error(),
			)

			select {
			case <-ctx.Done():
				return
			case <-time.After(entity.EventRetryDelay):
			}

			continue
		}

		cursor = next

		for _, event := range events {
			h.fan(event)
		}
	}
}

func (h *hub) fan(event entity.Event) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for target := range h.subscribers {
		if event.Kind.Rescopes() && event.AccountID == target.accountID {
			target.rescope()
		}

		if !target.deliver(event) {
			delete(h.subscribers, target)
			target.close()
		}
	}
}

func (h *hub) add(target *subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.subscribers[target] = struct{}{}
}

func (h *hub) remove(target *subscriber) int {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, present := h.subscribers[target]; present {
		delete(h.subscribers, target)
		target.close()
	}

	return len(h.subscribers)
}
