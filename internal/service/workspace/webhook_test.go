package workspace_test

import (
	"go.uber.org/mock/gomock"

	eventsvc "github.com/usenorn/norn/internal/service/event"
	webhooksvc "github.com/usenorn/norn/internal/service/webhook"
)

func silentEmitter(ctrl *gomock.Controller) *webhooksvc.MockWebhookEmitter {
	emitter := webhooksvc.NewMockWebhookEmitter(ctrl)
	emitter.EXPECT().Emit(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	return emitter
}

func silentEvents(ctrl *gomock.Controller) *eventsvc.MockEvents {
	events := eventsvc.NewMockEvents(ctrl)
	events.EXPECT().Publish(gomock.Any(), gomock.Any()).AnyTimes()

	return events
}
