package entity

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

var DefaultEmailAttributes = []string{
	"email",
	"mail",
	"emailAddress",
	"urn:oid:0.9.2342.19200300.100.1.3",
	"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
}

var DefaultNameAttributes = []string{
	"displayName",
	"name",
	"cn",
	"urn:oid:2.16.840.1.113730.3.1.241",
	"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
	"http://schemas.microsoft.com/identity/claims/displayname",
}

var DefaultGroupAttributes = []string{
	"groups",
	"memberOf",
	"Role",
	"http://schemas.xmlsoap.org/claims/Group",
	"http://schemas.microsoft.com/ws/2008/06/identity/claims/groups",
}

type SAMLAttributeMapping struct {
	Email  string
	Name   string
	Groups string
}

func (m SAMLAttributeMapping) EmailNames() []string {
	return chosenOrDefault(m.Email, DefaultEmailAttributes)
}

func (m SAMLAttributeMapping) NameNames() []string {
	return chosenOrDefault(m.Name, DefaultNameAttributes)
}

func (m SAMLAttributeMapping) GroupNames() []string {
	return chosenOrDefault(m.Groups, DefaultGroupAttributes)
}

func chosenOrDefault(chosen string, fallback []string) []string {
	if trimmed := strings.TrimSpace(chosen); trimmed != "" {
		return []string{trimmed}
	}

	return fallback
}

type SAMLDescriptor struct {
	EntityID     string
	SSOURL       string
	SLOURL       string
	Certificates []string
	ExpiresAt    time.Time
}

func (d SAMLDescriptor) Validate() error {
	if strings.TrimSpace(d.EntityID) == "" {
		return NewSSOError(
			SSOStageMetadata,
			"The provider metadata does not name an entity ID.",
		)
	}

	if strings.TrimSpace(d.SSOURL) == "" {
		return NewSSOError(
			SSOStageMetadata,
			"The provider metadata has no sign-in URL for the HTTP-Redirect or HTTP-POST binding.",
		)
	}

	if err := requireHTTPSURL(d.SSOURL); err != nil {
		return NewSSOError(
			SSOStageMetadata,
			"The provider's sign-in URL is not a usable https address.",
		)
	}

	if len(d.Certificates) == 0 {
		return NewSSOError(
			SSOStageCertificate,
			"The provider metadata carries no signing certificate, so no assertion could be trusted.",
		)
	}

	return nil
}

type SAMLConnection struct {
	WorkspaceID       uuid.UUID
	Descriptor        SAMLDescriptor
	MetadataURL       string
	SPEntityID        string
	SPPrivateKey      []byte
	SPCertificate     string
	AllowIDPInitiated bool
	Mapping           SAMLAttributeMapping
	Provisioning      bool
	AdminGroup        string
	VerifiedAt        *time.Time
	ExpiryNoticeDays  *int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (c SAMLConnection) Verified() bool { return c.VerifiedAt != nil }

func (c SAMLConnection) Validate() error {
	if err := c.Descriptor.Validate(); err != nil {
		return err
	}

	if strings.TrimSpace(c.SPEntityID) == "" {
		return NewSSOError(SSOStageMetadata, "Norn has no entity ID for this workspace.")
	}

	if len(c.SPPrivateKey) == 0 || strings.TrimSpace(c.SPCertificate) == "" {
		return NewSSOError(
			SSOStageCertificate,
			"Norn has no keypair for this workspace, so it cannot publish metadata.",
		)
	}

	return nil
}

const MaxSAMLClockSkew = 3 * time.Minute

func CertificateExpired(expiry, now time.Time) bool {
	return !expiry.After(now)
}

type SAMLAttempt struct {
	Purpose     SSOPurpose
	WorkspaceID uuid.UUID
	RequestID   string
	Correlator  string
	ReturnTo    string
	CreatedAt   time.Time
}

type SAMLAssertion struct {
	ID           string
	InResponseTo string
	NameID       string
	NameIDFormat string
	Attributes   map[string][]string
	NotOnOrAfter time.Time
}

func ValidateSAMLMetadataURL(address string) error {
	if err := requireHTTPSURL(address); err != nil {
		return NewSSOError(
			SSOStageMetadata,
			"The metadata address must be an https URL, for example "+
				"https://login.example.com/metadata.",
		)
	}

	return nil
}

func SAMLRequestMismatch(expected string, assertion SAMLAssertion) error {
	if expected != "" && assertion.InResponseTo == expected {
		return nil
	}

	return NewSSOError(
		SSOStageResponse,
		"The response does not answer any sign-in Norn started. Begin the sign-in again.",
	)
}

type SAMLIdentity struct {
	Subject string
	Email   string
	Name    string
	Groups  []string
}

func ResolveSAMLIdentity(
	assertion SAMLAssertion,
	mapping SAMLAttributeMapping,
) (SAMLIdentity, error) {
	if strings.TrimSpace(assertion.ID) == "" {
		return SAMLIdentity{}, NewSSOError(
			SSOStageResponse,
			"The assertion has no ID, so Norn cannot tell whether it has been used before.",
		)
	}

	identity := SAMLIdentity{
		Subject: strings.TrimSpace(assertion.NameID),
		Email:   NormalizeEmail(first(assertion.Attributes, mapping.EmailNames())),
		Name:    first(assertion.Attributes, mapping.NameNames()),
		Groups:  all(assertion.Attributes, mapping.GroupNames()),
	}

	if identity.Email == "" && looksLikeEmail(assertion.NameID) {
		identity.Email = NormalizeEmail(assertion.NameID)
	}

	if identity.Subject == "" {
		return SAMLIdentity{}, NewSSOError(
			SSOStageAttributes,
			"The assertion carried no NameID, so Norn has nothing durable to recognise this "+
				"person by. Configure your provider to release one.",
		)
	}

	if identity.Email == "" {
		return SAMLIdentity{}, NewSSOError(
			SSOStageAttributes,
			"The assertion carried no email address. Map the attribute your provider releases it under.",
		)
	}

	return identity, nil
}

func looksLikeEmail(value string) bool {
	at := strings.Index(value, "@")

	return at > 0 && at < len(value)-1 && !strings.Contains(value, " ")
}

func first(attributes map[string][]string, names []string) string {
	for _, name := range names {
		for _, value := range attributes[name] {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				return trimmed
			}
		}
	}

	return ""
}

func all(attributes map[string][]string, names []string) []string {
	found := make([]string, 0)

	for _, name := range names {
		for _, value := range attributes[name] {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				found = append(found, trimmed)
			}
		}
	}

	return found
}

func SAMLConditionsFailure(notBefore, notOnOrAfter, now time.Time) error {
	return NewSSOError(
		SSOStageConditions,
		"The provider's response is only valid between "+notBefore.UTC().Format(time.RFC3339)+
			" and "+notOnOrAfter.UTC().Format(time.RFC3339)+", but this instance thinks it is "+
			now.UTC().Format(time.RFC3339)+". The two clocks disagree by more than Norn allows.",
	)
}

func SAMLReplayFailure() error {
	return NewSSOError(
		SSOStageReplay,
		"This sign-in has already been used. Start a new one rather than reloading the page.",
	)
}
