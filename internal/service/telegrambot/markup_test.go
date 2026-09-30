package telegrambot

import (
	"strings"
	"testing"
)

func TestAPlanWrittenInMarkdownReadsAsTelegramFormattingRatherThanItsSymbols(t *testing.T) {
	rendered := markup("## Steps\n\n1. Add `median` to **stats.py**\n2. See [the docs](https://go.dev)\n\n- keep <tests> green", 1500)

	for _, want := range []string{
		"<b>Steps</b>",
		"1. Add <code>median</code> to <b>stats.py</b>",
		`2. See <a href="https://go.dev">the docs</a>`,
		"• keep &lt;tests&gt; green",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("the plan rendered without %q:\n%s", want, rendered)
		}
	}

	for _, raw := range []string{"##", "**", "`"} {
		if strings.Contains(rendered, raw) {
			t.Errorf("the plan still shows the markdown %q:\n%s", raw, rendered)
		}
	}
}

func TestATagMentionedInThePlanIsShownRatherThanSwallowed(t *testing.T) {
	rendered := markup("Add <p id=\"median\"> under the mean, and `<p id=\"mean\">` stays.", 300)

	if !strings.Contains(rendered, "Add &lt;p id=&#34;median&#34;&gt; under the mean") ||
		!strings.Contains(rendered, "<code>&lt;p id=&#34;mean&#34;&gt;</code>") {
		t.Fatalf("the tags in the plan came out as:\n%s", rendered)
	}
}

func TestALinkThatIsNotTheWebIsLeftAsText(t *testing.T) {
	if rendered := markup("[run me](javascript:alert(1))", 200); strings.Contains(rendered, "<a ") {
		t.Fatalf("a script link became clickable: %s", rendered)
	}
}

func TestALongPlanStopsAtAWholeBlockAndSaysSo(t *testing.T) {
	rendered := markup(strings.Repeat("A paragraph of the plan.\n\n", 200), 120)

	if !strings.HasSuffix(rendered, ellipsis) || len([]rune(plainOf(rendered))) > 140 {
		t.Fatalf("a long plan came out as %d characters:\n%s", len([]rune(plainOf(rendered))), rendered)
	}
}

func TestEachDecisionSaysWhatKindItIsBeforeAnythingElse(t *testing.T) {
	about := decisionContext{agentName: "Builder", reference: "BOBO-4", title: "Add a median"}

	for kind, want := range map[decisionKind]string{
		kindQuestion:    "❓ <b>Question</b>",
		kindPlan:        "📋 <b>Plan to approve</b>",
		kindReview:      "🔍 <b>Changes to review</b>",
		kindPublication: "⚠️ <b>Publication incomplete</b>",
	} {
		if header := about.header(kind, true); !strings.HasPrefix(header, want) {
			t.Errorf("a %s opens with %q", kind.open, header)
		}
	}
}
