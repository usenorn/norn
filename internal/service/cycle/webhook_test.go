package cycle_test

import (
	"go.uber.org/mock/gomock"

	webhooksvc "github.com/usenorn/norn/internal/service/webhook"
)

func silentEmitter(ctrl *gomock.Controller) *webhooksvc.MockWebhookEmitter {
	emitter := webhooksvc.NewMockWebhookEmitter(ctrl)
	emitter.EXPECT().Emit(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	return emitter
}
