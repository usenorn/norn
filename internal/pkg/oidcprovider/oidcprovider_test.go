package oidcprovider_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/oidcprovider"
)

const discoveryPath = "/app/.well-known/openid-configuration"

func discoveryServer(t *testing.T, issuerFor func(base string) string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != discoveryPath {
			http.NotFound(w, r)

			return
		}

		base := "http://" + r.Host

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 issuerFor(base),
			"authorization_endpoint": base + "/app/authorize/",
			"token_endpoint":         base + "/app/token/",
			"jwks_uri":               base + "/app/jwks/",
			"userinfo_endpoint":      base + "/app/userinfo/",
		}); err != nil {
			t.Errorf("encoding the discovery document: %v", err)
		}
	}))

	t.Cleanup(server.Close)

	return server
}

func discoverer() *oidcprovider.Client {
	return oidcprovider.New(config.OIDC{RequestTimeout: 5 * time.Second, MaxResponseSize: 1 << 20})
}

func TestAnIssuerIsAcceptedWhicheverWayTheTrailingSlashIsTyped(t *testing.T) {
	server := discoveryServer(t, func(base string) string { return base + "/app/" })
	canonical := server.URL + "/app/"

	client := discoverer()

	for name, typed := range map[string]string{
		"as the provider advertises it": canonical,
		"without the trailing slash":    server.URL + "/app",
		"with surrounding whitespace":   "  " + canonical + "  ",
	} {
		endpoints, err := client.Discover(context.Background(), typed)
		if err != nil {
			t.Errorf("%s: discovering %q: %v", name, typed, err)

			continue
		}

		if endpoints.Issuer != canonical {
			t.Errorf(
				"%s: recorded issuer %q, want the one the provider advertises, %q",
				name, endpoints.Issuer, canonical,
			)
		}

		if want := server.URL + "/app/token/"; endpoints.TokenEndpoint != want {
			t.Errorf("%s: token endpoint %q, want %q", name, endpoints.TokenEndpoint, want)
		}
	}
}

func TestAProviderThatNamesADifferentIssuerIsRefusedWithTheAddressItNames(t *testing.T) {
	const advertised = "https://login.elsewhere.example.com/app/"

	server := discoveryServer(t, func(string) string { return advertised })

	_, err := discoverer().Discover(context.Background(), server.URL+"/app/")
	if err == nil {
		t.Fatal("accepted a document naming an unrelated issuer")
	}

	failure, ok := entity.AsSSOError(err)
	if !ok {
		t.Fatalf("returned %v, want an SSO failure", err)
	}

	if failure.Stage != entity.SSOStageDiscovery {
		t.Errorf("reported stage %q, want %q", failure.Stage, entity.SSOStageDiscovery)
	}

	if !strings.Contains(failure.Message, advertised) {
		t.Errorf("message %q does not name the issuer the provider advertises", failure.Message)
	}
}

func TestAnIssuerWithNoDocumentBehindItIsReportedAtDiscovery(t *testing.T) {
	server := discoveryServer(t, func(base string) string { return base + "/app/" })

	_, err := discoverer().Discover(context.Background(), server.URL+"/missing")
	if err == nil {
		t.Fatal("accepted an issuer serving no discovery document")
	}

	failure, ok := entity.AsSSOError(err)
	if !ok {
		t.Fatalf("returned %v, want an SSO failure", err)
	}

	if failure.Stage != entity.SSOStageDiscovery {
		t.Errorf("reported stage %q, want %q", failure.Stage, entity.SSOStageDiscovery)
	}
}
