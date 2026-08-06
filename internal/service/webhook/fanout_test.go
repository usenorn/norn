package webhook_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
)

func TestAnOwnerWhoseScopeMissesTheTeamIsSilentlyPassedOver(t *testing.T) {
	h := newHarness(t)

	workspaceID, ownerID := uuid.New(), uuid.New()
	visible, hidden := uuid.New(), uuid.New()

	hook := enabledWebhook(workspaceID, ownerID, entity.WebhookIssueCreated)

	ctx := h.ownersHold(map[uuid.UUID]grant{
		ownerID: {role: entity.MembershipRoleMember, teams: []uuid.UUID{visible}},
	})

	h.membersEverywhere()
	h.subscriptions(hook)
	h.singleEvent(outboxEntry(workspaceID, hidden, entity.WebhookIssueCreated))

	queued := h.capturesQueue()

	fanned, err := h.fanOut.FanOut(ctx)
	if err != nil {
		t.Fatalf(
			"FanOut error = %v, want nil. An event the subscriber may not see is not a failure of "+
				"the fan-out; it is the same silence a request for that issue would get.",
			err,
		)
	}

	if fanned != 0 || len(*queued) != 0 {
		t.Fatalf(
			"an event on a team outside the owner's scope produced %d deliveries. A webhook is a "+
				"standing read, so it must not hand its owner work they cannot open in the app.",
			len(*queued),
		)
	}
}

func TestAnOwnerWhoseMembershipIsGoneReceivesNothing(t *testing.T) {
	h := newHarness(t)

	workspaceID, ownerID := uuid.New(), uuid.New()
	hook := enabledWebhook(workspaceID, ownerID, entity.WebhookIssueCreated)

	ctx := h.ownersHold(map[uuid.UUID]grant{
		ownerID: {role: entity.MembershipRoleAdmin, allTeams: true},
	})

	h.memberships.EXPECT().
		Get(gomock.Any(), workspaceID, ownerID).
		Return(entity.Membership{}, entity.ErrMembershipNotFound)

	h.subscriptions(hook)
	h.singleEvent(outboxEntry(workspaceID, uuid.Nil, entity.WebhookIssueCreated))

	queued := h.capturesQueue()

	fanned, err := h.fanOut.FanOut(ctx)
	if err != nil {
		t.Fatalf("FanOut error = %v, want nil; a removed owner is an expected state, not a fault", err)
	}

	if fanned != 0 || len(*queued) != 0 {
		t.Fatalf(
			"a subscription owned by somebody who has left the workspace still delivered %d "+
				"events. Removing a person must cut off the credentials they left behind.",
			len(*queued),
		)
	}
}

func TestADeactivatedOwnerReceivesNothing(t *testing.T) {
	h := newHarness(t)

	workspaceID, ownerID := uuid.New(), uuid.New()
	hook := enabledWebhook(workspaceID, ownerID, entity.WebhookIssueCreated)
	deactivated := time.Now().UTC().Add(-time.Hour)

	ctx := h.ownersHold(map[uuid.UUID]grant{
		ownerID: {role: entity.MembershipRoleAdmin, allTeams: true},
	})

	h.memberships.EXPECT().
		Get(gomock.Any(), workspaceID, ownerID).
		Return(entity.Membership{
			WorkspaceID:   workspaceID,
			AccountID:     ownerID,
			Role:          entity.MembershipRoleAdmin,
			DeactivatedAt: &deactivated,
		}, nil)

	h.subscriptions(hook)
	h.singleEvent(outboxEntry(workspaceID, uuid.Nil, entity.WebhookIssueCreated))

	queued := h.capturesQueue()

	if _, err := h.fanOut.FanOut(ctx); err != nil {
		t.Fatalf("FanOut: %v", err)
	}

	if len(*queued) != 0 {
		t.Fatalf(
			"a deactivated member's subscription delivered %d events. Directory sync deactivates "+
				"rather than deletes, so the membership row survives and only this check stops it.",
			len(*queued),
		)
	}
}

