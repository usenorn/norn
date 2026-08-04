package samlprovider

import (
	"errors"
	"fmt"
	"testing"

	"github.com/crewjam/saml"

	"github.com/usenorn/norn/internal/entity"
)

func stageOf(t *testing.T, err error) entity.SSOStage {
	t.Helper()

	failure, ok := entity.AsSSOError(err)
	if !ok {
		t.Fatalf("%v is not a staged failure", err)
	}

	return failure.Stage
}

func wrapped(cause error) error {
	return &saml.InvalidResponseError{PrivateErr: cause}
}

func TestUnsignedAndBadlySignedAreToldApart(t *testing.T) {
	unsigned := classify(wrapped(errors.New("signature element not present")))
	tampered := classify(wrapped(fmt.Errorf(
		"cannot validate signature on Assertion: %w",
		errors.New("Could not verify certificate against trusted certs"),
	)))

	if stageOf(t, unsigned) != entity.SSOStageSignature {
		t.Fatalf("an unsigned assertion reported %q", stageOf(t, unsigned))
	}

	if stageOf(t, tampered) != entity.SSOStageSignature {
		t.Fatalf("a wrongly signed assertion reported %q", stageOf(t, tampered))
	}

	unsignedFailure, _ := entity.AsSSOError(unsigned)
	tamperedFailure, _ := entity.AsSSOError(tampered)

	if unsignedFailure.Message == tamperedFailure.Message {
		t.Fatal(
			"an unsigned assertion and one signed by the wrong key produce identical copy. " +
				"They need different fixes — turn signing on at the provider, versus correct the " +
				"certificate here — so an administrator must be able to tell which happened.",
		)
	}
}

func TestClockSkewIsNotReportedAsSomethingElse(t *testing.T) {
	for name, cause := range map[string]string{
		"conditions not yet valid": "assertion Conditions is not yet valid",
		"conditions expired":       "assertion Conditions is expired",
		"confirmation expired":     "assertion SubjectConfirmationData is expired",
		"issue instant expired":    "response IssueInstant expired at 2026-08-04T12:00:00Z",
	} {
		err := classify(wrapped(errors.New(cause)))

		if stage := stageOf(t, err); stage != entity.SSOStageConditions {
			t.Errorf("%s: stage %q, want %q", name, stage, entity.SSOStageConditions)
		}

		failure, _ := entity.AsSSOError(err)
		if failure.Detail != cause {
			t.Errorf("%s: the library's own words were lost", name)
		}
	}
}

func TestAnUnrecognisedFailureStillNamesAStageAndKeepsTheCause(t *testing.T) {
	err := classify(wrapped(errors.New("something nobody anticipated")))

	if stage := stageOf(t, err); stage != entity.SSOStageResponse {
		t.Fatalf("stage %q, want %q as the catch-all", stage, entity.SSOStageResponse)
	}

	failure, _ := entity.AsSSOError(err)
	if failure.Detail != "something nobody anticipated" {
		t.Fatalf("detail %q lost the cause", failure.Detail)
	}
}

func TestAudienceAndIssuerMismatchesAreTheirOwnAdvice(t *testing.T) {
	audience := classify(wrapped(errors.New(
		`assertion Conditions AudienceRestriction does not contain "https://norn/sp"`)))
	issuer := classify(wrapped(errors.New(
		`response Issuer does not match the IDP metadata (expected "https://idp")`)))

	for name, err := range map[string]error{"audience": audience, "issuer": issuer} {
		if stage := stageOf(t, err); stage != entity.SSOStageResponse {
			t.Errorf("%s: stage %q", name, stage)
		}
	}

	audienceFailure, _ := entity.AsSSOError(audience)
	issuerFailure, _ := entity.AsSSOError(issuer)

	if audienceFailure.Message == issuerFailure.Message {
		t.Fatal("a wrong audience and a wrong issuer give identical advice")
	}
}

func TestClassifyingNothingIsNothing(t *testing.T) {
	if classify(nil) != nil {
		t.Fatal("classify invented a failure from a nil error")
	}
}
