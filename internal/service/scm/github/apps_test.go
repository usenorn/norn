package github_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/forge"
	"github.com/usenorn/norn/internal/service/scm/github"
)

func reachable(t *testing.T, handler http.Handler) (*github.Forge, string) {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := config.SourceControl{
		GitHubEndpoint:      server.URL,
		PageSize:            20,
		RequestTimeout:      5 * time.Second,
		DialTimeout:         2 * time.Second,
		MaxResponseSize:     1 << 20,
		AllowedDestinations: []string{"127.0.0.1/32", "::1/128"},
	}

	client, err := forge.New(cfg)
	if err != nil {
		t.Fatalf("build a forge client: %v", err)
	}

	return github.New(client, cfg), server.URL
}

func TestOnlyInstallationsOfThisApplicationAreOffered(t *testing.T) {
	held, address := reachable(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/installations" {
			t.Errorf("the adapter asked for %q, want /user/installations", r.URL.Path)
		}

		if got := r.Header.Get("Authorization"); got != "Bearer gho-the-user-token" {
			t.Errorf("the listing was made as %q, want the signed-in person", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"installations":[
            {"id":884411,"app_id":4711,"account":{"login":"flagroll","type":"Organization"}},
            {"id":990022,"app_id":9999,"account":{"login":"someone-else","type":"User"}}
        ]}`))
	}))

	found, err := held.Installations(
		context.Background(),
		entity.SCMApp{BaseURL: address, ExternalAppID: "4711"},
		"gho-the-user-token",
	)
	if err != nil {
		t.Fatalf("Installations: %v", err)
	}

	if len(found) != 1 {
		t.Fatalf(
			"%d installations were offered. A person signs in to this instance but may have "+
				"other applications installed; offering one of those would connect a workspace to "+
				"an application this instance holds no key for",
			len(found),
		)
	}

	if found[0].ExternalID != "884411" || found[0].AccountLogin != "flagroll" {
		t.Fatalf("the installation came back as %+v", found[0])
	}

	if found[0].AccountKind != "organization" {
		t.Errorf("the account kind came back %q, want it folded to lower case", found[0].AccountKind)
	}
}

func TestAManifestThatConvertsToNothingUsableIsRefused(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "no identifier", body: `{"slug":"norn","pem":"key","webhook_secret":"hook"}`},
		{name: "no key", body: `{"id":4711,"slug":"norn","webhook_secret":"hook"}`},
		{name: "no webhook secret", body: `{"id":4711,"slug":"norn","pem":"key"}`},
	}

	for _, held := range cases {
		t.Run(held.name, func(t *testing.T) {
			forgeFor, address := reachable(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(held.body))
			}))

			_, err := forgeFor.ConvertManifest(context.Background(), address, "the-code")

			if err == nil {
				t.Fatal(
					"a half-converted application was stored. Without the key it can mint no " +
						"token and without the secret it can verify no delivery, so the instance " +
						"would look connected and do nothing",
				)
			}
		})
	}
}

func TestAConvertedApplicationCarriesEverythingItNeeds(t *testing.T) {
	held, address := reachable(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("the conversion was a %s, want a POST", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
            "id":4711,"slug":"norn-northwind","client_id":"Iv1.deadbeef",
            "client_secret":"shhh","pem":"-----BEGIN RSA PRIVATE KEY-----",
            "webhook_secret":"hook"
        }`))
	}))

	converted, err := held.ConvertManifest(context.Background(), address, "the-code")
	if err != nil {
		t.Fatalf("ConvertManifest: %v", err)
	}

	if converted.ExternalAppID != "4711" || converted.Slug != "norn-northwind" {
		t.Errorf("the application came back as %+v", converted)
	}

	if converted.PrivateKey == "" || converted.WebhookSecret == "" || converted.ClientSecret == "" {
		t.Error("the conversion dropped a secret, so nothing that needs it would work")
	}
}

func TestTheSignInAddressPointsAtTheForgeRatherThanItsApi(t *testing.T) {
	held, _ := reachable(t, http.NotFoundHandler())

	address := held.AuthorizeURL(
		entity.SCMApp{BaseURL: "https://api.github.com", ClientID: "Iv1.deadbeef"},
		"the-state",
		"https://norn.example/v1/source-control/github-app/connected",
	)

	const want = "https://github.com/login/oauth/authorize?"

	if len(address) < len(want) || address[:len(want)] != want {
		t.Fatalf(
			"a person would be sent to %q. The api host serves no sign-in page, so the browser "+
				"lands on json instead of a consent screen",
			address,
		)
	}
}

func TestConvertingAManifestIsNotSentAsAnEmptyCredential(t *testing.T) {
	var sent http.Header

	held, address := reachable(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent = r.Header.Clone()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":4711,"slug":"norn","pem":"key","webhook_secret":"hook"}`))
	}))

	if _, err := held.ConvertManifest(context.Background(), address, "the-code"); err != nil {
		t.Fatalf("ConvertManifest: %v", err)
	}

	if _, carried := sent["Authorization"]; carried {
		t.Fatalf(
			"the conversion carried an Authorization header of %q. It takes no credential, and a "+
				"forge handed an empty bearer answers 401 rather than treating the call as "+
				"unauthenticated — which loses the only copy of the application's key",
			sent.Get("Authorization"),
		)
	}
}

func TestACallThatHasACredentialStillCarriesIt(t *testing.T) {
	var sent http.Header

	held, address := reachable(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent = r.Header.Clone()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"installations":[]}`))
	}))

	if _, err := held.Installations(
		context.Background(),
		entity.SCMApp{BaseURL: address, ExternalAppID: "4711"},
		"gho-the-user-token",
	); err != nil {
		t.Fatalf("Installations: %v", err)
	}

	if sent.Get("Authorization") != "Bearer gho-the-user-token" {
		t.Fatalf(
			"a call that has a credential sent %q. Dropping it would make every authenticated "+
				"read fail",
			sent.Get("Authorization"),
		)
	}
}

func TestASignInThatTheForgeRefusedSaysWhatItSaid(t *testing.T) {
	held, address := reachable(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
            "error":"incorrect_client_credentials",
            "error_description":"The client_id and/or client_secret passed are incorrect."
        }`))
	}))

	_, err := held.ExchangeCode(
		context.Background(),
		entity.SCMApp{BaseURL: address, ClientID: "Iv1.deadbeef"},
		"the-code",
		"https://norn.example/v1/source-control/github-app/connected",
	)

	if !errors.Is(err, entity.ErrSCMAppRefused) {
		t.Fatalf("ExchangeCode returned %v, want the refusal sentinel", err)
	}

	if !strings.Contains(err.Error(), "client_secret") {
		t.Fatalf(
			"the refusal reads %q. The forge said exactly what was wrong and it was dropped, so "+
				"an operator reading the log learns nothing beyond that it failed",
			err,
		)
	}
}
