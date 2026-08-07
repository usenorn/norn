package entity_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestReadCapabilityGrantsOnlyReadScopes(t *testing.T) {
	scopes := entity.MCPScopesFor(entity.MCPCapabilityRead)

	if !scopes.Permits(entity.ResourceIssue, entity.ActionRead) {
		t.Error("a read connection cannot read issues")
	}

	if !scopes.Permits(entity.ResourceCycle, entity.ActionRead) {
		t.Error("a read connection cannot read cycles")
	}

	if scopes.Permits(entity.ResourceIssue, entity.ActionManage) {
		t.Fatal("a read connection may manage issues")
	}

	if got := entity.CapabilityOf(scopes); got != entity.MCPCapabilityRead {
		t.Errorf("capability of read scopes = %q, want read", got)
	}
}

func TestWriteCapabilityAddsIssueAndCommentManagementOnly(t *testing.T) {
	scopes := entity.MCPScopesFor(entity.MCPCapabilityWrite)

	if !scopes.Permits(entity.ResourceIssue, entity.ActionManage) {
		t.Error("a write connection cannot manage issues")
	}

	if !scopes.Permits(entity.ResourceComment, entity.ActionManage) {
		t.Error("a write connection cannot manage comments")
	}

	if scopes.Permits(entity.ResourceTeam, entity.ActionManage) {
		t.Fatal("a write connection may manage teams")
	}

	if scopes.Permits(entity.ResourceMembership, entity.ActionManage) {
		t.Fatal("a write connection may manage memberships")
	}

	if got := entity.CapabilityOf(scopes); got != entity.MCPCapabilityWrite {
		t.Errorf("capability of write scopes = %q, want write", got)
	}
}

func TestEveryMCPScopeExistsInTheCatalog(t *testing.T) {
	for _, scope := range entity.MCPScopesFor(entity.MCPCapabilityWrite) {
		if !scope.Valid() {
			t.Errorf(
				"mcp capability grants %q which the scope catalog does not know; Holds would "+
					"pass it but nothing could ever mint it",
				scope,
			)
		}
	}
}

func TestOAuthScopeStringsResolveToTheStrongestCapability(t *testing.T) {
	for _, probe := range []struct {
		value      string
		capability entity.MCPCapability
		rejected   bool
	}{
		{"", entity.MCPCapabilityRead, false},
		{"read", entity.MCPCapabilityRead, false},
		{"write", entity.MCPCapabilityWrite, false},
		{"read write", entity.MCPCapabilityWrite, false},
		{"write read", entity.MCPCapabilityWrite, false},
		{"admin", "", true},
		{"read admin", "", true},
	} {
		capability, err := entity.ParseMCPScopes(probe.value)

		if probe.rejected {
			if err == nil {
				t.Errorf("scope %q was accepted", probe.value)
			}

			continue
		}

		if err != nil {
			t.Errorf("scope %q was rejected: %v", probe.value, err)

			continue
		}

		if capability != probe.capability {
			t.Errorf("scope %q resolved to %q, want %q", probe.value, capability, probe.capability)
		}
	}
}

func TestALoopbackRedirectMayComeBackOnAnyPort(t *testing.T) {
	client := entity.MCPClient{
		RedirectURIs: []string{"http://127.0.0.1:8976/callback"},
	}

	for _, uri := range []string{
		"http://127.0.0.1:8976/callback",
		"http://127.0.0.1:9000/callback",
		"http://127.0.0.1:54304/callback",
	} {
		if !client.PermitsRedirect(uri) {
			t.Errorf(
				"a loopback redirect to %q was refused. Clients take an ephemeral port from the "+
					"operating system when they start listening, so the port they registered with "+
					"is rarely the one they come back on, and RFC 8252 requires any port here.",
				uri,
			)
		}
	}

	for _, uri := range []string{
		"http://127.0.0.1:8976/callback/",
		"http://127.0.0.1:8976/other",
		"http://localhost:8976/callback",
		"https://127.0.0.1:8976/callback",
		"http://example.com:8976/callback",
	} {
		if client.PermitsRedirect(uri) {
			t.Errorf("a redirect to %q was permitted despite not being registered", uri)
		}
	}
}

func TestOnlyLoopbackRedirectsAreForgivenTheirPort(t *testing.T) {
	client := entity.MCPClient{
		RedirectURIs: []string{"https://client.example.com:8443/callback"},
	}

	if !client.PermitsRedirect("https://client.example.com:8443/callback") {
		t.Error("the registered redirect was refused")
	}

	if client.PermitsRedirect("https://client.example.com:9443/callback") {
		t.Error(
			"a public https redirect was accepted on a different port. Only loopback addresses " +
				"take whatever port the operating system hands out; anywhere else the port is part " +
				"of the address that was registered.",
		)
	}
}

