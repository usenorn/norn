package entity_test

import (
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestAProjectLinkRefusesAnythingThatIsNotAWebAddress(t *testing.T) {
	for _, test := range []struct {
		address string
		code    string
	}{
		{address: "https://docs.example.com/spec", code: ""},
		{address: "http://example.com", code: ""},
		{address: "  https://example.com/x  ", code: ""},
		{address: "", code: entity.ValidationCodeRequired},
		{address: "javascript:alert(1)", code: entity.ValidationCodeMalformed},
		{address: "data:text/html;base64,PHNjcmlwdD4=", code: entity.ValidationCodeMalformed},
		{address: "file:///etc/passwd", code: entity.ValidationCodeMalformed},
		{address: "docs.example.com", code: entity.ValidationCodeMalformed},
		{address: "https://", code: entity.ValidationCodeMalformed},
	} {
		t.Run(test.address, func(t *testing.T) {
			got := entity.ValidateProjectLinkURL("url", test.address).Code

			if got != test.code {
				t.Fatalf(
					"ValidateProjectLinkURL(%q) code = %q, want %q. A link is rendered for other "+
						"people to click, so a scheme that runs script must never reach the page.",
					test.address,
					got,
					test.code,
				)
			}
		})
	}
}
