package licence_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/licence"
)

func TestAnInstanceWithNoKeyIsUnlicensedRatherThanBroken(t *testing.T) {
	resolved, err := licence.Resolve(config.Licence{})
	if err != nil {
		t.Fatalf(
			"an instance with no licence key refused to start: %v. Self-hosting without a "+
				"licence is the ordinary case, not a misconfiguration.",
			err,
		)
	}

	if resolved.Status(time.Now(), 0) != entity.LicenceAbsent {
		t.Fatal("an absent key produced a valid licence")
	}

	if resolved.Features.Audit {
		t.Fatal("an absent key granted a paid feature")
	}
}

func TestAKeyNobodyIssuedIsRefused(t *testing.T) {
	for name, key := range map[string]string{
		"not a key at all":  "hello",
		"no signature":      "eyJob2xkZXIiOiJhY21lIn0",
		"unparseable parts": "@@@.@@@",
		"forged signature":  "eyJob2xkZXIiOiJhY21lIn0.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	} {
		if _, err := licence.Verify(key); err == nil {
			t.Errorf(
				"%s was accepted. Anything short of a signature this build can verify has to be "+
					"refused, or the gate is decoration.",
				name,
			)
		}
	}
}

func TestTheLicenceCannotBecomeWhereSeatsOrSingleSignOnGetPriced(t *testing.T) {
	forbidden := []string{
		"sso", "singlesignon", "saml", "oidc",
		"agent", "seat", "member", "user", "account", "workspace", "issue", "storage",
	}

	surface := reflect.TypeOf(entity.LicenceFeatures{})

	for i := range surface.NumField() {
		field := strings.ToLower(surface.Field(i).Name)

		for _, word := range forbidden {
			if strings.Contains(field, word) {
				t.Errorf(
					"entity.LicenceFeatures carries %q. Single sign-on is free on every install "+
						"and nothing about people, agents or workspaces is counted or sold; a "+
						"field here is where that would stop being true.",
					surface.Field(i).Name,
				)
			}
		}
	}

	if surface.NumField() == 0 {
		t.Fatal("the licence grants nothing at all, so the gate cannot be doing its job")
	}
}

func TestAnExpiredLicenceGrantsNothing(t *testing.T) {
	expired := entity.Licence{
		ExpiresAt: time.Now().Add(-time.Hour),
		Features:  entity.LicenceFeatures{Audit: true},
	}

	if expired.Status(time.Now(), 0) == entity.LicenceActive {
		t.Fatal("a licence that ran out yesterday still reported itself valid")
	}

	if expired.Permits(time.Now(), 0, entity.FeatureAudit) {
		t.Fatal(
			"an expired licence still permitted a paid feature. The features it names are only " +
				"meaningful while it is in date.",
		)
	}
}
