package ssoconnection_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var ssoPackages = []string{
	"../../entity/sso.go",
	"../../entity/oidc.go",
	"../../entity/saml.go",
	"../../pkg/crypter",
	"../../pkg/oidcprovider",
	"../../pkg/samlkey",
	"../../pkg/samlprovider",
	"../../repository/oidcprovider",
	"../../repository/oidcstate",
	"../../repository/samlreplay",
	"../../repository/samlrequest",
	"../../repository/ssoconnection",
	"../../handler/http/sso",
	"../../handler/http/v1/dashboard/sso.go",
	".",
}

var commercialWords = []string{
	"licence", "license", "tier", "entitlement", "quota", "subscription",
	"billing", "seat", "upgrade", "paywall", "premium", "enterprise",
}

func TestSingleSignOnIsNotGatedBehindAnythingCommercial(t *testing.T) {
	for _, target := range ssoPackages {
		for _, path := range goFilesUnder(t, target) {
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}

			lowered := strings.ToLower(string(body))

			for _, word := range commercialWords {
				if strings.Contains(lowered, word) {
					t.Errorf(
						"%s mentions %q. Single sign-on is free on every tier and works on a "+
							"bare self-hosted install; there is nothing here to gate it on, and "+
							"no licensing code exists to gate it with.",
						path, word,
					)
				}
			}
		}
	}
}

func goFilesUnder(t *testing.T, target string) []string {
	t.Helper()

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf(
			"%s is not there. This guard names the files it protects, so a rename must update "+
				"it rather than silently cover nothing.",
			target,
		)
	}

	if !info.IsDir() {
		return []string{target}
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		t.Fatalf("read %s: %v", target, err)
	}

	files := make([]string, 0, len(entries))

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}

		if strings.HasPrefix(entry.Name(), "mock_") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		files = append(files, filepath.Join(target, entry.Name()))
	}

	if len(files) == 0 {
		t.Fatalf("%s has no Go files, so this guard covers nothing there", target)
	}

	return files
}
