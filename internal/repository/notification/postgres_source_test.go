package notification

import (
	"regexp"
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

var aggregating = regexp.MustCompile(`(?i)\b(count|sum|avg|min|max)\s*\(`)

func TestOnlyTheReadersOwnTallyCounts(t *testing.T) {
	for name, query := range map[string]string{
		"deliverQuery":          deliverQuery,
		"digestRecipientsQuery": digestRecipientsQuery,
		"markReadQuery":         markReadQuery,
		"markAllReadQuery":      markAllReadQuery,
		"snoozeQuery":           snoozeQuery,
		"claimDigestQuery":      claimDigestQuery,
		"digestSentQuery":       digestSentQuery,
		"digestFailedQuery":     digestFailedQuery,
		"visibleDeliveries":     visibleDeliveries,
	} {
		if aggregating.MatchString(query) {
			t.Errorf(
				"%s aggregates rows. The only tally this package may produce is a reader's own "+
					"unread count, which discloses nothing by subtraction because every row in "+
					"its scope is already one of that reader's deliveries. A number computed over "+
					"anything wider would be a fact about work the reader may not be able to see.",
				name,
			)
		}
	}
}

func TestEveryGroupedReadNarrowsToTheReadersOwnDeliveries(t *testing.T) {
	for name, query := range map[string]string{
		"inboxQuery":          inboxQuery,
		"unreadSubjectsQuery": unreadSubjectsQuery,
		"digestEntriesQuery":  digestEntriesQuery,
	} {
		for _, scope := range []string{"d.workspace_id = $1", "d.account_id = $2"} {
			if !strings.Contains(query, scope) {
				t.Errorf(
					"%s does not narrow by %s, so one reader's grouping would fold in another's "+
						"deliveries and the unread tally would count rows that are not theirs",
					name, scope,
				)
			}
		}
	}
}

func TestEveryReadJoinsLiveVisibilityRatherThanTrustingTheDeliveryRow(t *testing.T) {
	rules := []string{
		"m.role = 'admin'",
		"it.visibility = 'public'",
		"itm.account_id IS NOT NULL",
		"i.status = 'active'",
		"p.archived_at IS NULL",
		"t.visibility = 'public'",
		"tm.account_id IS NOT NULL",
	}

	for name, query := range map[string]string{
		"visibleDeliveries": visibleDeliveries,
		"directedQuery":     directedQuery,
	} {
		for _, rule := range rules {
			if !strings.Contains(query, rule) {
				t.Fatalf(
					"%s is missing %q. A delivery row records that someone was "+
						"notified, not that they may still look: an issue moved to a private team, a "+
						"team flipped to private, a member removed from a team and an archived issue "+
						"all revoke access after the row was written. Nothing sweeps those rows up, so "+
						"this join is the only thing standing between a revocation and a leak.",
					name, rule,
				)
			}
		}
	}

	if strings.Contains(visibleDeliveries, "d.title") || strings.Contains(visibleDeliveries, "d.summary") {
		t.Fatal(
			"a delivery row carries rendered content. Text frozen at notification time outlives " +
				"the access check that produced it and is rendered again by the digest, so it " +
				"survives every revocation the live join is there to catch.",
		)
	}
}

func TestAnAgentIsToldInTheAppAndNeverByMail(t *testing.T) {
	if strings.Contains(audienceQuery, "a.kind = 'person'") {
		t.Fatal(
			"the in-app audience excludes agents, so assigning an issue to one or mentioning it " +
				"records the intent and then drops it. Being addressable has to reach the agent " +
				"or it means nothing.",
		)
	}

	if !strings.Contains(digestRecipientsQuery, "a.kind = 'person'") {
		t.Fatal(
			"the digest would mail an agent. An agent reads its inbox over the API; sending it " +
				"email delivers to whatever address the account happens to carry, which is a " +
				"person's inbox filling with machine traffic nobody asked for.",
		)
	}
}

func TestTheDigestNeverMailsSomethingAlreadyReadOrAnAgent(t *testing.T) {
	for _, rule := range []string{
		"a.status = 'active'",
		"a.kind = 'person'",
		"r.read_through IS NULL OR e.created_at > r.read_through",
	} {
		if !strings.Contains(digestRecipientsQuery, rule) {
			t.Fatalf(
				"the digest recipient query is missing %q. Agents hold memberships and carry "+
					"email addresses, deactivated members keep theirs, and a notification already "+
					"read in the app should not arrive again by mail.",
				rule,
			)
		}
	}
}

func TestTheInboxAndTheDigestReadDifferentChannels(t *testing.T) {
	if !strings.Contains(inboxQuery, "d.inbox") {
		t.Fatal(
			"the inbox does not filter on the inbox channel, so turning a kind off for the " +
				"inbox while leaving it on for email would still put it on screen",
		)
	}

	if !strings.Contains(digestEntriesQuery, "d.email") {
		t.Fatal(
			"the digest does not filter on the email channel, so every inbox-only notification " +
				"would be mailed and the email preference would be a no-op",
		)
	}
}

func TestTheStrongestReasonIsWhicheverEntityRanksFirst(t *testing.T) {
	reasons := entity.NotificationReasons()

	if got := strongest(strings.Join([]string{"following", "mentioned", "assigned"}, " ")); got != reasons[0] {
		t.Fatalf("folded three reasons to %q, want %q", got, reasons[0])
	}

	if got := strongest("following"); got != entity.NotificationReasonFollowing {
		t.Fatalf("folded one reason to %q, want following", got)
	}
}

func TestAReasonThatIsReadAgainDropsBackToTheWeakerOne(t *testing.T) {
	if !strings.Contains(inboxQuery, "FILTER (WHERE "+unreadRows+")") {
		t.Fatal(
			"the reason is folded over every delivery rather than the unread ones, so an entry " +
				"read after a mention would stay labelled as a mention forever",
		)
	}
}
