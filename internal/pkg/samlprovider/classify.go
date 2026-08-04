package samlprovider

import (
	"errors"
	"strings"

	"github.com/crewjam/saml"

	"github.com/usenorn/norn/internal/entity"
)

const (
	unsignedMarker  = "signature element not present"
	signatureMarker = "cannot validate signature on"
)

var conditionMarkers = []string{
	"assertion Conditions is not yet valid",
	"assertion Conditions is expired",
	"assertion SubjectConfirmationData is expired",
	"IssueInstant expired",
	"expired on ",
}

var requestMarkers = []string{
	"`InResponseTo` does not match any of the possible request IDs",
	"does not match any of the possible request IDs",
}

func classify(err error) error {
	if err == nil {
		return nil
	}

	cause := err

	var invalid *saml.InvalidResponseError
	if errors.As(err, &invalid) && invalid.PrivateErr != nil {
		cause = invalid.PrivateErr
	}

	message := cause.Error()

	switch {
	case strings.Contains(message, unsignedMarker):
		return entity.SSOFailure(
			entity.SSOStageSignature,
			"The provider's response was not signed. Norn only accepts signed assertions.",
			cause,
		)

	case strings.Contains(message, signatureMarker):
		return entity.SSOFailure(
			entity.SSOStageSignature,
			"The signature on the provider's response did not verify against the certificate "+
				"configured here.",
			cause,
		)

	case containsAny(message, conditionMarkers):
		return entity.SSOFailure(
			entity.SSOStageConditions,
			"The provider's response arrived outside the window it is valid for. This is almost "+
				"always the clocks on this instance and the provider disagreeing.",
			cause,
		)

	case containsAny(message, requestMarkers):
		return entity.SSOFailure(
			entity.SSOStageResponse,
			"The response does not answer any sign-in Norn started. Begin the sign-in again.",
			cause,
		)

	case strings.Contains(message, "AudienceRestriction"):
		return entity.SSOFailure(
			entity.SSOStageResponse,
			"The provider addressed the response to a different application. Check that the "+
				"entity ID registered there matches Norn's.",
			cause,
		)

	case strings.Contains(message, "Issuer does not match") ||
		strings.Contains(message, "issuer is not"):
		return entity.SSOFailure(
			entity.SSOStageResponse,
			"The response came from a different provider than the one configured here.",
			cause,
		)

	default:
		return entity.SSOFailure(
			entity.SSOStageResponse,
			"Norn could not read the provider's response.",
			cause,
		)
	}
}

func containsAny(message string, markers []string) bool {
	for _, marker := range markers {
		if strings.Contains(message, marker) {
			return true
		}
	}

	return false
}
