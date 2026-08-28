package entity_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestAMentionOutranksEveryOtherReasonWhicheverArrivesFirst(t *testing.T) {
	for _, weaker := range []entity.NotificationReason{
		entity.NotificationReasonAssigned,
		entity.NotificationReasonMembership,
		entity.NotificationReasonFollowing,
	} {
		if got := weaker.Strongest(entity.NotificationReasonMentioned); got != entity.NotificationReasonMentioned {
			t.Errorf("%s then a mention folded to %s, want mentioned", weaker, got)
		}

		if got := entity.NotificationReasonMentioned.Strongest(weaker); got != entity.NotificationReasonMentioned {
			t.Errorf("a mention then %s folded to %s, want mentioned", weaker, got)
		}
	}
}

func TestAnUnrecognisedReasonNeverDisplacesAKnownOne(t *testing.T) {
	if got := entity.NotificationReasonFollowing.Strongest("subscribed"); got != entity.NotificationReasonFollowing {
		t.Fatalf("an unknown reason won the fold and produced %q", got)
	}
}

func TestATeamOverrideWinsOverTheAccountsGlobalRow(t *testing.T) {
	team := uuid.New()

	global := entity.DefaultNotificationPreferences()
	global.Commented = entity.NotificationChannels{Inbox: true, Email: true}

	override := entity.DefaultNotificationPreferences()
	override.Commented = entity.NotificationChannels{}

	settings := []entity.NotificationSettings{
		{Preferences: global},
		{TeamID: team, Preferences: override},
	}

	if resolved := entity.ResolveNotificationPreferences(settings, team); resolved.Commented.Inbox {
		t.Fatal(
			"the global row beat the team override. A per-team setting exists precisely to " +
				"silence one noisy team without silencing the rest of the workspace.",
		)
	}

	if resolved := entity.ResolveNotificationPreferences(settings, uuid.New()); !resolved.Commented.Inbox {
		t.Fatal("an unrelated team picked up another team's override")
	}
}

func TestAnAccountWithNoRowsFallsBackToTheBuiltInDefaults(t *testing.T) {
	resolved := entity.ResolveNotificationPreferences(nil, uuid.New())

	if resolved != entity.DefaultNotificationPreferences() {
		t.Fatal("resolving without any stored row did not produce the built-in defaults")
	}
}

func TestASubjectWithNoTeamResolvesAgainstTheGlobalRowAlone(t *testing.T) {
	global := entity.DefaultNotificationPreferences()
	global.Membership = entity.NotificationChannels{}

	settings := []entity.NotificationSettings{
		{Preferences: global},
		{TeamID: uuid.New(), Preferences: entity.DefaultNotificationPreferences()},
	}

	if entity.ResolveNotificationPreferences(settings, uuid.Nil).Membership.Inbox {
		t.Fatal(
			"a project or team subject picked up a team override. Neither has a team to " +
				"resolve against, so only the global row can apply.",
		)
	}
}

func TestTurningAgentsOffSilencesOnlyWhatAnAgentDid(t *testing.T) {
	preferences := entity.DefaultNotificationPreferences()
	preferences.Agents = entity.NotificationChannels{}

	if preferences.Delivers(entity.NotificationKindAssigned, entity.ActorKindAgent).Inbox {
		t.Fatal("an agent assignment still notified after agent activity was turned off")
	}

	for _, actor := range []entity.ActorKind{
		entity.ActorKindUser,
		entity.ActorKindToken,
		entity.ActorKindSystem,
	} {
		if !preferences.Delivers(entity.NotificationKindAssigned, actor).Inbox {
			t.Errorf("turning agents off also silenced a %s assignment", actor)
		}
	}
}

func TestTurningAKindOffSilencesItForEveryActor(t *testing.T) {
	preferences := entity.DefaultNotificationPreferences()
	preferences.Commented = entity.NotificationChannels{}

	for _, actor := range []entity.ActorKind{entity.ActorKindUser, entity.ActorKindAgent} {
		if preferences.Delivers(entity.NotificationKindCommented, actor).Inbox {
			t.Errorf("a %s comment notified after comments were turned off", actor)
		}
	}
}

func TestTheDigestWindowIsAClosedGridCellRatherThanTheOneStillFilling(t *testing.T) {
	at := time.Date(2026, 8, 5, 10, 17, 42, 0, time.UTC)
	window := entity.NotificationDigestWindowAt(at)

	if want := time.Date(2026, 8, 5, 9, 45, 0, 0, time.UTC); !window.Equal(want) {
		t.Fatalf("the window at %s is %s, want %s", at, window, want)
	}

	onTheBoundary := time.Date(2026, 8, 5, 10, 15, 0, 0, time.UTC)
	closed := entity.NotificationDigestWindowAt(onTheBoundary).Add(entity.NotificationDigestWindow)

	if !closed.Add(entity.NotificationDigestWindow).Equal(onTheBoundary) {
		t.Fatal(
			"a run landing exactly on a grid boundary covers the cell that closed at that " +
				"instant. A delivery written a moment before the boundary can still be " +
				"uncommitted then, and nothing ever revisits a window, so it would never be " +
				"mailed at all.",
		)
	}
}

func TestACursorSurvivesTheRoundTrip(t *testing.T) {
	cursor := entity.NotificationCursor{
		LastEventAt: time.Now().UTC().Truncate(time.Microsecond),
		SubjectID:   uuid.New(),
	}

	decoded, err := entity.DecodeNotificationCursor(cursor.Encode())
	if err != nil {
		t.Fatalf("decoding a cursor we encoded failed: %v", err)
	}

	if !decoded.LastEventAt.Equal(cursor.LastEventAt) || decoded.SubjectID != cursor.SubjectID {
		t.Fatalf("decoded %+v, want %+v", decoded, cursor)
	}

	if _, err := entity.DecodeNotificationCursor("not-a-cursor"); err == nil {
		t.Fatal("a malformed cursor decoded without complaint")
	}
}
