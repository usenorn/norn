package entity_test

import (
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestAnAddressIsNamedAfterTheTeamButNotGuessable(t *testing.T) {
	for name, expected := range map[string]struct {
		team  entity.Team
		label string
	}{
		"a plain name":            {entity.Team{Name: "Core", Key: "DC"}, "core"},
		"a name with spaces":      {entity.Team{Name: "Customer Success", Key: "CS"}, "customer-success"},
		"a name with punctuation": {entity.Team{Name: "R&D / Labs", Key: "RD"}, "r-d-labs"},
		"a name with no latin":    {entity.Team{Name: "Поддержка", Key: "SUP"}, "podderzhka"},
		"a name of symbols only":  {entity.Team{Name: "***", Key: "OPS"}, "ops"},
		"nothing to go on":        {entity.Team{}, entity.IntakeLabelFallback},
	} {
		t.Run(name, func(t *testing.T) {
			if label := entity.IntakeLabel(expected.team); label != expected.label {
				t.Fatalf(
					"label=%q, want %q. The address is printed in a settings screen and typed into a "+
						"mail client, so it has to read as the team and survive being copied.",
					label, expected.label,
				)
			}
		})
	}
}

func TestAnAddressLabelStaysWithinItsLimit(t *testing.T) {
	label := entity.IntakeLabel(entity.Team{Name: strings.Repeat("platform ", 8), Key: "PLA"})

	if len(label) > entity.IntakeLabelMaxLen {
		t.Fatalf(
			"label is %d characters, over the %d limit. A local part has a length a mail server "+
				"will refuse, and the random half is what makes the address unguessable.",
			len(label), entity.IntakeLabelMaxLen,
		)
	}

	if strings.HasSuffix(label, "-") {
		t.Fatalf("label %q ends in the separator, so the address would read team--token", label)
	}
}

func TestOnlyMailForOurOwnDomainIsRouted(t *testing.T) {
	for name, expected := range map[string]struct {
		recipients []string
		local      string
		routed     bool
	}{
		"a plain address":         {[]string{"core-abc@submit.norn.so"}, "core-abc", true},
		"an address with a name":  {[]string{"Norn <core-abc@submit.norn.so>"}, "core-abc", true},
		"mixed case":              {[]string{"Core-ABC@Submit.Norn.So"}, "core-abc", true},
		"ours among others":       {[]string{"someone@example.com", "core-abc@submit.norn.so"}, "core-abc", true},
		"another domain entirely": {[]string{"core-abc@example.com"}, "", false},
		"a lookalike domain":      {[]string{"core-abc@notsubmit.norn.so.example"}, "", false},
		"nobody at all":           {nil, "", false},
	} {
		t.Run(name, func(t *testing.T) {
			local, err := entity.IntakeRecipient(expected.recipients, "submit.norn.so")

			if expected.routed && err != nil {
				t.Fatalf("routing refused %v: %v", expected.recipients, err)
			}

			if !expected.routed && err == nil {
				t.Fatalf(
					"routed %v to %q. Mail for a domain this instance does not own must not reach a "+
						"team's queue.",
					expected.recipients, local,
				)
			}

			if local != expected.local {
				t.Fatalf("local part=%q, want %q", local, expected.local)
			}
		})
	}
}

func TestAMessageWithNoSubjectStillBecomesAReadableIssue(t *testing.T) {
	if title := entity.IntakeTitle("   "); title != entity.IntakeSubjectFallback {
		t.Fatalf("title=%q, want the fallback. A blank row in triage tells nobody anything", title)
	}

	if title := entity.IntakeTitle("Cannot\nsign in\r\nat all"); title != "Cannot sign in at all" {
		t.Fatalf("title=%q, want the subject on one line", title)
	}

	long := entity.IntakeTitle(strings.Repeat("é", entity.IssueTitleMaxLen+40))

	if count := len([]rune(long)); count != entity.IssueTitleMaxLen {
		t.Fatalf(
			"title is %d runes, want %d. Titles are validated in characters, and a message the "+
				"backend refuses is a report that silently disappears.",
			count, entity.IssueTitleMaxLen,
		)
	}
}

func TestTheDescriptionSaysWhoWroteInAndWhatTheySent(t *testing.T) {
	description := entity.IntakeDescription(entity.InboundMessage{
		Sender: "Rae Whitfield <rae@northwind.co>",
		Text:   "The export button does nothing.",
	})

	for _, want := range []string{"rae@northwind.co", "The export button does nothing."} {
		if !strings.Contains(description, want) {
			t.Fatalf(
				"description is missing %q:\n%s\nNobody can answer a report without knowing who "+
					"sent it.",
				want, description,
			)
		}
	}
}

func TestAPictureKeepsItsPlaceInTheBody(t *testing.T) {
	message := entity.InboundMessage{
		Sender: "rae@northwind.co",
		Text:   "The export button does nothing.",
		HTML: `<p>Before</p><img src="cid:shot@mail" alt="The stuck page">` +
			`<p>After</p><img src="https://northwind.co/logo.png">`,
		Attachments: []entity.InboundAttachment{
			{FileName: "shot.png", ContentType: "image/png", ContentID: "shot@mail"},
		},
	}

	description := entity.IntakeDescription(message)

	if !strings.Contains(description, "![The stuck page](cid:shot@mail)") {
		t.Fatalf(
			"the picture lost its reference:\n%s\nThe plain text alternative carries no pictures, "+
				"so a message with one has to be read from the markup.",
			description,
		)
	}

	if strings.Index(description, "Before") > strings.Index(description, "![The stuck page]") {
		t.Fatalf("the picture moved out of the place the sender put it:\n%s", description)
	}

	embedded := entity.IntakeEmbed(
		description,
		map[string]string{"cid:shot@mail": "/v1/workspaces/w/attachments/a/content"},
		[]string{"huge.zip"},
	)

	if !strings.Contains(embedded, "![The stuck page](/v1/workspaces/w/attachments/a/content)") {
		t.Fatalf("the stored file was not put where the picture was:\n%s", embedded)
	}

	if !strings.Contains(embedded, "https://northwind.co/logo.png") {
		t.Fatalf("a picture hosted elsewhere was dropped:\n%s", embedded)
	}

	if !strings.Contains(embedded, "Not kept: huge.zip") {
		t.Fatalf(
			"nothing says a file was refused:\n%s\nA file silently missing is worse than one "+
				"named as too big to keep.",
			embedded,
		)
	}
}

func TestAPictureWithNoFileBehindItIsNotLeftBroken(t *testing.T) {
	description := entity.IntakeDescription(entity.InboundMessage{
		Sender: "rae@northwind.co",
		HTML:   `<p>Look</p><img src="cid:missing@mail" alt="Gone">`,
		Attachments: []entity.InboundAttachment{
			{FileName: "gone.png", ContentID: "missing@mail"},
		},
	})

	embedded := entity.IntakeEmbed(description, nil, nil)

	if strings.Contains(embedded, "cid:") {
		t.Fatalf(
			"a reference into the message survived into the issue:\n%s\nIt renders as a broken "+
				"image, which reads as Norn losing the file rather than never storing it.",
			embedded,
		)
	}

	if !strings.Contains(embedded, "Look") {
		t.Fatalf("the words around the picture were dropped with it:\n%s", embedded)
	}
}

func TestAMessageWithOnlyMarkupIsStillReadable(t *testing.T) {
	description := entity.IntakeDescription(entity.InboundMessage{
		Sender: "rae@northwind.co",
		HTML: "<style>p{color:red}</style><p>Line one</p><p>Line&nbsp;two &amp; more</p>" +
			"<script>alert(1)</script>",
	})

	if strings.Contains(description, "<") || strings.Contains(description, "alert(1)") {
		t.Fatalf("markup survived into the issue body:\n%s", description)
	}

	for _, want := range []string{"Line one", "Line two & more"} {
		if !strings.Contains(description, want) {
			t.Fatalf("description is missing %q:\n%s", want, description)
		}
	}
}
