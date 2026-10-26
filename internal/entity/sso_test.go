package entity_test

import (
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestOnlyAWellFormedAddressIsPassedToAProviderAsAHint(t *testing.T) {
	cases := map[string]string{
		" Rae@Northwind.co ":            "rae@northwind.co",
		"":                              "",
		"not an address":                "",
		"rae@northwind.co\r\nX-Inject:": "",
	}

	for raw, want := range cases {
		if got := entity.SSOLoginHint(raw); got != want {
			t.Errorf("SSOLoginHint(%q) = %q, want %q", raw, got, want)
		}
	}
}
