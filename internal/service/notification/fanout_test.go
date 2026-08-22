package notification_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
)

func deliveryTo(deliveries []repository.NotificationDelivery, accountID uuid.UUID) (repository.NotificationDelivery, bool) {
	for _, delivery := range deliveries {
		if delivery.AccountID == accountID {
			return delivery, true
		}
	}

	return repository.NotificationDelivery{}, false
}

func TestSomeoneWhoCannotSeeTheIssueIsNeverDeliveredTo(t *testing.T) {
	h := newHarness(t)
	shutOut := uuid.New()

	h.expectPendingEvents(h.issueEvent(entity.NotificationKindStateChanged, entity.ActorKindUser))
	h.expectIssueOnTeam()
	h.expectFollowers(h.readerID, shutOut)
	h.expectAudience(h.readerID)
	h.expectSettings()

	captured := h.captureDeliveries()

	if _, err := h.service.FanOut(context.Background()); err != nil {
		t.Fatalf("fanning out failed: %v", err)
	}

	if _, found := deliveryTo(*captured, shutOut); found {
		t.Fatal(
			"someone the audience filter excluded was still delivered to. Following an issue " +
				"survives losing access to its team, so the audience check is the only thing " +
				"between a revoked reader and a notification about work they cannot open.",
		)
	}

	if _, found := deliveryTo(*captured, h.readerID); !found {
		t.Fatal("the follower who can still see the issue was not delivered to")
	}
}

func TestAMentionOnAnIssueTheMentionedPersonCannotSeeDeliversNothing(t *testing.T) {
	h := newHarness(t)
	mentioned := uuid.New()
	commentID := uuid.New()

	event := h.issueEvent(entity.NotificationKindCommented, entity.ActorKindUser)
	event.CommentID = commentID

	h.expectPendingEvents(event)
	h.expectIssueOnTeam()
	h.expectFollowers()
	h.expectAudience()
	h.expectSettings()

	h.comments.EXPECT().
		Mentioned(gomock.Any(), commentID).
		Return([]entity.CommentMention{
			{Kind: entity.MentionKindAccount, AccountID: mentioned, Visible: true},
		}, nil)

	captured := h.captureDeliveries()

	if _, err := h.service.FanOut(context.Background()); err != nil {
		t.Fatalf("fanning out failed: %v", err)
	}

	if len(*captured) != 0 {
		t.Fatalf(
			"a mention produced %d deliveries for someone outside the issue's audience. "+
				"Being named in a comment must never be a way to learn that private work exists.",
			len(*captured),
		)
	}
}

func TestAMentionOutranksFollowingOnTheSameComment(t *testing.T) {
	h := newHarness(t)
	commentID := uuid.New()

	event := h.issueEvent(entity.NotificationKindCommented, entity.ActorKindUser)
	event.CommentID = commentID

	h.expectPendingEvents(event)
	h.expectIssueOnTeam()
	h.expectFollowers(h.readerID)
	h.expectAudience(h.readerID)
	h.expectSettings()

	h.comments.EXPECT().
		Mentioned(gomock.Any(), commentID).
		Return([]entity.CommentMention{
			{Kind: entity.MentionKindAccount, AccountID: h.readerID, Visible: true},
		}, nil)

	captured := h.captureDeliveries()

	if _, err := h.service.FanOut(context.Background()); err != nil {
		t.Fatalf("fanning out failed: %v", err)
	}

	if len(*captured) != 1 {
		t.Fatalf(
			"a comment that both mentions a follower and lands on their followed issue produced "+
				"%d deliveries; one comment is one thing that happened", len(*captured),
		)
	}

	delivery, _ := deliveryTo(*captured, h.readerID)
	if delivery.Reason != entity.NotificationReasonMentioned {
		t.Fatalf("the delivery reason is %q, want mentioned", delivery.Reason)
	}
}

func TestTheActorIsNeverNotifiedAboutTheirOwnChange(t *testing.T) {
	h := newHarness(t)

	h.expectPendingEvents(h.issueEvent(entity.NotificationKindStateChanged, entity.ActorKindUser))
	h.expectIssueOnTeam()
	h.expectFollowers(h.actorID, h.readerID)
	h.expectAudience(h.actorID, h.readerID)
	h.expectSettings()

	captured := h.captureDeliveries()

	if _, err := h.service.FanOut(context.Background()); err != nil {
		t.Fatalf("fanning out failed: %v", err)
	}

	if _, found := deliveryTo(*captured, h.actorID); found {
		t.Fatal("the person who made the change was told about it")
	}
}

