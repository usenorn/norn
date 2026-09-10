package entity_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestWhatSomebodyWroteSurvivesTheRoundTrip(t *testing.T) {
	for name, written := range map[string]string{
		"a paragraph":     "The export button does nothing.",
		"two paragraphs":  "First thing.\n\nSecond thing.",
		"headings":        "# Title\n\n## Section\n\nWords.",
		"emphasis":        "This is **bold**, this is _thin_, this is ~~gone~~, this is `code`.",
		"a link":          "See [the docs](https://norn.so/docs) for more.",
		"a bullet list":   "- first\n- second\n- third",
		"a numbered list": "1. first\n2. second\n3. third",
		"a checklist":     "- [ ] not yet\n- [x] done",
		"a quote":         "> Somebody said this.",
		"a rule":          "Above.\n\n---\n\nBelow.",
		"a code block":    "```go\nfmt.Println(\"hello\")\n```",
		"a table":         "| Name | State |\n| --- | --- |\n| Export | broken |",
		"an image":        "![The stuck page](/v1/workspaces/w/attachments/a/content)",

		"code inside a bold sentence": "**Stop declaring `additionalProperties` here.**",
		"bold inside a quoted italic": "It reads _\"they are **not** installed\"_ today.",
		"two code spans in one bold":  "**`one` and `two`** are both set.",
		"a link inside bold":          "**See [the docs](https://norn.so/docs) first.**",
		"a list nested under an item": "- Endpoints:\n  - `POST /issues`\n  - `GET /issues`",
		"a checklist under an item":   "- Work:\n  - [x] write it\n  - [ ] ship it",
		"an address a reader can see": "Write to rae@northwind.co about it.",
		"a bare url":                  "Read https://norn.so/docs for more.",
	} {
		t.Run(name, func(t *testing.T) {
			document := entity.DocumentFromMarkdown(written)

			if err := entity.ValidateDocument(document); err != nil {
				t.Fatalf("the document this instance produced is one it refuses: %v", err)
			}

			if rendered := document.Markdown(); rendered != written {
				t.Fatalf(
					"round trip changed the text.\n given: %q\nwritten: %q\n\nEvery description in "+
						"the workspace goes through this on its first edit; text that changes shape "+
						"is text somebody has to notice and fix by hand.",
					written, rendered,
				)
			}
		})
	}
}

func TestATaskListIsNotJustBulletsWithBrackets(t *testing.T) {
	document := entity.DocumentFromMarkdown("- [ ] write it\n- [x] check it")

	if document.Content[0].Type != entity.NodeTaskList {
		t.Fatalf(
			"a checklist parsed as %q. It is clicked rather than read, so it has to be its own "+
				"kind of block or the editor renders text where a checkbox belongs.",
			document.Content[0].Type,
		)
	}

	items := document.Content[0].Content

	if len(items) != 2 || items[0].Attrs["checked"] != false || items[1].Attrs["checked"] != true {
		t.Fatalf("the ticked state was lost: %+v", items)
	}
}

func TestADocumentSaysWhoItAddresses(t *testing.T) {
	account, team := uuid.New(), uuid.New()

	document := entity.NewDocument(entity.Node{
		Type: entity.NodeParagraph,
		Content: []entity.Node{
			{Type: entity.NodeText, Text: "Ask "},
			{
				Type:  entity.NodeMention,
				Attrs: map[string]any{"kind": "account", "id": account.String(), "label": "Rae"},
			},
			{Type: entity.NodeText, Text: " and "},
			{
				Type:  entity.NodeMention,
				Attrs: map[string]any{"kind": "team", "id": team.String(), "label": "Platform"},
			},
			{Type: entity.NodeText, Text: "."},
		},
	})

	mentions := entity.DocumentMentions(document)

	if len(mentions) != 2 {
		t.Fatalf("found %d mentions, want 2: %+v", len(mentions), mentions)
	}

	if mentions[0].AccountID != account || mentions[1].TeamID != team {
		t.Fatalf("the mentions point at the wrong subjects: %+v", mentions)
	}

	if rendered := document.Markdown(); rendered != "Ask @Rae and @Platform." {
		t.Fatalf("a mention reads badly outside the editor: %q", rendered)
	}
}

func TestADocumentRefusesWhatItCannotRender(t *testing.T) {
	unknown := entity.NewDocument(entity.Node{Type: "marquee"})

	if err := entity.ValidateDocument(unknown); err == nil {
		t.Fatal(
			"a block this instance cannot draw was accepted. It would be stored, read back and " +
				"silently vanish from the description.",
		)
	}

	deep := entity.Node{Type: entity.NodeBlockquote}
	nested := &deep

	for range entity.DocumentMaxDepth + 2 {
		nested.Content = []entity.Node{{Type: entity.NodeBlockquote}}
		nested = &nested.Content[0]
	}

	if err := entity.ValidateDocument(entity.NewDocument(deep)); err == nil {
		t.Fatal("a document nested past the limit was accepted, and every read walks it recursively")
	}
}

func TestAToggleKeepsItsSummaryOutsideTheEditor(t *testing.T) {
	document := entity.NewDocument(entity.Node{
		Type: entity.NodeDetails,
		Content: []entity.Node{
			{
				Type: entity.NodeDetailsSummary,
				Content: []entity.Node{{
					Type:    entity.NodeParagraph,
					Content: []entity.Node{{Type: entity.NodeText, Text: "What we tried"}},
				}},
			},
			{
				Type: entity.NodeDetailsContent,
				Content: []entity.Node{{
					Type:    entity.NodeParagraph,
					Content: []entity.Node{{Type: entity.NodeText, Text: "Rolling back did not help."}},
				}},
			},
		},
	})

	rendered := document.Markdown()

	for _, want := range []string{"<details>", "<summary>What we tried</summary>", "Rolling back did not help."} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("the toggle lost %q:\n%s", want, rendered)
		}
	}

	if summary := entity.DocumentFromMarkdown(rendered); summary.Content[0].Type != entity.NodeDetails {
		t.Fatalf("reading the toggle back produced %q", summary.Content[0].Type)
	}
}

func TestADescriptionWrittenInProductionSurvivesTheRoundTrip(t *testing.T) {
	written, err := os.ReadFile(filepath.Join("testdata", "issue-from-production.md"))
	if err != nil {
		t.Fatalf("%v", err)
	}

	// A stored description carries no trailing newline; the fixture is a file and does.
	given := strings.TrimRight(string(written), "\n")
	document := entity.DocumentFromMarkdown(given)

	if err := entity.ValidateDocument(document); err != nil {
		t.Fatalf("a description already in the workspace produced a document this instance refuses: %v", err)
	}

	if rendered := document.Markdown(); rendered != given {
		t.Fatalf(
			"a real description changed shape.\n\n--- given ---\n%s\n\n--- back ---\n%s",
			given, rendered,
		)
	}
}
