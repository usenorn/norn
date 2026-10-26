package entity_test

import (
	"strings"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

func TestAClaimedDomainIsNormalisedOrRefused(t *testing.T) {
	cases := []struct {
		raw      string
		want     string
		wantCode string
	}{
		{raw: "Northwind.CO", want: "northwind.co"},
		{raw: "  eu.northwind.co.  ", want: "eu.northwind.co"},
		{raw: "north-wind.io", want: "north-wind.io"},
		{raw: "", wantCode: entity.ValidationCodeRequired},
		{raw: "localhost", wantCode: entity.ValidationCodeMalformed},
		{raw: "northwind..co", wantCode: entity.ValidationCodeMalformed},
		{raw: "-northwind.co", wantCode: entity.ValidationCodeMalformed},
		{raw: "northwind-.co", wantCode: entity.ValidationCodeMalformed},
		{raw: "https://northwind.co", wantCode: entity.ValidationCodeMalformed},
		{raw: "rae@northwind.co", wantCode: entity.ValidationCodeMalformed},
		{raw: "nörthwind.co", wantCode: entity.ValidationCodeMalformed},
		{raw: strings.Repeat("a", entity.WorkspaceDomainLabelMaxLen+1) + ".co", wantCode: entity.ValidationCodeMalformed},
		{raw: "gmail.com", wantCode: entity.ValidationCodePersonalDomain},
		{raw: "outlook.co.uk", wantCode: entity.ValidationCodePersonalDomain},
		{raw: "proton.me", wantCode: entity.ValidationCodePersonalDomain},
	}

	for _, tc := range cases {
		got, problem := entity.ValidateWorkspaceDomain("domain", tc.raw)

		if problem.Code != tc.wantCode {
			t.Errorf("%q: code = %q, want %q", tc.raw, problem.Code, tc.wantCode)
		}

		if got != tc.want {
			t.Errorf("%q: domain = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestAnEmailRoutesOnItsDomainAlone(t *testing.T) {
	cases := []struct {
		email  string
		want   string
		wantOK bool
	}{
		{email: "Rae@Northwind.co", want: "northwind.co", wantOK: true},
		{email: "rae@gmail.com", want: "gmail.com", wantOK: true},
		{email: "no-at-sign", wantOK: false},
		{email: "rae@", wantOK: false},
		{email: "rae@intranet", wantOK: false},
	}

	for _, tc := range cases {
		got, ok := entity.EmailDomain(tc.email)
		if ok != tc.wantOK || got != tc.want {
			t.Errorf("%q: got (%q, %v), want (%q, %v)", tc.email, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestADomainRecordCarriesItsToken(t *testing.T) {
	token, err := entity.NewWorkspaceDomainToken()
	if err != nil {
		t.Fatalf("token: %v", err)
	}

	other, err := entity.NewWorkspaceDomainToken()
	if err != nil {
		t.Fatalf("token: %v", err)
	}

	if token == other {
		t.Fatal("two tokens were identical")
	}

	domain := entity.WorkspaceDomain{Token: token}

	if domain.Record() != "norn-verification="+token {
		t.Fatalf("record = %q", domain.Record())
	}

	if domain.Verified() {
		t.Fatal("a fresh domain reported itself verified")
	}

	at := time.Now()
	domain.VerifiedAt = &at

	if !domain.Verified() {
		t.Fatal("a domain with a verification time reported itself unverified")
	}
}