func TestAnAdministrativeEventFollowsTheOwnersCurrentRole(t *testing.T) {
	for name, probe := range map[string]struct {
		role  entity.MembershipRole
		wants int
	}{
		"a demoted owner stops receiving it": {role: entity.MembershipRoleMember, wants: 0},
		"an administrator receives it":       {role: entity.MembershipRoleAdmin, wants: 1},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)

			workspaceID, ownerID := uuid.New(), uuid.New()
			hook := enabledWebhook(workspaceID, ownerID, entity.WebhookMembershipChanged)

			ctx := h.ownersHold(map[uuid.UUID]grant{
				ownerID: {role: probe.role, allTeams: true},
			})

			h.membersEverywhere()
			h.subscriptions(hook)
			h.singleEvent(outboxEntry(workspaceID, uuid.Nil, entity.WebhookMembershipChanged))

			queued := h.capturesQueue()

			if _, err := h.fanOut.FanOut(ctx); err != nil {
				t.Fatalf("FanOut: %v", err)
			}

			if len(*queued) != probe.wants {
				t.Fatalf(
					"a %s owner received %d membership.changed deliveries, want %d. Who belongs to "+
						"a workspace is administrative, and a subscription registered while its "+
						"owner was an admin must stop the moment they are demoted.",
					probe.role, len(*queued), probe.wants,
				)
			}
		})
	}
}

func TestAProjectEventReachesAnOwnerWhoIsNotAnAdministrator(t *testing.T) {
	h := newHarness(t)

	workspaceID, ownerID := uuid.New(), uuid.New()
	hook := enabledWebhook(workspaceID, ownerID, entity.WebhookProjectUpdated)

	ctx := h.ownersHold(map[uuid.UUID]grant{
		ownerID: {role: entity.MembershipRoleMember, teams: []uuid.UUID{uuid.New()}},
	})

	h.membersEverywhere()
	h.subscriptions(hook)
	h.singleEvent(outboxEntry(workspaceID, uuid.New(), entity.WebhookProjectUpdated))

	queued := h.capturesQueue()

	if _, err := h.fanOut.FanOut(ctx); err != nil {
		t.Fatalf("FanOut: %v", err)
	}

	if len(*queued) != 1 {
		t.Fatalf(
			"a project event produced %d deliveries for an ordinary member, want 1. Projects "+
				"belong to the workspace rather than to one team, so a team scope must not gate "+
				"them or every non-admin subscription goes quiet.",
			len(*queued),
		)
	}
}

func TestADeliveryCarriesTheOutboxBodyUnchangedAndNamesItsEvent(t *testing.T) {
	h := newHarness(t)

	workspaceID, ownerID, teamID := uuid.New(), uuid.New(), uuid.New()
	hook := enabledWebhook(workspaceID, ownerID, entity.WebhookIssueCreated)
	entry := outboxEntry(workspaceID, teamID, entity.WebhookIssueCreated)

	ctx := h.ownersHold(map[uuid.UUID]grant{
		ownerID: {role: entity.MembershipRoleMember, teams: []uuid.UUID{teamID}},
	})

	h.membersEverywhere()
	h.subscriptions(hook)
	h.singleEvent(entry)

	queued := h.capturesQueue()

	fanned, err := h.fanOut.FanOut(ctx)
	if err != nil {
		t.Fatalf("FanOut: %v", err)
	}

	if fanned != 1 || len(*queued) != 1 {
		t.Fatalf("the event produced %d deliveries, want exactly one for the one subscription that covers it", len(*queued))
	}

	delivery := (*queued)[0]

	if !bytes.Equal(delivery.Body, entry.Body) {
		t.Fatalf(
			"the delivery body is %q but the outbox recorded %q. The body is the record of what "+
				"happened at the time; re-deriving or re-encoding it would let a replay send "+
				"something the event never said.",
			delivery.Body, entry.Body,
		)
	}

	if delivery.OutboxID != entry.ID {
		t.Errorf(
			"the delivery names outbox entry %s rather than %s, so nothing ties the delivery back "+
				"to the change that caused it",
			delivery.OutboxID, entry.ID,
		)
	}

	if delivery.WebhookID != hook.ID || delivery.Event != entry.Event || delivery.TeamID != entry.TeamID {
		t.Errorf(
			"the delivery describes webhook %s / event %q / team %s, want %s / %q / %s",
			delivery.WebhookID, delivery.Event, delivery.TeamID, hook.ID, entry.Event, entry.TeamID,
		)
	}

	if len(h.woken) != 1 || h.woken[0].payload.DeliveryID != delivery.ID {
		t.Errorf(
			"the queued delivery woke %d jobs, want one naming it; without it the delivery waits "+
				"for the rescue sweep instead of going out now",
			len(h.woken),
		)
	}
}

