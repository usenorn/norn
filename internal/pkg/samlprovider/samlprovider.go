package samlprovider

import (
	"context"
	"crypto/x509"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/crewjam/saml"
	dsig "github.com/russellhaering/goxmldsig"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/samlkey"
)

type Client struct {
	http *http.Client
	skew time.Duration
}

func New(cfg config.SAML) *Client {
	saml.MaxClockSkew = cfg.MaxClockSkew
	saml.MaxIssueDelay = cfg.MaxIssueDelay

	return &Client{
		http: &http.Client{
			Timeout:   cfg.RequestTimeout,
			Transport: &cappedTransport{inner: http.DefaultTransport, limit: cfg.MaxResponseSize},
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		skew: cfg.MaxClockSkew,
	}
}

type cappedTransport struct {
	inner http.RoundTripper
	limit int64
}

func (t *cappedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.inner.RoundTrip(request)
	if err != nil {
		return nil, err
	}

	response.Body = struct {
		io.Reader
		io.Closer
	}{io.LimitReader(response.Body, t.limit), response.Body}

	return response, nil
}

func (c *Client) Fetch(ctx context.Context, metadataURL string) (entity.SAMLDescriptor, error) {
	if err := entity.ValidateSAMLMetadataURL(metadataURL); err != nil {
		return entity.SAMLDescriptor{}, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURL, nil)
	if err != nil {
		return entity.SAMLDescriptor{}, entity.SSOFailure(
			entity.SSOStageMetadata,
			"That metadata address could not be used.",
			err,
		)
	}

	response, err := c.http.Do(request)
	if err != nil {
		return entity.SAMLDescriptor{}, entity.SSOFailure(
			entity.SSOStageMetadata,
			"Norn could not reach the provider metadata at that address.",
			err,
		)
	}

	defer func() { _ = response.Body.Close() }()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return entity.SAMLDescriptor{}, entity.SSOFailure(
			entity.SSOStageMetadata,
			"The provider metadata could not be read.",
			err,
		)
	}

	if response.StatusCode != http.StatusOK {
		return entity.SAMLDescriptor{}, entity.NewSSOError(
			entity.SSOStageMetadata,
			"The provider metadata address answered with "+response.Status+".",
		)
	}

	return Parse(body)
}

func Parse(document []byte) (entity.SAMLDescriptor, error) {
	var descriptor saml.EntityDescriptor

	if err := xml.Unmarshal(document, &descriptor); err != nil {
		var entities saml.EntitiesDescriptor
		if nested := xml.Unmarshal(document, &entities); nested != nil || len(entities.EntityDescriptors) == 0 {
			return entity.SAMLDescriptor{}, entity.SSOFailure(
				entity.SSOStageMetadata,
				"That does not read as SAML metadata.",
				err,
			)
		}

		descriptor = entities.EntityDescriptors[0]
	}

	read := entity.SAMLDescriptor{EntityID: descriptor.EntityID}

	for _, idp := range descriptor.IDPSSODescriptors {
		for _, keyDescriptor := range idp.KeyDescriptors {
			if keyDescriptor.Use != "" && keyDescriptor.Use != "signing" {
				continue
			}

			for _, certificate := range keyDescriptor.KeyInfo.X509Data.X509Certificates {
				if trimmed := strings.TrimSpace(certificate.Data); trimmed != "" {
					read.Certificates = append(read.Certificates, trimmed)
				}
			}
		}

		for _, service := range idp.SingleSignOnServices {
			if read.SSOURL == "" || service.Binding == saml.HTTPRedirectBinding {
				read.SSOURL = service.Location
			}
		}

		for _, service := range idp.SingleLogoutServices {
			if read.SLOURL == "" || service.Binding == saml.HTTPRedirectBinding {
				read.SLOURL = service.Location
			}
		}
	}

	if err := read.Validate(); err != nil {
		return entity.SAMLDescriptor{}, err
	}

	expiry, err := samlkey.EarliestExpiry(read.Certificates)
	if err != nil {
		return entity.SAMLDescriptor{}, entity.SSOFailure(
			entity.SSOStageCertificate,
			"The signing certificate in that metadata could not be read.",
			err,
		)
	}

	read.ExpiresAt = expiry

	return read, nil
}

