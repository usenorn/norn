package github_test

import (
	"context"
	"encoding/pem"
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

type signedIn struct {
	login         string
	installations string
	memberships   map[string]string
}

func (s signedIn) handler(t *testing.T) http.Handler {
	t.Helper()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer gho-the-user-token" {
			t.Errorf("%s was asked for as %q, want the signed-in person", r.URL.Path, got)
		}

		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/user":
			_, _ = w.Write([]byte(`{"login":"` + s.login + `"}`))

		case r.URL.Path == "/user/installations":
			_, _ = w.Write([]byte(s.installations))

		case strings.HasPrefix(r.URL.Path, "/user/memberships/orgs/"):
			membership, found := s.memberships[strings.TrimPrefix(r.URL.Path, "/user/memberships/orgs/")]
			if !found {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"message":"Not Found"}`))

				return
			}

			_, _ = w.Write([]byte(membership))

		default:
			t.Errorf("the adapter asked for %q, which this test serves nothing for", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func TestOnlyInstallationsOfThisApplicationAreOffered(t *testing.T) {
	held, address := reachable(t, signedIn{
		login: "rae",
		installations: `{"installations":[
            {"id":884411,"app_id":4711,"account":{"login":"flagroll","type":"Organization"}},
            {"id":990022,"app_id":9999,"account":{"login":"rae","type":"User"}}
        ]}`,
		memberships: map[string]string{"flagroll": `{"state":"active","role":"admin"}`},
	}.handler(t))

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

func TestOnlyInstallationsTheSignedInPersonAdministersAreOffered(t *testing.T) {
	cases := []struct {
		name    string
		account string
		orgs    map[string]string
	}{
		{
			name:    "a member of the organisation, not an owner",
			account: `{"login":"flagroll","type":"Organization"}`,
			orgs:    map[string]string{"flagroll": `{"state":"active","role":"member"}`},
		},
		{
			name:    "an owner whose invitation is still pending",
			account: `{"login":"flagroll","type":"Organization"}`,
			orgs:    map[string]string{"flagroll": `{"state":"pending","role":"admin"}`},
		},
		{
			name:    "an organisation they are not in at all",
			account: `{"login":"flagroll","type":"Organization"}`,
			orgs:    map[string]string{},
		},
		{
			name:    "somebody else's personal account they collaborate on",
			account: `{"login":"morgan","type":"User"}`,
			orgs:    map[string]string{},
		},
	}

	for _, held := range cases {
		t.Run(held.name, func(t *testing.T) {
			forgeFor, address := reachable(t, signedIn{
				login:         "rae",
				installations: `{"installations":[{"id":884411,"app_id":4711,"account":` + held.account + `}]}`,
				memberships:   held.orgs,
			}.handler(t))

			found, err := forgeFor.Installations(
				context.Background(),
				entity.SCMApp{BaseURL: address, ExternalAppID: "4711"},
				"gho-the-user-token",
			)
			if err != nil {
				t.Fatalf("Installations: %v", err)
			}

			if len(found) != 0 {
				t.Fatalf(
					"%d installations were offered to somebody who administers none of them. "+
						"One application serves this whole instance, so connecting an "+
						"installation mints tokens for every repository it was granted — "+
						"reading one of them is not a claim on the rest",
					len(found),
				)
			}
		})
	}
}

func TestAnOrganisationBlockingTheApplicationDoesNotEndTheListing(t *testing.T) {
	held, address := reachable(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/user":
			_, _ = w.Write([]byte(`{"login":"rae"}`))

		case "/user/installations":
			_, _ = w.Write([]byte(`{"installations":[
                {"id":884411,"app_id":4711,"account":{"login":"blocked","type":"Organization"}},
                {"id":884412,"app_id":4711,"account":{"login":"rae","type":"User"}}
            ]}`))

		case "/user/memberships/orgs/blocked":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"Resource not accessible by integration"}`))

		default:
			t.Errorf("the adapter asked for %q, which this test serves nothing for", r.URL.Path)
		}
	}))

	found, err := held.Installations(
		context.Background(),
		entity.SCMApp{BaseURL: address, ExternalAppID: "4711"},
		"gho-the-user-token",
	)
	if err != nil {
		t.Fatalf(
			"Installations: %v. One organisation refusing to answer for this application is not "+
				"a reason to hide the installations the same person does administer",
			err,
		)
	}

	if len(found) != 1 || found[0].AccountLogin != "rae" {
		t.Fatalf("the offer came back as %+v", found)
	}
}

func TestEveryPageOfInstallationsIsRead(t *testing.T) {
	var address string

	held, base := reachable(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/user":
			_, _ = w.Write([]byte(`{"login":"rae"}`))

		case r.URL.Path == "/user/installations" && r.URL.Query().Get("page") == "":
			w.Header().Set("Link", `<`+address+`/user/installations?page=2>; rel="next"`)
			_, _ = w.Write([]byte(`{"installations":[
                {"id":884411,"app_id":4711,"account":{"login":"rae","type":"User"}}
            ]}`))

		case r.URL.Path == "/user/installations":
			_, _ = w.Write([]byte(`{"installations":[
                {"id":884412,"app_id":4711,"account":{"login":"flagroll","type":"Organization"}}
            ]}`))

		case r.URL.Path == "/user/memberships/orgs/flagroll":
			_, _ = w.Write([]byte(`{"state":"active","role":"admin"}`))

		default:
			t.Errorf("the adapter asked for %q, which this test serves nothing for", r.URL.Path)
		}
	}))

	address = base

	found, err := held.Installations(
		context.Background(),
		entity.SCMApp{BaseURL: base, ExternalAppID: "4711"},
		"gho-the-user-token",
	)
	if err != nil {
		t.Fatalf("Installations: %v", err)
	}

	if len(found) != 2 {
		t.Fatalf(
			"%d installations were offered out of two pages. Somebody whose installations run "+
				"past the first page cannot connect the ones that fell off it",
			len(found),
		)
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

			_, err := forgeFor.ConvertManifest(context.Background(), entity.SCMApp{BaseURL: address}, "the-code")

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

	converted, err := held.ConvertManifest(context.Background(), entity.SCMApp{BaseURL: address}, "the-code")
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

	if _, err := held.ConvertManifest(context.Background(), entity.SCMApp{BaseURL: address}, "the-code"); err != nil {
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

func onPrivateAuthority(t *testing.T, handler http.Handler) (*github.Forge, string, string) {
	t.Helper()

	server := httptest.NewTLSServer(handler)
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

	authority := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: server.Certificate().Raw,
	})

	return github.New(client, cfg), server.URL, string(authority)
}

func TestAnApplicationOnAPrivateAuthorityIsReachedOnlyWithTheOneItWasGiven(t *testing.T) {
	held, address, authority := onPrivateAuthority(
		t,
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"repositories":[]}`))
		}),
	)

	unknown := entity.SCMApp{BaseURL: address, ExternalAppID: "4711"}

	if _, err := held.InstallationRepositories(
		context.Background(), unknown, "ghs-installation",
	); err == nil {
		t.Fatal(
			"a certificate no public authority signed was accepted. The application's stored " +
				"authority would then be decoration, and any certificate would do",
		)
	}

	trusted := unknown
	trusted.Trust = entity.SCMTrust{CACertificate: authority}

	if _, err := held.InstallationRepositories(
		context.Background(), trusted, "ghs-installation",
	); err != nil {
		t.Fatalf(
			"the authority stored with the application did not open the call: %v. An enterprise "+
				"instance behind its own authority is reachable no other way, so an installation "+
				"there could never be read",
			err,
		)
	}
}
