package triage_test

import (
	"context"

	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
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
