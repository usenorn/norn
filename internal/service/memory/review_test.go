package memory_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (h *harness) actAsMember(accountID uuid.UUID) {
	h.actAs(entity.Actor{Kind: entity.ActorKindUser, AccountID: accountID}, entity.MembershipRoleMember)
}

func (h *harness) notifiedTargets(memoryID uuid.UUID) []uuid.UUID {
	var targets []uuid.UUID

	for _, event := range h.notified {
		if event.Subject == entity.NotifyMemory(memoryID) && event.Kind == entity.NotificationKindMemoryProposed {
			targets = append(targets, event.Target)
		}
	}

	return targets
}

func TestAnAgentsProjectDecisionWaitsForTheLeadAndTheAdmins(t *testing.T) {
	h := newHarness(t)
	h.actAsAgent(entity.AgentScopeMember)
	agent := h.actor

	proposal := h.create(t, "Ship the API behind a flag", entity.ProjectAudience(h.projectID))

	if proposal.Status != entity.MemoryStatusProposed {
		t.Fatalf("an agent's project memory is %s, want proposed", proposal.Status)
	}

	targets := h.notifiedTargets(proposal.ID)
	if len(targets) != 2 || !slices.Contains(targets, h.leadID) || !slices.Contains(targets, h.accountID) {
		t.Fatalf("notified %v, want the project lead %s and the admin %s", targets, h.leadID, h.accountID)
	}

	for _, event := range h.notified {
		if event.Actor != agent.AccountID || event.ActorKind != entity.ActorKindAgent {
			t.Fatalf("the proposal was announced by %s (%s), want the agent", event.Actor, event.ActorKind)
		}
	}

	h.actAsMember(h.leadID)

	accepted, err := h.service.Accept(context.Background(), h.workspaceID, proposal.ID, service.AcceptMemoryInput{
		ExpectedVersion: proposal.Version,
	})
	if err != nil {
		t.Fatalf("the lead accepting: %v", err)
	}

	if accepted.Status != entity.MemoryStatusAccepted {
		t.Fatalf("after the lead accepted, the memory is %s", accepted.Status)
	}

	revisions := h.revisionsOf(proposal.ID)
	last := revisions[len(revisions)-1]

	if last.Action != entity.MemoryRevisionAccepted || last.By.AccountID != h.leadID || last.By.Source != entity.RevisionSourcePerson {
		t.Fatalf("the acceptance was recorded as %s by %s (%s), want accepted by the lead", last.Action, last.By.AccountID, last.By.Source)
	}
}

func TestAMembersWorkspaceMemoryWaitsForTheAdmins(t *testing.T) {
	h := newHarness(t)
	member := uuid.New()
	h.actAsMember(member)

	proposal := h.create(t, "Releases need two approvals", entity.WorkspaceAudience())

	if proposal.Status != entity.MemoryStatusProposed {
		t.Fatalf("a member's workspace memory is %s, want proposed", proposal.Status)
	}

	if targets := h.notifiedTargets(proposal.ID); !slices.Equal(targets, []uuid.UUID{h.accountID}) {
		t.Fatalf("notified %v, want only the admin %s", targets, h.accountID)
	}
}

func TestPersonalMemoriesAndAReviewersOwnNeverWaitForReview(t *testing.T) {
	for name, testCase := range map[string]struct {
		act      func(h *harness)
		audience func(h *harness) entity.KnowledgeAudience
	}{
		"an agent's personal memory": {
			act:      func(h *harness) { h.actAsAgent(entity.AgentScopeMember) },
			audience: func(*harness) entity.KnowledgeAudience { return entity.PersonalAudience() },
		},
		"the lead's project memory": {
			act:      func(h *harness) { h.actAsMember(h.leadID) },
			audience: func(h *harness) entity.KnowledgeAudience { return entity.ProjectAudience(h.projectID) },
		},
		"an admin's workspace memory": {
			act:      func(*harness) {},
			audience: func(*harness) entity.KnowledgeAudience { return entity.WorkspaceAudience() },
		},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			testCase.act(h)

			memory := h.create(t, "Prefer small pull requests", testCase.audience(h))

			if memory.Status != entity.MemoryStatusAccepted || len(h.notified) != 0 {
				t.Fatalf("got %s with %d notifications, want accepted with none", memory.Status, len(h.notified))
			}
		})
	}
}

