package entity_test

import (
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func signInPolicy(slug string, enforcement entity.AuthEnforcement, ready bool) entity.WorkspaceSignInPolicy {
	return entity.WorkspaceSignInPolicy{
		Workspace:   entity.SignInWorkspace{Slug: slug, Name: slug, Protocol: entity.SSOProtocolOIDC, Host: slug + ".okta.com"},
		Enforcement: enforcement,
		SSOReady:    ready,
	}
}

func methodsOf(options entity.SignInOptions) []string {
	methods := make([]string, 0, len(options.Options))

	for _, option := range options.Options {
		method := string(option.Method)
		if option.Workspace != nil {
			method += ":" + option.Workspace.Slug
		}

		methods = append(methods, method)
	}

	return methods
}

func TestSignInMethodsFollowInstanceAndWorkspacePolicy(t *testing.T) {
	cases := []struct {
		name          string
		passwordAuth  bool
		policies      []entity.WorkspaceSignInPolicy
		want          []string
		wantPreferred entity.SessionAuthMethod
	}{
		{
			name:          "an address no workspace claims signs in with a password",
			passwordAuth:  true,
			want:          []string{"password"},
			wantPreferred: entity.SessionAuthMethodPassword,
		},
		{
			name:          "a workspace that allows both prefers its provider and keeps the password",
			passwordAuth:  true,
			policies:      []entity.WorkspaceSignInPolicy{signInPolicy("northwind", entity.AuthEnforcementAny, true)},
			want:          []string{"sso:northwind", "password"},
			wantPreferred: entity.SessionAuthMethodSSO,
		},
		{
			name:          "a workspace that requires its provider offers nothing else",
			passwordAuth:  true,
			policies:      []entity.WorkspaceSignInPolicy{signInPolicy("northwind", entity.AuthEnforcementSSO, true)},
			want:          []string{"sso:northwind"},
			wantPreferred: entity.SessionAuthMethodSSO,
		},
		{
			name:          "a provider that has not passed its test is never offered",
			passwordAuth:  true,
			policies:      []entity.WorkspaceSignInPolicy{signInPolicy("northwind", entity.AuthEnforcementAny, false)},
			want:          []string{"password"},
			wantPreferred: entity.SessionAuthMethodPassword,
		},
		{
			name:          "a required provider that is not ready leaves the password as the way in",
			passwordAuth:  true,
			policies:      []entity.WorkspaceSignInPolicy{signInPolicy("northwind", entity.AuthEnforcementSSO, false)},
			want:          []string{"password"},
			wantPreferred: entity.SessionAuthMethodPassword,
		},
		{
			name:         "several workspaces on one domain each offer their provider",
			passwordAuth: true,
			policies: []entity.WorkspaceSignInPolicy{
				signInPolicy("northwind", entity.AuthEnforcementSSO, true),
				signInPolicy("northwind-labs", entity.AuthEnforcementSSO, true),
			},
			want:          []string{"sso:northwind", "sso:northwind-labs"},
			wantPreferred: entity.SessionAuthMethodSSO,
		},
		{
			name:         "one workspace that still accepts passwords keeps the password on offer",
			passwordAuth: true,
			policies: []entity.WorkspaceSignInPolicy{
				signInPolicy("northwind", entity.AuthEnforcementSSO, true),
				signInPolicy("northwind-labs", entity.AuthEnforcementAny, false),
			},
			want:          []string{"sso:northwind", "password"},
			wantPreferred: entity.SessionAuthMethodSSO,
		},
		{
			name:          "an instance without passwords offers only providers",
			passwordAuth:  false,
			policies:      []entity.WorkspaceSignInPolicy{signInPolicy("northwind", entity.AuthEnforcementAny, true)},
			want:          []string{"sso:northwind"},
			wantPreferred: entity.SessionAuthMethodSSO,
		},
		{
			name:         "an instance without passwords and no provider offers nothing",
			passwordAuth: false,
			want:         []string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := entity.DeriveSignInOptions(tc.passwordAuth, tc.policies)

			methods := methodsOf(got)
			if len(methods) != len(tc.want) {
				t.Fatalf("methods = %v, want %v", methods, tc.want)
			}

			for i := range methods {
				if methods[i] != tc.want[i] {
					t.Fatalf("methods = %v, want %v", methods, tc.want)
				}
			}

			if got.Preferred != tc.wantPreferred {
				t.Errorf("preferred = %q, want %q", got.Preferred, tc.wantPreferred)
			}
		})
	}
}
