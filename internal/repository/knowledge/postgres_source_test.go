package knowledge

import (
	"strings"
	"testing"
)

func TestEveryLaneFiltersByAudienceBeforeItScores(t *testing.T) {
	lanes := map[string]struct {
		statement string
		boundary  string
		rules     []string
	}{
		"lexical memories": {lexicalMemoriesQuery, "LIMIT $13", []string{
			"m.visibility = 'workspace'", "m.project_id = ANY($3::uuid[])", "m.author_account_id = $4::uuid",
		}},
		"semantic memories": {semanticMemoriesQuery, "ORDER BY", []string{
			"m.visibility = 'workspace'", "m.project_id = ANY($3::uuid[])", "m.author_account_id = $4::uuid",
		}},
		"lexical passages": {lexicalPassagesQuery, "LIMIT $9", []string{
			"d.visibility = 'workspace'", "d.project_id = ANY($3::uuid[])", "d.author_account_id = $4::uuid",
			"d.status = 'active'",
		}},
		"semantic passages": {semanticPassagesQuery, "ORDER BY", []string{
			"d.visibility = 'workspace'", "d.project_id = ANY($3::uuid[])", "d.author_account_id = $4::uuid",
			"d.status = 'active'",
		}},
	}

	for name, lane := range lanes {
		boundary := strings.Index(lane.statement, lane.boundary)
		if boundary < 0 {
			t.Fatalf("%s has no %q", name, lane.boundary)
		}

		candidates := lane.statement[:boundary]

		for _, rule := range lane.rules {
			if !strings.Contains(candidates, rule) {
				t.Errorf(
					"%s applies %q only after %s; something the reader cannot see would then take a "+
						"candidate slot and shape which visible knowledge makes the cut",
					name, rule, lane.boundary,
				)
			}
		}

		if strings.Contains(candidates, "ts_headline") {
			t.Errorf("%s builds excerpts for every candidate rather than only the rows returned", name)
		}
	}
}