func TestOnlyAReviewingPersonMayAcceptOrRejectAProposal(t *testing.T) {
	h := newHarness(t)
	author := uuid.New()
	h.actAsMember(author)
	proposal := h.create(t, "Ship the API behind a flag", entity.ProjectAudience(h.projectID))

	for name, act := range map[string]func(){
		"its author":        func() { h.actAsMember(author) },
		"another member":    func() { h.actAsMember(uuid.New()) },
		"an admin's agent":  func() { h.actAsAgent(entity.AgentScopeMember); h.reader.Identity.Role = entity.MembershipRoleAdmin },
		"a workspace agent": func() { h.actAsAgent(entity.AgentScopeWorkspace) },
	} {
		act()

		_, accepting := h.service.Accept(context.Background(), h.workspaceID, proposal.ID, service.AcceptMemoryInput{
			ExpectedVersion: proposal.Version,
		})
		_, rejecting := h.service.Reject(context.Background(), h.workspaceID, proposal.ID, service.RejectMemoryInput{
			ExpectedVersion: proposal.Version, Reason: "not agreed",
		})

		for _, err := range []error{accepting, rejecting} {
			if !errors.Is(err, entity.ErrMemoryReviewForbidden) && !errors.Is(err, entity.ErrKnowledgeNotEditable) {
				t.Errorf("%s reviewing got %v, want a refusal", name, err)
			}
		}
	}

	if stored := h.stored[proposal.ID]; stored.Status != entity.MemoryStatusProposed || stored.Version != proposal.Version {
		t.Fatalf("refused reviews changed the proposal to %s at version %d", stored.Status, stored.Version)
	}
}

func TestEditingAndAcceptingRecordsTheCorrectionAndTheAcceptance(t *testing.T) {
	h := newHarness(t)
	h.actAsAgent(entity.AgentScopeMember)
	proposal := h.create(t, "Ship the API behind a flag", entity.WorkspaceAudience())
	h.actAs(entity.Actor{Kind: entity.ActorKindUser, AccountID: h.accountID}, entity.MembershipRoleAdmin)

	edited := content("Ship the public API behind a flag")

	accepted, err := h.service.Accept(context.Background(), h.workspaceID, proposal.ID, service.AcceptMemoryInput{
		ExpectedVersion: proposal.Version,
		Content:         &edited,
	})
	if err != nil {
		t.Fatalf("edit and accept: %v", err)
	}

	if accepted.Status != entity.MemoryStatusAccepted || accepted.Statement != "Ship the public API behind a flag" || accepted.Version != 3 {
		t.Fatalf("got %s %q at version %d, want the edited statement accepted at version 3",
			accepted.Status, accepted.Statement, accepted.Version)
	}

	var actions []entity.MemoryRevisionAction
	for _, revision := range h.revisionsOf(proposal.ID) {
		actions = append(actions, revision.Action)
	}

	want := []entity.MemoryRevisionAction{entity.MemoryRevisionCreated, entity.MemoryRevisionCorrected, entity.MemoryRevisionAccepted}
	if !slices.Equal(actions, want) {
		t.Fatalf("history is %v, want %v", actions, want)
	}

	if revisions := h.revisionsOf(proposal.ID); revisions[1].Status != entity.MemoryStatusProposed || revisions[1].By.AccountID != h.accountID {
		t.Fatalf("the correction was recorded as %s by %s, want proposed by the admin", revisions[1].Status, revisions[1].By.AccountID)
	}
}

func TestARejectionNeedsAReasonAndClosesTheProposal(t *testing.T) {
	h := newHarness(t)
	h.actAsAgent(entity.AgentScopeMember)
	proposal := h.create(t, "Deploy on Fridays", entity.ProjectAudience(h.projectID))
	h.actAsMember(h.leadID)

	_, err := h.service.Reject(context.Background(), h.workspaceID, proposal.ID, service.RejectMemoryInput{
		ExpectedVersion: proposal.Version, Reason: "  ",
	})

	var invalid entity.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("a blank reason got %v, want a validation error", err)
	}

	rejected, err := h.service.Reject(context.Background(), h.workspaceID, proposal.ID, service.RejectMemoryInput{
		ExpectedVersion: proposal.Version, Reason: " contradicts the release freeze ",
	})
	if err != nil {
		t.Fatalf("reject: %v", err)
	}

	if rejected.Status != entity.MemoryStatusRejected {
		t.Fatalf("the rejected memory is %s", rejected.Status)
	}

	revisions := h.revisionsOf(proposal.ID)
	last := revisions[len(revisions)-1]

	if last.Action != entity.MemoryRevisionRejected || last.Reason != "contradicts the release freeze" || last.By.AccountID != h.leadID {
		t.Fatalf("the rejection was recorded as %s %q by %s", last.Action, last.Reason, last.By.AccountID)
	}

	_, err = h.service.Accept(context.Background(), h.workspaceID, proposal.ID, service.AcceptMemoryInput{
		ExpectedVersion: rejected.Version,
	})
	if !errors.Is(err, entity.ErrMemoryClosed) {
		t.Fatalf("accepting a rejected memory got %v, want ErrMemoryClosed", err)
	}
}

func TestAReviewAgainstAStaleVersionIsRefused(t *testing.T) {
	h := newHarness(t)
	h.actAsAgent(entity.AgentScopeMember)
	proposal := h.create(t, "Deploy on Fridays", entity.WorkspaceAudience())
	h.actAs(entity.Actor{Kind: entity.ActorKindUser, AccountID: h.accountID}, entity.MembershipRoleAdmin)

	_, err := h.service.Accept(context.Background(), h.workspaceID, proposal.ID, service.AcceptMemoryInput{
		ExpectedVersion: proposal.Version + 1,
	})
	if !errors.Is(err, entity.ErrMemoryStale) {
		t.Fatalf("a stale acceptance got %v, want ErrMemoryStale", err)
	}
}

