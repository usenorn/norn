package entity_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func runFor(reference, title string) entity.Execution {
	return entity.Execution{ID: "exec-01M0SMJXBJ451KZ0MCQ6TY2GH1", IssueReference: reference, IssueTitle: title}
}

func TestAPreviewAddressIsTheIssueTheRunAndThePortAndNothingAnAgentChose(t *testing.T) {
	cases := map[string]struct {
		run     entity.Execution
		port    int
		mode    entity.PreviewMode
		domain  string
		address string
	}{
		"a subdomain per port": {
			run:     runFor("NORN-75", "A preview address must be one per task, execution and port"),
			port:    43000,
			mode:    entity.PreviewBySubdomain,
			domain:  "norn.ink",
			address: "norn-75-a-preview-address-exec-01m0smjxbj451kz0mcq6ty2gh1-43000.norn.ink",
		},
		"the same run on another port": {
			run:     runFor("NORN-75", "A preview address must be one per task, execution and port"),
			port:    5173,
			mode:    entity.PreviewBySubdomain,
			domain:  "norn.ink",
			address: "norn-75-a-preview-address-exec-01m0smjxbj451kz0mcq6ty2gh1-5173.norn.ink",
		},
		"one host for the whole run": {
			run:     runFor("NORN-75", "A preview address must be one per task, execution and port"),
			port:    43000,
			mode:    entity.PreviewByPath,
			domain:  "norn.ink",
			address: "norn-75-a-preview-address-must-exec-01m0smjxbj451kz0mcq6ty2gh1.norn.ink",
		},
		"a title that slugifies to nothing": {
			run:     runFor("NORN-75", "—"),
			port:    43000,
			mode:    entity.PreviewBySubdomain,
			domain:  "norn.ink",
			address: "norn-75-exec-01m0smjxbj451kz0mcq6ty2gh1-43000.norn.ink",
		},
		"a reference that leaves the title no room": {
			run:     runFor("ABCDE-123456789", "A preview address"),
			port:    43000,
			mode:    entity.PreviewBySubdomain,
			domain:  "norn.ink",
			address: "abcde-123456789-a-preview-exec-01m0smjxbj451kz0mcq6ty2gh1-43000.norn.ink",
		},
		"no domain to compose from": {
			run:     runFor("NORN-75", "A preview address"),
			port:    43000,
			mode:    entity.PreviewBySubdomain,
			domain:  "",
			address: "",
		},
	}

	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			got := entity.PreviewHost(want.run, want.port, want.mode, want.domain)
			if got != want.address {
				t.Fatalf(
					"the host came out as %q, want %q. This is the only thing the gateway "+
						"routes by, so a host nobody agrees on reaches nothing",
					got, want.address,
				)
			}
		})
	}
}

func TestNoIssueTitleEverPushesAPreviewLabelPastWhatDNSCarries(t *testing.T) {
	references := []string{"", "AB-1", "NORN-75", "ABCDE-123456789", strings.Repeat("LONG-9", 20)}
	titles := []string{
		"",
		"web",
		strings.Repeat("a very long title indeed ", 40),
		strings.Repeat("————— ", 40),
	}
	ports := []int{1, 43000, 65535}

	for _, reference := range references {
		for _, title := range titles {
			for _, port := range ports {
				for _, mode := range entity.PreviewModes() {
					host := entity.PreviewHost(runFor(reference, title), port, mode, "norn.ink")

					label, _, _ := strings.Cut(host, ".")
					if len(label) > channelv1.PreviewLabelMax {
						t.Fatalf(
							"%q is %d characters. DNS stops at %d, so the whole address is "+
								"refused before it ever reaches norn",
							label, len(label), channelv1.PreviewLabelMax,
						)
					}

					if strings.Contains(label, "--") || strings.HasSuffix(label, "-") {
						t.Fatalf(
							"%q holds an empty segment, which is not a label a browser "+
								"resolves", label,
						)
					}
				}
			}
		}
	}
}

func TestThePreviewLabelCarriesTheExecutionsOwnPrefixOnlyOnce(t *testing.T) {
	host := entity.PreviewHost(runFor("NORN-75", "A preview address"), 43000, entity.PreviewBySubdomain, "norn.ink")

	if strings.Contains(host, entity.ExecutionIDPrefix+entity.ExecutionIDPrefix) {
		t.Fatalf(
			"the host came out as %q. The execution id already carries its own prefix, so "+
				"adding a second one names a run that does not exist",
			host,
		)
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

func TestAPathModePreviewPutsItsPortInThePathRatherThanTheNameItWasGiven(t *testing.T) {
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
			"the address came out as %q, want %q. Two ports on one run share the host in this "+
				"mode, so the port is the only thing that tells them apart",
			got, want,
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
