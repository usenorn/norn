package scm

import (
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestAPastedAddressIsReadTheSameWayOnEitherForge(t *testing.T) {
	cases := []struct {
		name       string
		url        string
		repository string
		kind       entity.CodeLinkKind
		number     int
		externalID string
	}{
		{
			name:       "a github pull request",
			url:        "https://github.com/acme/api/pull/41",
			repository: "acme/api",
			kind:       entity.CodeLinkChange,
			number:     41,
			externalID: "41",
		},
		{
			name:       "a gitlab merge request, past the marker segment",
			url:        "https://gitlab.com/acme/api/-/merge_requests/41",
			repository: "acme/api",
			kind:       entity.CodeLinkChange,
			number:     41,
			externalID: "41",
		},
		{
			name:       "a gitlab project nested in groups",
			url:        "https://gitlab.example.com/acme/platform/api/-/merge_requests/7",
			repository: "acme/platform/api",
			kind:       entity.CodeLinkChange,
			number:     7,
			externalID: "7",
		},
		{
			name:       "a commit, which has no number",
			url:        "https://github.com/acme/api/commit/abc123",
			repository: "acme/api",
			kind:       entity.CodeLinkCommit,
			number:     0,
			externalID: "abc123",
		},
		{
			name:       "a branch",
			url:        "https://github.com/acme/api/tree/eng-12-drop-the-cache",
			repository: "acme/api",
			kind:       entity.CodeLinkBranch,
			number:     0,
			externalID: "eng-12-drop-the-cache",
		},
		{
			name:       "an address carrying a fragment and a query",
			url:        "https://github.com/acme/api/pull/41?w=1#discussion",
			repository: "acme/api",
			kind:       entity.CodeLinkChange,
			number:     41,
			externalID: "41",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			address, err := parseCodeURL(testCase.url)
			if err != nil {
				t.Fatalf("parseCodeURL(%q): %v", testCase.url, err)
			}

			if address.repository != testCase.repository {
				t.Errorf("repository = %q, want %q", address.repository, testCase.repository)
			}

			if address.kind != testCase.kind {
				t.Errorf("kind = %q, want %q", address.kind, testCase.kind)
			}

			if address.number != testCase.number {
				t.Errorf("number = %d, want %d", address.number, testCase.number)
			}

			if address.externalID != testCase.externalID {
				t.Errorf("externalID = %q, want %q", address.externalID, testCase.externalID)
			}
		})
	}
}

func TestAnAddressThatNamesNoChangeIsRefusedRatherThanGuessedAt(t *testing.T) {
	for _, raw := range []string{
		"",
		"not a url",
		"https://github.com/acme/api",
		"https://github.com/acme/api/pull",
		"https://github.com/pull/41",
		"ftp://github.com/acme/api/pull/41",
		"javascript:alert(1)//github.com/acme/api/pull/41",
	} {
		if _, err := parseCodeURL(raw); err == nil {
			t.Errorf("parseCodeURL(%q) was accepted; a guessed link points at the wrong change", raw)
		}
	}
}

func TestAnAddressBelongsToAConnectionByHostAsWellAsRepository(t *testing.T) {
	hosted := mustParse(t, "https://gitlab.example.com/acme/api/-/merge_requests/1")
	cloud := mustParse(t, "https://gitlab.com/acme/api/-/merge_requests/1")

	if !sameHost("https://gitlab.example.com", hosted.host) {
		t.Error("an address on a self-hosted forge must belong to the connection naming that host")
	}

	if sameHost("https://gitlab.example.com", cloud.host) {
		t.Error(
			"a repository path that happens to match on another host was accepted. Two forges " +
				"can hold a project of the same name and linking across them points at somebody " +
				"else's work",
		)
	}

	if !sameHost("", cloud.host) || !sameHost("", hosted.host) {
		t.Error("a connection with no base url follows the forge's own host and must still match")
	}
}

func mustParse(t *testing.T, raw string) codeAddress {
	t.Helper()

	address, err := parseCodeURL(raw)
	if err != nil {
		t.Fatalf("parseCodeURL(%q): %v", raw, err)
	}

	return address
}