func TestEverySubscriptionCoveringAnEventGetsItsOwnDelivery(t *testing.T) {
	h := newHarness(t)

	workspaceID := uuid.New()
	first, second := uuid.New(), uuid.New()

	watching := enabledWebhook(workspaceID, first, entity.WebhookIssueCreated)
	alsoWatching := enabledWebhook(workspaceID, second, entity.WebhookIssueCreated, entity.WebhookIssueDeleted)
	elsewhere := enabledWebhook(workspaceID, first, entity.WebhookCommentPosted)

	ctx := h.ownersHold(map[uuid.UUID]grant{
		first:  {role: entity.MembershipRoleAdmin, allTeams: true},
		second: {role: entity.MembershipRoleAdmin, allTeams: true},
	})

	h.membersEverywhere()
	h.subscriptions(watching, alsoWatching, elsewhere)
	h.singleEvent(outboxEntry(workspaceID, uuid.New(), entity.WebhookIssueCreated))

	queued := h.capturesQueue()

	if _, err := h.fanOut.FanOut(ctx); err != nil {
		t.Fatalf("FanOut: %v", err)
	}

	if len(*queued) != 2 {
		t.Fatalf(
			"two subscriptions to issue.created produced %d deliveries, want 2. One delivery per "+
				"subscription is what lets each receiver retry, fail and be disabled on its own.",
			len(*queued),
		)
	}

	reached := map[uuid.UUID]bool{}

	for _, delivery := range *queued {
		reached[delivery.WebhookID] = true
	}

	if !reached[watching.ID] || !reached[alsoWatching.ID] {
		t.Errorf("the deliveries reached %v rather than both subscriptions to the event", reached)
	}

	if reached[elsewhere.ID] {
		t.Error("a subscription that never asked for issue.created was sent it anyway")
	}
}

func TestFanOutQueuesNothingWhenTheOutboxIsEmpty(t *testing.T) {
	h := newHarness(t)

	ctx := h.ownersHold(nil)

	h.outbox.EXPECT().ClaimPending(gomock.Any(), fanOutBatch).Return(nil, nil)

	fanned, err := h.fanOut.FanOut(ctx)
	if err != nil {
		t.Fatalf("FanOut: %v", err)
	}

	if fanned != 0 || len(h.woken) != 0 {
		t.Errorf("an empty outbox reported %d deliveries and woke %d jobs", fanned, len(h.woken))
	}
}

func TestAnEventWithNoTeamReachesAScopedOwner(t *testing.T) {
	h := newHarness(t)

	workspaceID, ownerID := uuid.New(), uuid.New()
	hook := enabledWebhook(workspaceID, ownerID, entity.WebhookIssueCreated)

	ctx := h.ownersHold(map[uuid.UUID]grant{
		ownerID: {role: entity.MembershipRoleMember, teams: []uuid.UUID{uuid.New()}},
	})

	h.membersEverywhere()
	h.subscriptions(hook)
	h.singleEvent(outboxEntry(workspaceID, uuid.Nil, entity.WebhookIssueCreated))

	queued := h.capturesQueue()

	if _, err := h.fanOut.FanOut(ctx); err != nil {
		t.Fatalf("FanOut: %v", err)
	}

	if len(*queued) != 1 {
		t.Fatalf(
			"an event that belongs to no team produced %d deliveries for a team-scoped owner, "+
				"want 1. Treating an absent team as an unreachable one would drop every "+
				"workspace-level change.",
			len(*queued),
		)
	}
}
