package entity

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"slices"
	"strings"
)

const SCMCertificateMaxLen = 32768

var ErrSCMCertificateInvalid = errors.New("that is not a certificate this instance can read")

type SCMCapability string

const (
	CapabilityWebhooks     SCMCapability = "webhooks"
	CapabilityReviews      SCMCapability = "reviews"
	CapabilityChecks       SCMCapability = "checks"
	CapabilityChangedPaths SCMCapability = "changed_paths"
	CapabilityIssues       SCMCapability = "issues"
	CapabilityLabels       SCMCapability = "labels"
	CapabilityAssignees    SCMCapability = "assignees"
)

func SCMCapabilities() []SCMCapability {
	return []SCMCapability{
		CapabilityWebhooks,
		CapabilityReviews,
		CapabilityChecks,
		CapabilityChangedPaths,
		CapabilityIssues,
		CapabilityLabels,
		CapabilityAssignees,
	}
}

func (c SCMCapability) Valid() bool {
	return slices.Contains(SCMCapabilities(), c)
}

type SCMCapabilitySet []SCMCapability

func (set SCMCapabilitySet) Has(capability SCMCapability) bool {
	return slices.Contains(set, capability)
}

func (set SCMCapabilitySet) Missing() []SCMCapability {
	missing := make([]SCMCapability, 0)

	for _, capability := range SCMCapabilities() {
		if !set.Has(capability) {
			missing = append(missing, capability)
		}
	}

	return missing
}

func (set SCMCapabilitySet) Strings() []string {
	values := make([]string, len(set))
	for i, capability := range set {
		values[i] = string(capability)
	}

	return values
}

func SCMCapabilitiesFrom(values []string) SCMCapabilitySet {
	set := make(SCMCapabilitySet, 0, len(values))

	for _, value := range values {
		capability := SCMCapability(strings.TrimSpace(value))
		if capability.Valid() && !set.Has(capability) {
			set = append(set, capability)
		}
	}

	return set
}

type SCMTrust struct {
	AllowPrivateAddress bool
	CACertificate       string
}

func (t SCMTrust) Custom() bool {
	return t.AllowPrivateAddress || strings.TrimSpace(t.CACertificate) != ""
}

func ValidateSCMCertificate(field, certificate string) FieldError {
	trimmed := strings.TrimSpace(certificate)

	if trimmed == "" {
		return FieldError{}
	}

	if len(trimmed) > SCMCertificateMaxLen {
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	}

	if _, err := ParseSCMCertificates(trimmed); err != nil {
		return FieldError{Field: field, Code: ValidationCodeUnsupportedValue}
	}

	return FieldError{}
}

func ParseSCMCertificates(bundle string) ([]*x509.Certificate, error) {
	rest := []byte(strings.TrimSpace(bundle))
	found := make([]*x509.Certificate, 0, 2)

	for len(rest) > 0 {
		var block *pem.Block

		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}

		if block.Type != "CERTIFICATE" {
			continue
		}

		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, ErrSCMCertificateInvalid
		}

		found = append(found, certificate)
	}

	if len(found) == 0 {
		return nil, ErrSCMCertificateInvalid
	}

	return found, nil
}
