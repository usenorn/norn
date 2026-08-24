package execution_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func (h *harness) offered(t *testing.T) channelv1.Offer {
	t.Helper()

	message, sent := h.sent(entity.ChannelExecutionOffer)
	if !sent {
		t.Fatal("nothing was offered to the machine, so the delegation would sit forever")
	}

	var offer channelv1.Offer

	if err := json.Unmarshal(message.Payload, &offer); err != nil {
		t.Fatalf("read the offer: %v", err)
	}

	return offer
}

func (h *harness) delegate(t *testing.T) {
	t.Helper()

	h.runners.EXPECT().
		ListByAgentID(gomock.Any(), h.runner.AgentID).
		Return([]entity.Runner{h.runner}, nil)
	h.live(h.runner, 2, 0)

	h.opening(1)

	if err := h.service.OnDelegated(context.Background(), h.issue, entity.IssueDelegation{
		ID:      uuid.New(),
		IssueID: h.issue.ID,
		AgentID: h.runner.AgentID,
	}); err != nil {
		t.Fatalf("delegate: %v", err)
	}
}

func TestTheOfferCarriesTheBranchNornGaveTheIssue(t *testing.T) {
	h := newHarness(t)
	h.delegate(t)

	if branch := h.offered(t).Branch; branch != "rae/norn-1-a-run" {
		t.Fatalf(
			"the offer named branch %q, so the machine would fall back to inventing one and "+
				"the two halves of the product would disagree about what the branch is called",
			branch,
		)
	}
}

func TestAnOfferStillGoesOutWhenNornCannotNameTheBranch(t *testing.T) {
	h := newHarness(t)

	h.branch = ""
	h.branchFails = errors.New("no source control here")

	h.delegate(t)

	if branch := h.offered(t).Branch; branch != "" {
		t.Fatalf("the offer named branch %q, want none", branch)
	}
}
