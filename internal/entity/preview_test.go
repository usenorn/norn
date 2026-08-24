package entity_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestAPreviewHostIsTheIssueTheRunAndThePortAndNothingItWasNamed(t *testing.T) {
	host := entity.PreviewHost(
		"NORN-75", "A preview address", "exec-01ABC", 43000,
		entity.PreviewBySubdomain, "preview.norn.site",
	)

	want := "norn-75-a-preview-address-exec-01abc-43000.preview.norn.site"
	if host != want {
		t.Fatalf(
			"the host came out as %q, want %q. This is the only thing the gateway routes by, "+
				"so a host nobody agrees on reaches nothing",
			host, want,
		)
	}
}

func TestAPreviewHostIsLowerCaseBecauseAnExecutionIdIsNot(t *testing.T) {
	host := entity.PreviewHost(
		"NORN-75", "web", "exec-01M0QEGQBR", 43000,
		entity.PreviewBySubdomain, "preview.norn.site",
	)

	if host != "norn-75-web-exec-01m0qegqbr-43000.preview.norn.site" {
		t.Fatalf(
			"the host came out as %q. An execution id is upper-case and a hostname is not "+
				"case-sensitive, so a browser lower-casing it would ask about a host norn has "+
				"never heard of and be turned away from its own preview",
			host,
		)
	}
}

func TestAServerServingNoPreviewDomainComposesNoHostAtAll(t *testing.T) {
	host := entity.PreviewHost(
		"NORN-75", "A preview address", "exec-01ABC", 43000, entity.PreviewBySubdomain, "",
	)

	if host != "" {
		t.Fatalf("a server with no preview domain composed %q", host)
	}
}

func TestAPreviewWithNoHostAnswersWithNoAddressAtAll(t *testing.T) {
	preview := entity.PreviewSession{Name: "web", State: entity.PreviewOpen}

	if address := preview.URL("https"); address != "" {
		t.Fatalf(
			"a preview with no host answered with %q. Handing somebody an address that resolves "+
				"nowhere is worse than admitting there is none",
			address,
		)
	}
}

func TestAPathModePreviewPutsItsPortInThePathRatherThanTheHost(t *testing.T) {
	preview := entity.PreviewSession{
		Name:  "web",
		Mode:  entity.PreviewByPath,
		Host:  "norn-75-exec-01abc.preview.norn.site",
		Port:  43000,
		Path:  "/app",
		State: entity.PreviewOpen,
	}

	want := "https://norn-75-exec-01abc.preview.norn.site/43000/app"
	if got := preview.URL("https"); got != want {
		t.Fatalf(
			"the address came out as %q, want %q. One host serves the whole run in that mode, "+
				"so the port is what tells two previews of it apart",
			got, want,
		)
	}
}

func TestAPreviewIsRefusedWithoutThePortItsAddressIsDerivedFrom(t *testing.T) {
	preview := entity.PreviewSession{
		Name:    "web",
		Service: "web",
		Mode:    entity.PreviewBySubdomain,
		State:   entity.PreviewOpen,
	}

	if err := entity.ValidatePreviewSession("preview", preview); err == nil {
		t.Fatal(
			"a preview carrying no port was accepted. Its address is derived from the port, " +
				"so a report without one has none of its own and would take the address " +
				"another preview of the run already holds",
		)
	}
}

func TestAPreviewIsRefusedWhenItsNameCouldNotBeASubdomain(t *testing.T) {
	cases := map[string]string{
		"an empty name":        "",
		"an upper-case name":   "Web",
		"a name with a dot":    "web.app",
		"a name with a stop":   "-web",
		"a name past the cap":  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"a name with a slash":  "web/app",
		"a name with a colon":  "web:8080",
		"a name with a space":  "web app",
		"a name with an equal": "web=1",
	}

	for name, given := range cases {
		t.Run(name, func(t *testing.T) {
			field := entity.ValidatePreviewName("name", given)
			if field.Code == "" {
				t.Fatalf(
					"%q was accepted as a preview name. It becomes a label in a hostname, so a "+
						"name that is not one produces an address no browser resolves",
					given,
				)
			}
		})
	}
}

func TestAPreviewSessionRefusesAPathThatLeavesTheService(t *testing.T) {
	preview := entity.PreviewSession{
		Name:    "web",
		Service: "web",
		Mode:    entity.PreviewBySubdomain,
		State:   entity.PreviewOpen,
		Port:    43000,
		Path:    "app",
	}

	if err := entity.ValidatePreviewSession("preview", preview); err == nil {
		t.Fatal("a path that does not begin with a slash was accepted as a preview path")
	}
}