func TestRedirectURIsAcceptHTTPSLoopbackAndPrivateSchemesOnly(t *testing.T) {
	for _, probe := range []struct {
		uri   string
		valid bool
	}{
		{"https://example.com/callback", true},
		{"http://localhost:8080/cb", true},
		{"http://127.0.0.1:33418/cb", true},
		{"http://[::1]:8080/cb", true},
		{"http://example.com/cb", false},
		{"claude://oauth/callback", true},
		{"com.example.app:/oauth/callback", true},
		{"https://example.com/cb#fragment", false},
		{"", false},
		{"/relative/path", false},
	} {
		if got := entity.ValidMCPRedirectURI(probe.uri); got != probe.valid {
			t.Errorf("redirect %q valid = %v, want %v", probe.uri, got, probe.valid)
		}
	}
}

func TestARedirectTheBrowserWouldExecuteIsNeverRegistrable(t *testing.T) {
	for _, uri := range []string{
		"javascript:eval(atob('ZmV0Y2g='))//",
		"JavaScript:alert(1)",
		"data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==",
		"vbscript:msgbox(1)",
		"blob:https://example.com/1234",
		"file:///etc/passwd",
		"about:blank",
	} {
		if entity.ValidMCPRedirectURI(uri) {
			t.Errorf(
				"%q was accepted as a redirect. Registration is unauthenticated and the consent "+
					"screen hands the result to the browser, so a scheme the browser executes turns "+
					"one click — approve or deny — into script running on our own origin with the "+
					"signed-in session attached.",
				uri,
			)
		}
	}
}

func TestPKCEAcceptsTheVerifierThatProducedTheChallenge(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	if !entity.VerifyPKCE(challenge, verifier) {
		t.Error("the verifier that produced the challenge was refused")
	}

	if entity.VerifyPKCE(challenge, verifier+"x") {
		t.Fatal("a different verifier satisfied the challenge")
	}

	if entity.VerifyPKCE("", "") {
		t.Fatal("an empty challenge and verifier passed PKCE; a missing challenge must never verify")
	}
}

func TestARevokedConnectionIsNeverUsable(t *testing.T) {
	now := time.Now()
	connection := entity.MCPConnection{ID: uuid.New()}

	if !connection.Usable() {
		t.Error("a live connection was refused")
	}

	connection.RevokedAt = &now

	if connection.Usable() {
		t.Fatal("a revoked connection was still usable")
	}
}

func TestAConnectionWithoutGrantsFollowsMembership(t *testing.T) {
	connection := entity.MCPConnection{}

	if !connection.FollowsMembership() {
		t.Error("a grantless connection does not follow membership")
	}

	connection.Grants = entity.APITokenGrants{{WorkspaceID: uuid.New(), AllTeams: true}}

	if connection.FollowsMembership() {
		t.Error("a narrowed connection still claims to follow membership")
	}
}

func TestUsageIsStampedAtMostOncePerInterval(t *testing.T) {
	now := time.Now()
	interval := time.Minute
	connection := entity.MCPConnection{}

	if !connection.NeedsUsageStamp(now, interval) {
		t.Error("a never-used connection was not stamped")
	}

	recent := now.Add(-30 * time.Second)
	connection.LastUsedAt = &recent

	if connection.NeedsUsageStamp(now, interval) {
		t.Error("a connection stamped within the interval was stamped again")
	}

	stale := now.Add(-2 * time.Minute)
	connection.LastUsedAt = &stale

	if !connection.NeedsUsageStamp(now, interval) {
		t.Error("a connection last stamped beyond the interval was not stamped")
	}
}

func TestMCPTokensExpireAtTheirDeadlineExactly(t *testing.T) {
	now := time.Now()
	token := entity.MCPToken{ExpiresAt: now.Add(time.Hour)}

	if token.ExpiredAt(now) {
		t.Error("a live token was treated as expired")
	}

	if !token.ExpiredAt(now.Add(time.Hour)) {
		t.Error("a token at its deadline was still live")
	}
}

func TestMintedMCPTokenValuesCarryThePrefixAndHashConsistently(t *testing.T) {
	value, hash, err := entity.NewMCPTokenValue()
	if err != nil {
		t.Fatalf("mint mcp token: %v", err)
	}

	if !entity.LooksLikeMCPToken(value) {
		t.Errorf("minted value %q does not carry the mcp prefix", value)
	}

	if entity.LooksLikeAPIToken(value) {
		t.Fatal(
			"an mcp token value looks like an api token; the dashboard bearer middleware would " +
				"try to authenticate it",
		)
	}

	if !bytes.Equal(hash, entity.HashMCPToken(value)) {
		t.Error("the returned hash does not match hashing the returned value")
	}
}