func TestATeamOverrideSilencesOnlyThatTeam(t *testing.T) {
	h := newHarness(t)

	silenced := entity.DefaultNotificationPreferences()
	silenced.StateChanged = entity.NotificationChannels{}

	h.expectPendingEvents(h.issueEvent(entity.NotificationKindStateChanged, entity.ActorKindUser))
	h.expectIssueOnTeam()
	h.expectFollowers(h.readerID)
	h.expectAudience(h.readerID)
	h.expectSettings(
		entity.NotificationSettings{
			AccountID:   h.readerID,
			Preferences: entity.DefaultNotificationPreferences(),
		},
		entity.NotificationSettings{
			AccountID:   h.readerID,
			TeamID:      h.teamID,
			Preferences: silenced,
		},
	)

	captured := h.captureDeliveries()

	if _, err := h.service.FanOut(context.Background()); err != nil {
		t.Fatalf("fanning out failed: %v", err)
	}

	if len(*captured) != 0 {
		t.Fatal(
			"the global row won over the team override. A per-team setting exists to quieten " +
				"one team without quietening the workspace, so it has to be the one that applies.",
		)
	}
}

func TestTurningAgentActivityOffStopsOnlyTheAgentsChanges(t *testing.T) {
	h := newHarness(t)

	withoutAgents := entity.DefaultNotificationPreferences()
	withoutAgents.Agents = entity.NotificationChannels{}

	h.expectPendingEvents(
		h.issueEvent(entity.NotificationKindStateChanged, entity.ActorKindAgent),
		h.issueEvent(entity.NotificationKindStateChanged, entity.ActorKindUser),
	)
	h.expectIssueOnTeam()
	h.expectFollowers(h.readerID)
	h.expectAudience(h.readerID)
	h.expectSettings(entity.NotificationSettings{
		AccountID:   h.readerID,
		Preferences: withoutAgents,
	})

	captured := h.captureDeliveries()

	if _, err := h.service.FanOut(context.Background()); err != nil {
		t.Fatalf("fanning out failed: %v", err)
	}

	if len(*captured) != 1 {
		t.Fatalf(
			"with agent activity off, an agent change and a human change produced %d deliveries; "+
				"the switch is about who acted, not about what changed", len(*captured),
		)
	}
}

func TestABulkOperationNotifiesNobody(t *testing.T) {
	h := newHarness(t)

	event := h.issueEvent(entity.NotificationKindStateChanged, entity.ActorKindUser)
	event.BulkActionID = uuid.New()

	h.expectPendingEvents(event)
	h.expectSettings()

	captured := h.captureDeliveries()

	if _, err := h.service.FanOut(context.Background()); err != nil {
		t.Fatalf("fanning out failed: %v", err)
	}

	if len(*captured) != 0 {
		t.Fatal(
			"a bulk operation produced notifications. One relabel across a filter can touch " +
				"thousands of issues, and one entry per issue is exactly the flood that makes " +
				"an inbox worth abandoning.",
		)
	}
}

func TestAMutedThreadStaysQuietUntilSomeoneNamesYou(t *testing.T) {
	h := newHarness(t)
	commentID := uuid.New()

	h.issues.EXPECT().
		GetVisible(gomock.Any(), h.workspaceID, h.issueID, gomock.Any()).
		Return(entity.Issue{ID: h.issueID, WorkspaceID: h.workspaceID, TeamID: h.teamID}, nil).
		AnyTimes()

	h.followers.EXPECT().
		List(gomock.Any(), h.issueID).
		Return([]entity.IssueFollower{{
			IssueID:     h.issueID,
			WorkspaceID: h.workspaceID,
			AccountID:   h.readerID,
			State:       entity.FollowStateMuted,
		}}, nil).
		AnyTimes()

	h.expectAudience(h.readerID)
	h.expectSettings()

	quiet := h.issueEvent(entity.NotificationKindStateChanged, entity.ActorKindUser)
	naming := h.issueEvent(entity.NotificationKindCommented, entity.ActorKindUser)
	naming.CommentID = commentID

	h.expectPendingEvents(quiet, naming)

	h.comments.EXPECT().
		Mentioned(gomock.Any(), commentID).
		Return([]entity.CommentMention{
			{Kind: entity.MentionKindAccount, AccountID: h.readerID, Visible: true},
		}, nil)

	captured := h.captureDeliveries()

	if _, err := h.service.FanOut(context.Background()); err != nil {
		t.Fatalf("fanning out failed: %v", err)
	}

	if len(*captured) != 1 {
		t.Fatalf(
			"a muted thread produced %d deliveries, want exactly the one that names the reader. "+
				"Unsubscribing silences the thread's chatter without making someone unreachable.",
			len(*captured),
		)
	}

	if (*captured)[0].Reason != entity.NotificationReasonMentioned {
		t.Fatalf("the one delivery that got through is %q, want mentioned", (*captured)[0].Reason)
	}
}
