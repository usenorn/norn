package entity_test

import (
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestATargetReportsWhatItCannotDoRatherThanDoingNothing(t *testing.T) {
	limited := entity.SCMCapabilitySet{entity.CapabilityIssues, entity.CapabilityLabels}

	missing := limited.Missing()

	if len(missing) == 0 {
		t.Fatal("a target offering two of the features reported nothing missing")
	}

	for _, capability := range missing {
		if limited.Has(capability) {
			t.Errorf("%s is both held and reported missing", capability)
		}
	}

	if !limited.Has(entity.CapabilityIssues) {
		t.Error("a held capability reads as absent")
	}
}

func TestAFullTargetIsMissingNothing(t *testing.T) {
	full := entity.SCMCapabilitySet(entity.SCMCapabilities())

	if got := full.Missing(); len(got) != 0 {
		t.Fatalf("a target offering everything reported %v missing", got)
	}
}

func TestSomethingThisInstanceDoesNotKnowAboutIsDropped(t *testing.T) {
	set := entity.SCMCapabilitiesFrom([]string{"webhooks", "teleportation", "webhooks", ""})

	if len(set) != 1 || !set.Has(entity.CapabilityWebhooks) {
		t.Fatalf(
			"SCMCapabilitiesFrom = %v; a value this instance cannot act on must not be stored "+
				"as though it could, and a repeat is still one capability",
			set,
		)
	}
}

func TestOnlySomethingThisInstanceCanReadIsAcceptedAsACertificate(t *testing.T) {
	for _, refused := range []string{
		"not a certificate",
		"-----BEGIN CERTIFICATE-----\nnot base64\n-----END CERTIFICATE-----",
		"-----BEGIN RSA PRIVATE KEY-----\nMIIB\n-----END RSA PRIVATE KEY-----",
	} {
		if field := entity.ValidateSCMCertificate("caCertificate", refused); field.Field == "" {
			t.Errorf(
				"ValidateSCMCertificate(%.30q) was accepted; storing one this instance cannot "+
					"read leaves every call failing with a TLS error naming nothing",
				refused,
			)
		}
	}

	if field := entity.ValidateSCMCertificate("caCertificate", "   "); field.Field != "" {
		t.Error("an empty certificate means the system trust store, not an error")
	}
}

func TestTrustIsOnlyCustomWhenSomethingWasGranted(t *testing.T) {
	if (entity.SCMTrust{}).Custom() {
		t.Error("a connection granted nothing must use the instance's ordinary rules")
	}

	if !(entity.SCMTrust{AllowPrivateAddress: true}).Custom() {
		t.Error("a private-address exception is a custom trust")
	}

	if !(entity.SCMTrust{CACertificate: "x"}).Custom() {
		t.Error("a supplied authority is a custom trust")
	}
}