func TestAnAcceptedMemoryIsNoLongerReviewed(t *testing.T) {
	h := newHarness(t)
	memory := h.create(t, "Deploy on Tuesdays", entity.WorkspaceAudience())

	_, err := h.service.Reject(context.Background(), h.workspaceID, memory.ID, service.RejectMemoryInput{
		ExpectedVersion: memory.Version, Reason: "changed my mind",
	})
	if !errors.Is(err, entity.ErrMemoryNotPending) {
		t.Fatalf("rejecting an accepted memory got %v, want ErrMemoryNotPending", err)
	}
}

func TestAnAuthorsReconciliationLeavesTheProposalForReview(t *testing.T) {
	h := newHarness(t)
	existing := h.create(t, "Production uses PostgreSQL", entity.WorkspaceAudience())
	author := uuid.New()
	h.actAsMember(author)

	written, err := h.service.Create(context.Background(), h.workspaceID, service.CreateMemoryInput{
		MemoryContent: content("Production uses PostgreSQL"),
		Audience:      entity.WorkspaceAudience(),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	proposal := written.Memory

	reconciled, err := h.service.Reconcile(context.Background(), h.workspaceID, proposal.ID, service.ReconcileMemoryInput{
		Resolution:      entity.MemoryResolutionIndependent,
		ExpectedVersion: proposal.Version,
	})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if reconciled.Memory.Status != entity.MemoryStatusProposed {
		t.Fatalf("the author's independent reconciliation moved the proposal to %s", reconciled.Memory.Status)
	}

	revisions := h.revisionsOf(proposal.ID)
	if last := revisions[len(revisions)-1]; last.Action != entity.MemoryRevisionReconciled {
		t.Fatalf("the author's reconciliation was recorded as %s, want reconciled", last.Action)
	}

	if h.stored[existing.ID].Status != entity.MemoryStatusAccepted {
		t.Fatalf("the existing memory became %s", h.stored[existing.ID].Status)
	}
}

func TestAnAuthorsSupersessionWaitsForAcceptance(t *testing.T) {
	h := newHarness(t)
	h.actAsMember(uuid.New())
	existing := h.create(t, "Production uses PostgreSQL", entity.WorkspaceAudience())
	h.actAs(entity.Actor{Kind: entity.ActorKindUser, AccountID: h.accountID}, entity.MembershipRoleAdmin)

	existing, err := h.service.Accept(context.Background(), h.workspaceID, existing.ID, service.AcceptMemoryInput{
		ExpectedVersion: existing.Version,
	})
	if err != nil {
		t.Fatalf("accept the existing memory: %v", err)
	}

	h.actAsMember(existing.AuthorID)

	written, err := h.service.Create(context.Background(), h.workspaceID, service.CreateMemoryInput{
		MemoryContent: content("Production uses PostgreSQL 18"),
		Audience:      entity.WorkspaceAudience(),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	proposal := written.Memory

	reconciled, err := h.service.Reconcile(context.Background(), h.workspaceID, proposal.ID, service.ReconcileMemoryInput{
		Resolution:      entity.MemoryResolutionSupersede,
		ExpectedVersion: proposal.Version,
		Target:          &service.MemoryTarget{ID: existing.ID, ExpectedVersion: existing.Version},
	})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if reconciled.Memory.Status != entity.MemoryStatusProposed || reconciled.Memory.SupersedesID == nil ||
		h.stored[existing.ID].Status != entity.MemoryStatusAccepted {
		t.Fatalf("the author's supersession applied before review: proposal %s, existing %s",
			reconciled.Memory.Status, h.stored[existing.ID].Status)
	}

	h.actAs(entity.Actor{Kind: entity.ActorKindUser, AccountID: h.accountID}, entity.MembershipRoleAdmin)

	if _, err := h.service.Accept(context.Background(), h.workspaceID, proposal.ID, service.AcceptMemoryInput{
		ExpectedVersion: reconciled.Memory.Version,
	}); err != nil {
		t.Fatalf("accept the proposal: %v", err)
	}

	if old := h.stored[existing.ID]; old.Status != entity.MemoryStatusSuperseded || *old.SupersededByID != proposal.ID {
		t.Fatalf("after acceptance the existing memory is %s", old.Status)
	}
}

func TestResharingIntoASharedAudienceSendsTheMemoryBackForReview(t *testing.T) {
	h := newHarness(t)
	author := uuid.New()
	h.actAsMember(author)
	memory := h.create(t, "Prefer small pull requests", entity.PersonalAudience())

	reshared, err := h.service.Reshare(context.Background(), h.workspaceID, memory.ID, service.ReshareMemoryInput{
		ExpectedVersion: memory.Version,
		Audience:        entity.ProjectAudience(h.projectID),
	})
	if err != nil {
		t.Fatalf("reshare: %v", err)
	}

	if reshared.Status != entity.MemoryStatusProposed {
		t.Fatalf("a member's reshare into a project is %s, want proposed", reshared.Status)
	}

	if targets := h.notifiedTargets(memory.ID); len(targets) != 2 || !slices.Contains(targets, h.leadID) {
		t.Fatalf("notified %v, want the lead and the admin", targets)
	}
}
