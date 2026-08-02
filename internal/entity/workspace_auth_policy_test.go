package entity_test

import (
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestSSOIsEnforcedEverywhereOnlyWhenEveryWorkspaceSaysSo(t *testing.T) {
	cases := []struct {
		name         string
		enforcements []entity.AuthEnforcement
		enforced     bool
	}{
		{"no workspaces", nil, false},
		{"every workspace enforces sso", []entity.AuthEnforcement{entity.AuthEnforcementSSO, entity.AuthEnforcementSSO}, true},
		{"one workspace still accepts passwords", []entity.AuthEnforcement{entity.AuthEnforcementSSO, entity.AuthEnforcementAny}, false},
		{"no workspace enforces sso", []entity.AuthEnforcement{entity.AuthEnforcementAny}, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := entity.SSOEnforcedEverywhere(c.enforcements); got != c.enforced {
				t.Fatalf("SSOEnforcedEverywhere(%v) = %v, want %v", c.enforcements, got, c.enforced)
			}
		})
	}
}
