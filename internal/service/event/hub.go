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
