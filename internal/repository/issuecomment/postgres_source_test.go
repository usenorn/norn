package issuecomment

import (
	"regexp"
	"strings"
	"testing"
)

func parenthesised(query string, from int) string {
	open := strings.Index(query[from:], "(")
	if open < 0 {
		return ""
	}

	open += from
	depth := 0

	for i, r := range query[open:] {
		switch r {
		case '(':
			depth++
		case ')':
			depth--

			if depth == 0 {
				return query[open+1 : open+i]
			}
		}
	}

	return ""
}

func writtenColumns(query string) string {
	return parenthesised(query, strings.Index(query, "INSERT INTO"))
}

var selected = regexp.MustCompile(`(?s)SELECT (.*?)\n\s*FROM`)

func writtenValues(query string) string {
	columns := writtenColumns(query)
	source := query[strings.Index(query, columns)+len(columns):]

	if at := strings.Index(source, "VALUES"); at >= 0 {
		return parenthesised(source, at)
	}

	rows := selected.FindStringSubmatch(source)
	if rows == nil {
		return ""
	}

	return rows[1]
}

func fields(list string) int {
	depth, count := 0, 1

	for _, r := range list {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				count++
			}
		}
	}

	return count
}

func TestEveryCommentInsertSuppliesAValueForEveryColumnItNames(t *testing.T) {
	for name, query := range map[string]string{
		"createCommentQuery": createCommentQuery,
		"recordMentionQuery": recordMentionQuery,
		"reactQuery":         reactQuery,
	} {
		columns, values := writtenColumns(query), writtenValues(query)

		if columns == "" || values == "" {
			t.Fatalf("could not read %s's shape", name)
		}

		if named, bound := fields(columns), fields(values); named != bound {
			t.Errorf(
				"%s names %d columns but supplies %d values. A column added to the table and to "+
					"the entity but not here is written as its default, so the field silently "+
					"never persists and nothing fails loudly.",
				name, named, bound,
			)
		}
	}
}

func TestWhatEachInsertWritesTheMatchingReadReturns(t *testing.T) {
	for name, pair := range map[string]struct {
		insert, read string
		skipped      map[string]bool
	}{
		"a comment": {createCommentQuery, commentColumns, nil},
		"a mention": {recordMentionQuery, mentionsQuery, map[string]bool{"workspace_id": true}},
	} {
		for _, raw := range strings.Split(writtenColumns(pair.insert), ",") {
			column := strings.TrimSpace(raw)

			if column == "" || pair.skipped[column] {
				continue
			}

			if !strings.Contains(pair.read, column) {
				t.Errorf(
					"the insert for %s writes %s but the read never selects it, so the value is "+
						"stored and then thrown away on every read",
					name, column,
				)
			}
		}
	}
}

func TestNoCommentQueryAggregatesAnything(t *testing.T) {
	counting := regexp.MustCompile(`(?i)\b(count|sum|array_agg)\s*\(`)

	for name, query := range map[string]string{
		"createCommentQuery":    createCommentQuery,
		"commentByIDQuery":      commentByIDQuery,
		"threadRootsQuery":      threadRootsQuery,
		"repliesQuery":          repliesQuery,
		"mentionsQuery":         mentionsQuery,
		"reactionsQuery":        reactionsQuery,
		"recordMentionQuery":    recordMentionQuery,
		"editCommentQuery":      editCommentQuery,
		"tombstoneCommentQuery": tombstoneCommentQuery,
		"reactQuery":            reactQuery,
		"unreactQuery":          unreactQuery,
		"visibleAudienceQuery":  visibleAudienceQuery,
	} {
		if counting.MatchString(query) {
			t.Errorf(
				"%s aggregates rows. A reaction tally is assembled in Go from the rows the caller "+
					"is already reading, so nothing here produces a number whose scope no guard "+
					"covers — a comment count per issue least of all.",
				name,
			)
		}
	}
}

func TestATombstoneClearsTheEditMarkerItCannotKeep(t *testing.T) {
	if !strings.Contains(tombstoneCommentQuery, "edited_at = NULL") {
		t.Fatal(
			"deleting a comment leaves edited_at set. The table's tombstone CHECK refuses that " +
				"row, so deleting an edited comment would fail at the database rather than here.",
		)
	}

	if !strings.Contains(tombstoneCommentQuery, "body = ''") {
		t.Fatal(
			"deleting a comment keeps its words. Deletion has to actually remove what was said; " +
				"the row surviving is the slot, not the content.",
		)
	}
}

func TestNeitherWriteTouchesAnAlreadyDeletedComment(t *testing.T) {
	for name, query := range map[string]string{
		"editCommentQuery":      editCommentQuery,
		"tombstoneCommentQuery": tombstoneCommentQuery,
	} {
		if !strings.Contains(query, "deleted_at IS NULL") {
			t.Errorf(
				"%s can land on a tombstone. Two people deleting at once would each record an "+
					"activity entry, and an edit would resurrect words their author removed.",
				name,
			)
		}
	}
}

func TestTheAudienceQueryNarrowsByTheSameThreeRulesTeamScopeDoes(t *testing.T) {
	rules := []string{"m.role = 'admin'", "t.visibility = 'public'", "tm.account_id IS NOT NULL"}

	for _, rule := range rules {
		if !strings.Contains(visibleAudienceQuery, rule) {
			t.Fatalf(
				"the audience query is missing %q. The members list is deliberately unscoped, so "+
					"the mention picker's candidates are wider than the issue's audience and this "+
					"query is the only place that narrowing happens.",
				rule,
			)
		}
	}
}

func TestTheThreadPagesForwardOverRootsAlone(t *testing.T) {
	if !strings.Contains(threadRootsQuery, "(c.created_at, c.id) > ") {
		t.Fatal(
			"the thread cursor does not compare forward. A conversation reads oldest first where " +
				"an audit log reads newest first; paging backwards would land a reader mid-argument.",
		)
	}

	if !strings.Contains(threadRootsQuery, "c.parent_comment_id IS NULL") {
		t.Fatal(
			"the page is taken over every comment rather than over roots only, so a reply can " +
				"land on a page whose parent is on another one and render as an orphan",
		)
	}
}