func TestAShareLinkOnlyWorksInsideItsOwnWindowAndBeforeItIsWithdrawn(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	cases := map[string]struct {
		link    entity.PreviewShareLink
		refused error
	}{
		"inside its window": {
			link: entity.PreviewShareLink{ExpiresAt: now.Add(time.Hour)},
		},
		"exactly at its deadline": {
			link:    entity.PreviewShareLink{ExpiresAt: now},
			refused: entity.ErrPreviewShareExpired,
		},
		"past its deadline": {
			link:    entity.PreviewShareLink{ExpiresAt: now.Add(-time.Second)},
			refused: entity.ErrPreviewShareExpired,
		},
		"withdrawn while still inside its window": {
			link: entity.PreviewShareLink{
				ExpiresAt: now.Add(time.Hour),
				RevokedAt: now.Add(-time.Minute),
			},
			refused: entity.ErrPreviewShareRevoked,
		},
	}

	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			err := want.link.Usable(now)
			if want.refused == nil && err != nil {
				t.Fatalf("a link that should still work was refused: %v", err)
			}

			if want.refused != nil && err != want.refused {
				t.Fatalf(
					"the link was refused with %v, want %v. Withdrawing a link has to take "+
						"effect at once, and an expiry that is off by one keeps a link alive "+
						"past what somebody was told",
					err, want.refused,
				)
			}
		})
	}
}

func TestAPasscodeIsEitherAbsentOrLongEnoughToBeWorthAsking(t *testing.T) {
	cases := map[string]struct {
		passcode string
		refused  bool
	}{
		"no passcode at all":   {passcode: ""},
		"a passcode that fits": {passcode: "letmein"},
		"a passcode too short": {passcode: "abc", refused: true},
		"a passcode too long": {
			passcode: string(make([]byte, entity.PreviewSharePasscodeMaxLen+1)),
			refused:  true,
		},
	}

	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			field := entity.ValidatePreviewSharePasscode("passcode", want.passcode)
			if want.refused != (field.Code != "") {
				t.Fatalf(
					"a passcode of %d characters was refused=%v, want %v",
					len(want.passcode), field.Code != "", want.refused,
				)
			}
		})
	}
}

func TestAPreviewGrantOnlyOpensThePreviewItWasMintedFor(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	previewID := uuid.New()

	grant := entity.PreviewGrant{
		Audience:  entity.PreviewGrantAudience,
		PreviewID: previewID,
		ExpiresAt: now.Add(time.Minute),
	}

	if grant.Spent(previewID, now) {
		t.Fatal("a fresh grant was read as spent for the preview it was minted for")
	}

	if !grant.Spent(uuid.New(), now) {
		t.Fatal(
			"a grant opened a preview it was not minted for. One run's viewer would then reach " +
				"another run's service by pointing the same cookie at a different host",
		)
	}

	if !grant.Spent(previewID, now.Add(time.Minute)) {
		t.Fatal("a grant at its own deadline was still accepted")
	}

	wrong := grant
	wrong.Audience = "session"

	if !wrong.Spent(previewID, now) {
		t.Fatal(
			"a grant minted for something other than a preview was accepted. The audience is " +
				"what stops one kind of token being spent as another",
		)
	}
}

func TestAShareLinkViewerIsCountedApartFromTheMemberWhoMintedIt(t *testing.T) {
	linkID := uuid.New()
	accountID := uuid.New()

	link := entity.PreviewGrant{LinkID: linkID, AccountID: accountID}
	member := entity.PreviewGrant{AccountID: accountID}

	if link.Viewer() == member.Viewer() {
		t.Fatal(
			"a share-link viewer and a member came out as the same viewer, so one look would " +
				"silence the audit line for the other",
		)
	}
}

func TestAPreviewShareTokenIsStoredOnlyAsItsHash(t *testing.T) {
	token, hash, err := entity.NewPreviewShareToken()
	if err != nil {
		t.Fatalf("mint a share token: %v", err)
	}

	if len(hash) != 32 {
		t.Fatalf("the stored hash was %d bytes, want a sha256 digest", len(hash))
	}

	if string(hash) == token {
		t.Fatal("the token was stored as itself; a database read would then hand out live links")
	}

	again, _, err := entity.NewPreviewShareToken()
	if err != nil {
		t.Fatalf("mint a second share token: %v", err)
	}

	if again == token {
		t.Fatal("two share tokens came out the same")
	}
}
