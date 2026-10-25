package cycle_test

import (
	"go.uber.org/mock/gomock"

	memoryevidencesvc "github.com/usenorn/norn/internal/service/memoryevidence"
	webhooksvc "github.com/usenorn/norn/internal/service/webhook"
)

func silentEmitter(ctrl *gomock.Controller) *webhooksvc.MockWebhookEmitter {
	emitter := webhooksvc.NewMockWebhookEmitter(ctrl)
	emitter.EXPECT().Emit(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	return emitter
}

func uncheckedEvidence(ctrl *gomock.Controller) *memoryevidencesvc.MockMemoryEvidenceChecks {
	evidence := memoryevidencesvc.NewMockMemoryEvidenceChecks(ctrl)
	evidence.EXPECT().Schedule(gomock.Any(), gomock.Any()).AnyTimes()

	return evidence
}
