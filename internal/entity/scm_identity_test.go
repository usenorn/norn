package entity_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestALoginIsMatchedWhateverItsCase(t *testing.T) {
	account := uuid.New()

	identities := entity.SCMIdentities{
		{AccountID: account, Provider: entity.SCMProviderGitHub, Login: "RaeChen"},
	}

	for _, login := range []string{"RaeChen", "raechen", "RAECHEN"} {
		if got, found := identities.AccountFor(entity.SCMProviderGitHub, login); !found || got != account {
			t.Errorf("AccountFor(%q) = %v, %t; forges do not agree on case", login, got, found)
		}
	}
}

func TestALoginOnAnotherForgeIsNotTheSamePerson(t *testing.T) {
	identities := entity.SCMIdentities{
		{AccountID: uuid.New(), Provider: entity.SCMProviderGitHub, Login: "rae"},
	}

	if _, found := identities.AccountFor(entity.SCMProviderGitLab, "rae"); found {
		t.Fatal(
			"a GitHub login matched a GitLab one. The same handle on two forges is two " +
				"different people as often as not, and acting on the guess assigns somebody " +
				"else's work",
		)
	}
}

func TestAnUnmappedLoginIsNobodyRatherThanAGuess(t *testing.T) {
	identities := entity.SCMIdentities{
		{AccountID: uuid.New(), Provider: entity.SCMProviderGitHub, Login: "rae"},
	}

	if _, found := identities.AccountFor(entity.SCMProviderGitHub, "sam"); found {
		t.Fatal("an unmapped login resolved to somebody")
	}
}

func TestALoginIsCleanedOfWhatPeopleTypeAroundIt(t *testing.T) {
	if got := entity.NormalizeSCMLogin("  @rae  "); got != "rae" {
		t.Fatalf("NormalizeSCMLogin = %q, want rae", got)
	}

	for _, refused := range []string{"", "   ", "rae chen", "acme/rae", "@"} {
		if field := entity.ValidateSCMLogin("login", refused); field.Field == "" {
			t.Errorf("ValidateSCMLogin(%q) was accepted; that is not a handle", refused)
		}
	}

	if field := entity.ValidateSCMLogin("login", "rae-chen_1"); field.Field != "" {
		t.Error("an ordinary handle was refused")
	}
}
