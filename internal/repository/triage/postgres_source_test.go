package triage

import (
	"regexp"
	"strings"
	"testing"
)

func TestNoTriageQueryCountsIssues(t *testing.T) {
	counting := regexp.MustCompile(`(?i)\b(count|sum)\s*\(`)

	for name, query := range map[string]string{
		"settingsByTeamQuery":  settingsByTeamQuery,
		"upsertSettingsQuery":  upsertSettingsQuery,
		"disableSettingsQuery": disableSettingsQuery,
		"decideQuery":          decideQuery,
	} {
		if counting.MatchString(query) {
			t.Errorf(
				"%s tallies rows. How much is waiting is a scoped question the issue query already "+
					"answers through the caller's own TeamScope; a count taken here would carry no "+
					"scope at all.",
				name,
			)
		}
	}
}

func TestADecisionOnlyEverLandsOnSomethingStillWaiting(t *testing.T) {
	if !strings.Contains(decideQuery, "triage_state = 'waiting'") {
		t.Fatal(
			"the decision write does not require the issue to still be waiting. Two people " +
				"deciding at once would each overwrite the other, and the second would report " +
				"success for a decision that never happened.",
		)
	}
}
