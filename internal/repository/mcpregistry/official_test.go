package mcpregistry_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/toolingclient"
	"github.com/usenorn/norn/internal/repository/mcpregistry"
)

const listing = `{"servers":[
 {"server":{"name":"io.github.getsentry/sentry-mcp","description":"Sentry issues","version":"1.4.0",
   "repository":{"url":"https://github.com/getsentry/sentry-mcp"},
   "packages":[{"registryType":"npm","identifier":"@sentry/mcp-server","version":"1.4.0",
     "transport":{"type":"stdio"},
     "packageArguments":[{"type":"named","name":"--host","valueHint":"host"}],
     "environmentVariables":[{"name":"SENTRY_TOKEN","description":"Access token","isRequired":true,"isSecret":true}]},
    {"registryType":"oci","identifier":"ghcr.io/getsentry/sentry-mcp","version":"1.4.0","transport":{"type":"stdio"},
     "environmentVariables":[{"name":"SENTRY_TOKEN","isSecret":true}]},
    {"registryType":"nuget","identifier":"Sentry.Mcp","transport":{"type":"stdio"}}],
   "remotes":[{"type":"streamable-http","url":"https://mcp.sentry.dev/mcp"}]},
  "_meta":{"io.modelcontextprotocol.registry/official":{"status":"active"}}},
 {"server":{"name":"com.example/retired","description":"Gone","version":"0.1.0",
   "remotes":[{"type":"sse","url":"https://retired.example/sse"}]},
  "_meta":{"io.modelcontextprotocol.registry/official":{"status":"deleted"}}},
 {"server":{"name":"com.example/source-only","description":"Nothing to run","version":"0.1.0"}}
]}`

func TestARegistryEntryBecomesTheServersItCanBeRunAs(t *testing.T) {
	var asked string

	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r.URL.RawQuery
		_, _ = w.Write([]byte(listing))
	}))
	t.Cleanup(registry.Close)

	client, err := toolingclient.New(config.AgentTooling{
		GitHubEndpoint:      registry.URL,
		GitHubRawEndpoint:   registry.URL,
		RegistryEndpoint:    registry.URL,
		RequestTimeout:      5 * time.Second,
		DialTimeout:         time.Second,
		MaxResponseSize:     8 << 20,
		AllowedDestinations: []string{"127.0.0.1/32"},
	})
	if err != nil {
		t.Fatalf("toolingclient.New: %v", err)
	}

	entries, err := mcpregistry.New(client).Search(context.Background(), "sentry")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if asked != "limit=30&search=sentry&version=latest" {
		t.Errorf("query = %q", asked)
	}

	if len(entries) != 1 || entries[0].Name != "io.github.getsentry/sentry-mcp" {
		t.Fatalf("entries = %+v, want only the active entry that has something to run", entries)
	}

	templates := entries[0].Templates
	if len(templates) != 3 {
		t.Fatalf("templates = %+v, want the remote, npm and oci ways to run it", templates)
	}

	if templates[0].Transport != entity.AgentMCPHTTP || templates[0].URL != "https://mcp.sentry.dev/mcp" {
		t.Errorf("remote = %+v", templates[0])
	}

	npm := templates[1]
	if npm.Command != "npx" || !slices.Equal(npm.Args, []string{"-y", "@sentry/mcp-server@1.4.0", "--host", "<host>"}) {
		t.Errorf("npm = %s %v", npm.Command, npm.Args)
	}

	if len(npm.Env) != 1 || !npm.Env[0].Secret || !npm.Env[0].Required {
		t.Errorf("npm env = %+v", npm.Env)
	}

	oci := templates[2]
	if oci.Command != "docker" ||
		!slices.Equal(oci.Args, []string{"run", "-i", "--rm", "-e", "SENTRY_TOKEN", "ghcr.io/getsentry/sentry-mcp:1.4.0"}) {
		t.Errorf("oci = %s %v; docker only passes a variable it is told to", oci.Command, oci.Args)
	}
}