type Endpoints struct {
	MetadataURL string
	ACSURL      string
}

func (c *Client) serviceProvider(
	connection entity.SAMLConnection,
	endpoints Endpoints,
) (*saml.ServiceProvider, error) {
	key, err := samlkey.ParsePrivateKey(connection.SPPrivateKey)
	if err != nil {
		return nil, entity.SSOFailure(
			entity.SSOStageCertificate,
			"Norn's own key for this workspace could not be read.",
			err,
		)
	}

	certificate, err := samlkey.ParseCertificate(connection.SPCertificate)
	if err != nil {
		return nil, entity.SSOFailure(
			entity.SSOStageCertificate,
			"Norn's own certificate for this workspace could not be read.",
			err,
		)
	}

	metadataURL, err := url.Parse(endpoints.MetadataURL)
	if err != nil {
		return nil, entity.SSOFailure(entity.SSOStageRequest, "Norn's metadata address is unusable.", err)
	}

	acsURL, err := url.Parse(endpoints.ACSURL)
	if err != nil {
		return nil, entity.SSOFailure(entity.SSOStageRequest, "Norn's callback address is unusable.", err)
	}

	idp, err := c.descriptorFor(connection)
	if err != nil {
		return nil, err
	}

	return &saml.ServiceProvider{
		EntityID:          connection.SPEntityID,
		Key:               key,
		Certificate:       certificate,
		MetadataURL:       *metadataURL,
		AcsURL:            *acsURL,
		IDPMetadata:       idp,
		AllowIDPInitiated: connection.AllowIDPInitiated,
		AuthnNameIDFormat: saml.EmailAddressNameIDFormat,
		SignatureMethod:   dsig.RSASHA256SignatureMethod,
		HTTPClient:        c.http,
	}, nil
}

func (c *Client) descriptorFor(connection entity.SAMLConnection) (*saml.EntityDescriptor, error) {
	certificates := make([]saml.X509Certificate, 0, len(connection.Descriptor.Certificates))

	for _, encoded := range connection.Descriptor.Certificates {
		parsed, err := samlkey.ParseCertificate(encoded)
		if err != nil {
			return nil, entity.SSOFailure(
				entity.SSOStageCertificate,
				"The provider's signing certificate could not be read.",
				err,
			)
		}

		certificates = append(certificates, saml.X509Certificate{
			Data: trimPEM(samlkey.MarshalCertificate(parsed)),
		})
	}

	return &saml.EntityDescriptor{
		EntityID: connection.Descriptor.EntityID,
		IDPSSODescriptors: []saml.IDPSSODescriptor{{
			SSODescriptor: saml.SSODescriptor{
				RoleDescriptor: saml.RoleDescriptor{
					ProtocolSupportEnumeration: "urn:oasis:names:tc:SAML:2.0:protocol",
					KeyDescriptors: []saml.KeyDescriptor{{
						Use: "signing",
						KeyInfo: saml.KeyInfo{
							X509Data: saml.X509Data{X509Certificates: certificates},
						},
					}},
				},
			},
			SingleSignOnServices: []saml.Endpoint{{
				Binding:  saml.HTTPRedirectBinding,
				Location: connection.Descriptor.SSOURL,
			}},
		}},
	}, nil
}

func (c *Client) Metadata(connection entity.SAMLConnection, endpoints Endpoints) ([]byte, error) {
	provider, err := c.serviceProvider(connection, endpoints)
	if err != nil {
		return nil, err
	}

	document, err := xml.MarshalIndent(signingOnly(provider.Metadata()), "", "  ")
	if err != nil {
		return nil, entity.SSOFailure(
			entity.SSOStageMetadata,
			"Norn could not render its own metadata.",
			err,
		)
	}

	return append([]byte(xml.Header), document...), nil
}

