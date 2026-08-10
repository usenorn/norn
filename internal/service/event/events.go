package event

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type eventsService struct {
	stream     repository.EventStream
	authorizer service.Authorizer

	mu   sync.Mutex
	hubs map[uuid.UUID]*hub
}

func New(stream repository.EventStream, authorizer service.Authorizer) service.Events {
	return &eventsService{
		stream:     stream,
		authorizer: authorizer,
		hubs:       map[uuid.UUID]*hub{},
	}
}

func (s *eventsService) Publish(ctx context.Context, event entity.Event) {
	if err := s.stream.Publish(ctx, event); err != nil {
		logging.From(ctx).ErrorContext(
			ctx,
			"publishing a change to subscribers failed",
			"workspace_id", event.WorkspaceID.String(),
			"kind", string(event.Kind),
			"error", err.Error(),
		)
	}
}

func (s *eventsService) Subscribe(
	ctx context.Context,
	input service.SubscribeInput,
) (*service.Subscription, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionRead,
		WorkspaceID: input.WorkspaceID,
		Scoped:      true,
	})
	if err != nil {
		return nil, err
	}

	missed, lapsed, cursor, err := s.catchUp(ctx, input, decision)
	if err != nil {
		return nil, err
	}

	target := &subscriber{
		accountID:    decision.Actor.AccountID,
		subscription: input.Subscription,
		events:       make(chan entity.Event, entity.EventSubscriberBuffer),
		refresh:      make(chan struct{}, 1),
		done:         make(chan struct{}),
	}

	scope := decision.Scope
	target.scope.Store(&scope)

	owner := s.hubFor(ctx, input.WorkspaceID, cursor)
	owner.add(target)

	// Subscribing is bounded so a stalled dependency cannot hold the handshake open forever, but
	// re-scoping outlives that bound and ends with the subscriber instead, the same way the hub
	// detaches from the request that first opened it.
	go s.rescope(context.WithoutCancel(ctx), input.WorkspaceID, target)

	return &service.Subscription{
		Events: target.events,
		Missed: missed,
		Lapsed: lapsed,
		Close: func() {
			if owner.remove(target) == 0 {
				s.retire(input.WorkspaceID, owner)
			}
		},
	}, nil
}

func (s *eventsService) catchUp(
	ctx context.Context,
	input service.SubscribeInput,
	decision entity.Decision,
) ([]entity.Event, bool, string, error) {
	latest, err := s.stream.Latest(ctx, input.WorkspaceID)
	if err != nil {
		return nil, false, "", err
	}

	if input.Cursor == "" {
		return nil, false, latest, nil
	}

	missed, err := s.stream.Since(ctx, input.WorkspaceID, input.Cursor)
	if err != nil {
		if errors.Is(err, entity.ErrEventStreamLapsed) {
			return nil, true, latest, nil
		}

		return nil, false, "", err
	}

	permitted := make([]entity.Event, 0, len(missed))

	for _, event := range missed {
		if event.Reaches(decision.Actor.AccountID, decision.Scope) &&
			input.Subscription.Wants(event) {
			permitted = append(permitted, event)
		}
	}

	cursor := input.Cursor
	if len(missed) > 0 {
		cursor = missed[len(missed)-1].ID
	}

	return permitted, false, cursor, nil
}

func (s *eventsService) rescope(ctx context.Context, workspaceID uuid.UUID, target *subscriber) {
	refresh := time.NewTicker(entity.EventScopeRefresh)
	defer refresh.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-target.done:
			return
		case <-target.refresh:
		case <-refresh.C:
		}

		decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
			Resource:    entity.ResourceIssue,
			Action:      entity.ActionRead,
			WorkspaceID: workspaceID,
			Scoped:      true,
		})
		if err != nil {
			logging.From(ctx).WarnContext(
				ctx,
				"re-resolving a subscriber's team scope failed",
				"workspace_id", workspaceID.String(),
				"error", err.Error(),
			)

			continue
		}

		scope := decision.Scope
		target.scope.Store(&scope)
	}
}

func (s *eventsService) hubFor(ctx context.Context, workspaceID uuid.UUID, cursor string) *hub {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, running := s.hubs[workspaceID]; running {
		return existing
	}

	created := newHub(workspaceID, s.stream)
	created.start(ctx, cursor)
	s.hubs[workspaceID] = created

	return created
}

func (s *eventsService) retire(workspaceID uuid.UUID, owner *hub) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.hubs[workspaceID] != owner {
		return
	}

	delete(s.hubs, workspaceID)
	owner.stop()
}
