package directory_test

import (
	"context"

	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	eventsvc "github.com/usenorn/norn/internal/service/event"
	webhooksvc "github.com/usenorn/norn/internal/service/webhook"
)

func capturingEmitter(
	ctrl *gomock.Controller,
	into *[]entity.WebhookOutboxEntry,
) *webhooksvc.MockWebhookEmitter {
	emitter := webhooksvc.NewMockWebhookEmitter(ctrl)
	emitter.EXPECT().
		Emit(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, entry entity.WebhookOutboxEntry) error {
			*into = append(*into, entry)

			return nil
		}).
		AnyTimes()

	return emitter
}

func silentEvents(ctrl *gomock.Controller) *eventsvc.MockEvents {
	events := eventsvc.NewMockEvents(ctrl)
	events.EXPECT().Publish(gomock.Any(), gomock.Any()).AnyTimes()

	return events
}