func signingOnly(descriptor *saml.EntityDescriptor) *saml.EntityDescriptor {
	for i := range descriptor.SPSSODescriptors {
		keys := descriptor.SPSSODescriptors[i].KeyDescriptors
		signing := make([]saml.KeyDescriptor, 0, len(keys))

		for _, key := range keys {
			if key.Use == "encryption" {
				continue
			}

			signing = append(signing, key)
		}

		descriptor.SPSSODescriptors[i].KeyDescriptors = signing
	}

	return descriptor
}

func (c *Client) AuthnRequest(
	connection entity.SAMLConnection,
	endpoints Endpoints,
	relayState string,
) (string, string, error) {
	provider, err := c.serviceProvider(connection, endpoints)
	if err != nil {
		return "", "", err
	}

	request, err := provider.MakeAuthenticationRequest(
		provider.GetSSOBindingLocation(saml.HTTPRedirectBinding),
		saml.HTTPRedirectBinding,
		saml.HTTPPostBinding,
	)
	if err != nil {
		return "", "", entity.SSOFailure(
			entity.SSOStageRequest,
			"Norn could not build a sign-in request for this provider.",
			err,
		)
	}

	target, err := request.Redirect(relayState, provider)
	if err != nil {
		return "", "", entity.SSOFailure(
			entity.SSOStageRequest,
			"Norn could not build a sign-in request for this provider.",
			err,
		)
	}

	return target.String(), request.ID, nil
}

func (c *Client) Parse(
	connection entity.SAMLConnection,
	endpoints Endpoints,
	document []byte,
	requestIDs []string,
) (entity.SAMLAssertion, error) {
	provider, err := c.serviceProvider(connection, endpoints)
	if err != nil {
		return entity.SAMLAssertion{}, err
	}

	acsURL, err := url.Parse(endpoints.ACSURL)
	if err != nil {
		return entity.SAMLAssertion{}, entity.SSOFailure(
			entity.SSOStageRequest,
			"Norn's callback address is unusable.",
			err,
		)
	}

	assertion, err := provider.ParseXMLResponse(document, requestIDs, *acsURL)
	if err != nil {
		return entity.SAMLAssertion{}, classify(err)
	}

	return read(assertion), nil
}

func read(assertion *saml.Assertion) entity.SAMLAssertion {
	found := entity.SAMLAssertion{
		ID:         assertion.ID,
		Attributes: map[string][]string{},
	}

	if assertion.Subject != nil {
		if assertion.Subject.NameID != nil {
			found.NameID = assertion.Subject.NameID.Value
			found.NameIDFormat = assertion.Subject.NameID.Format
		}

		for _, confirmation := range assertion.Subject.SubjectConfirmations {
			if confirmation.SubjectConfirmationData == nil {
				continue
			}

			if answered := confirmation.SubjectConfirmationData.InResponseTo; answered != "" {
				found.InResponseTo = answered

				break
			}
		}
	}

	if assertion.Conditions != nil {
		found.NotOnOrAfter = assertion.Conditions.NotOnOrAfter
	}

	for _, statement := range assertion.AttributeStatements {
		for _, attribute := range statement.Attributes {
			values := make([]string, 0, len(attribute.Values))
			for _, value := range attribute.Values {
				values = append(values, value.Value)
			}

			for _, name := range []string{attribute.Name, attribute.FriendlyName} {
				if name == "" {
					continue
				}

				found.Attributes[name] = append(found.Attributes[name], values...)
			}
		}
	}

	return found
}

func trimPEM(encoded string) string {
	out := make([]string, 0, 8)

	for _, line := range strings.Split(encoded, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "-----") {
			continue
		}

		out = append(out, trimmed)
	}

	return strings.Join(out, "")
}

func Fingerprint(certificate *x509.Certificate) string {
	return fmt.Sprintf("%x", certificate.SerialNumber)
}
