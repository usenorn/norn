package dashboard

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/service"
	cyclesvc "github.com/usenorn/norn/internal/service/cycle"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func TestAnEmptyReviewedSetStillAsksTheServiceToCheckTheMembership(t *testing.T) {
	ctrl := gomock.NewController(t)
	cycles := cyclesvc.NewMockCycles(ctrl)

	var received service.CloseCycleInput

	cycles.EXPECT().
		Close(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _, _ uuid.UUID, input service.CloseCycleInput,
		) (service.CycleView, error) {
			received = input

			return service.CycleView{}, nil
		})

	h := &handler{cycles: cycles}
	rollover := api.Backlog
	reviewed := []uuid.UUID{}

	if _, err := h.CloseWorkspaceCycle(context.Background(), api.CloseWorkspaceCycleRequestObject{
		WorkspaceId: uuid.New(),
		CycleId:     uuid.New(),
		Body: &api.CloseCycleRequest{
			Rollover:         &rollover,
			ReviewedIssueIds: &reviewed,
		},
	}); err != nil {
		t.Fatalf("CloseWorkspaceCycle: %v", err)
	}

	if received.Reviewed == nil {
		t.Fatal(
			"a form that reviewed no unfinished issues arrived as an absent set, which turns the " +
				"membership check off. A cycle that gained work since the form opened would then " +
				"close on the default decision instead of reporting a conflict.",
		)
	}
}
