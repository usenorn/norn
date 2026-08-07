package events_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/events"
	"github.com/usenorn/norn/internal/pkg/identity"
	eventsvc "github.com/usenorn/norn/internal/service/event"
)

const maxStreams = 2

func stream(t *testing.T, edge *events.Edge, accountID uuid.UUID) (*httptest.ResponseRecorder, func()) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	ctx = identity.WithActor(ctx, entity.Actor{Kind: entity.ActorKindUser, AccountID: accountID})

	request := httptest.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"/v1/workspaces/"+uuid.NewString()+"/events",
		nil,
	)
	request.SetPathValue("workspaceId", uuid.NewString())

	recorder := httptest.NewRecorder()

	var running sync.WaitGroup

	running.Add(1)

	go func() {
		defer running.Done()

		edge.Serve(recorder, request)
	}()

	return recorder, func() {
		cancel()
		running.Wait()
	}
}

func TestOneAccountCannotHoldEveryLiveStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	service := eventsvc.NewMockEvents(ctrl)
	holding := make(chan struct{}, maxStreams)

	service.EXPECT().
		Subscribe(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, _ any) (any, error) {
			holding <- struct{}{}
			<-ctx.Done()

			return nil, ctx.Err()
		}).
		AnyTimes()

	edge := events.New(service, config.Realtime{Enabled: true, MaxPerAccount: maxStreams})
	accountID := uuid.New()

	stops := make([]func(), 0, maxStreams)

	defer func() {
		for _, stop := range stops {
			stop()
		}
	}()

	for range maxStreams {
		_, stop := stream(t, edge, accountID)
		stops = append(stops, stop)

		<-holding
	}

	refused, stop := stream(t, edge, accountID)
	stop()

	if refused.Code != http.StatusTooManyRequests {
		t.Fatalf(
			"opening one stream past the cap answered %d, want %d. Each stream holds a goroutine "+
				"and a subscription for as long as it stays open, so one account must not be able "+
				"to take the whole instance.",
			refused.Code, http.StatusTooManyRequests,
		)
	}
}
