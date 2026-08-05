package workspace_test

import (
	"go.uber.org/mock/gomock"

	auditsvc "github.com/usenorn/norn/internal/service/audit"
)

func silentAudit(ctrl *gomock.Controller) *auditsvc.MockAudit {
	recorder := auditsvc.NewMockAudit(ctrl)
	recorder.EXPECT().Record(gomock.Any(), gomock.Any()).AnyTimes()

	return recorder
}
