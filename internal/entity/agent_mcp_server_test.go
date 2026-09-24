package entity_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

func fieldCodes(err error) map[string]string {
	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		return nil
	}

	codes := map[string]string{}
	for _, field := range validation.Fields {
		codes[field.Field] = field.Code
	}

	return codes
}

func TestAnMCPServerIsValidatedForItsTransport(t *testing.T) {
	cases := []struct {
		name  string
		input entity.AgentMCPServerInput
		field string
		code  string
	}{
		{"the runner's own name", entity.AgentMCPServerInput{Name: "norn", Transport: entity.AgentMCPStdio, Command: "npx"}, "name", entity.ValidationCodeTaken},
		{"a local server with nothing to run", entity.AgentMCPServerInput{Name: "files", Transport: entity.AgentMCPStdio}, "command", entity.ValidationCodeRequired},
		{"a remote server over ftp", entity.AgentMCPServerInput{Name: "linear", Transport: entity.AgentMCPHTTP, URL: "ftp://mcp.linear.app"}, "url", entity.ValidationCodeMalformed},
		{"header auth without a header", entity.AgentMCPServerInput{Name: "linear", Transport: entity.AgentMCPHTTP, URL: "https://mcp.linear.app/mcp", Auth: entity.AgentMCPAuthHeaders}, "headers", entity.ValidationCodeRequired},
		{"an env key with a dash", entity.AgentMCPServerInput{Name: "files", Transport: entity.AgentMCPStdio, Command: "npx", Env: map[string]string{"API-KEY": "x"}}, "env", entity.ValidationCodeMalformed},
		{"a header value that splits the request", entity.AgentMCPServerInput{Name: "linear", Transport: entity.AgentMCPHTTP, URL: "https://mcp.linear.app/mcp", Auth: entity.AgentMCPAuthHeaders, Headers: map[string]string{"X-Key": "a\r\nHost: evil"}}, "headers", entity.ValidationCodeMalformed},
		{"a client secret with no client", entity.AgentMCPServerInput{Name: "linear", Transport: entity.AgentMCPHTTP, URL: "https://mcp.linear.app/mcp", Auth: entity.AgentMCPAuthOAuth, OAuthClientSecret: "s"}, "oauthClientId", entity.ValidationCodeRequired},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			codes := fieldCodes(tc.input.Normalized().Validate())
			if codes[tc.field] != tc.code {
				t.Errorf("codes = %v, want %s: %s", codes, tc.field, tc.code)
			}
		})
	}
}

func TestNormalizingDropsWhatTheOtherTransportWouldHaveUsed(t *testing.T) {
	remote := entity.AgentMCPServerInput{
		Name:      " linear ",
		Transport: entity.AgentMCPHTTP,
		URL:       "https://mcp.linear.app/mcp",
		Command:   "npx",
		Args:      []string{"-y", " "},
		Env:       map[string]string{"TOKEN": "left-over"},
		Headers:   map[string]string{"X-Key": "k"},
	}.Normalized()

	if remote.Name != "linear" || remote.Command != "" || remote.Args != nil || remote.Env != nil {
		t.Errorf("remote = %+v, want the stdio fields gone", remote)
	}

	if remote.Auth != entity.AgentMCPAuthNone || remote.Headers != nil {
		t.Errorf("remote auth %q headers %v; headers are only kept when they are the sign-in", remote.Auth, remote.Headers)
	}

	local := entity.AgentMCPServerInput{
		Name:      "files",
		Transport: entity.AgentMCPStdio,
		Command:   "npx",
		Args:      []string{"-y", "", "@modelcontextprotocol/server-filesystem"},
		URL:       "https://left.over",
		Auth:      entity.AgentMCPAuthOAuth,
	}.Normalized()

	if local.URL != "" || local.Auth != entity.AgentMCPAuthNone ||
		!slices.Equal(local.Args, []string{"-y", "@modelcontextprotocol/server-filesystem"}) {
		t.Errorf("local = %+v", local)
	}

	if err := local.Validate(); err != nil {
		t.Errorf("a normalized local server did not validate: %v", err)
	}
}

func TestAConnectionPastItsExpiryReadsAsExpired(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Minute)
	soon := now.Add(3 * time.Minute)

	if (entity.AgentMCPConnection{Status: entity.AgentMCPConnected, ExpiresAt: &past}).Current(now) != entity.AgentMCPExpired {
		t.Error("an access token that expired a minute ago still reads as connected")
	}

	connection := entity.AgentMCPConnection{Status: entity.AgentMCPConnected, ExpiresAt: &soon}
	if connection.Current(now) != entity.AgentMCPConnected || !connection.DueForRefresh(now, 5*time.Minute) {
		t.Error("a token three minutes from expiry must still work and already be due for refresh")
	}

	if (entity.AgentMCPConnection{Status: entity.AgentMCPConnected}).DueForRefresh(now, time.Hour) {
		t.Error("a token with no expiry was scheduled for refresh")
	}
}

func TestAReturnPathStaysInsideNorn(t *testing.T) {
	for target, want := range map[string]bool{
		"/acme/settings/agents/1?tab=capabilities": true,
		"//evil.example/":                          false,
		"https://evil.example/":                    false,
		"/\\evil.example":                          false,
		"relative":                                 false,
	} {
		if entity.ValidAgentMCPReturnTo(target) != want {
			t.Errorf("%q: valid = %v, want %v", target, !want, want)
		}
	}
}
