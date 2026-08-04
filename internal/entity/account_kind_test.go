package entity_test

import (
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestAnAgentIsNeverAbleToHoldAPassword(t *testing.T) {
	agent := entity.Account{Kind: entity.AccountKindAgent, DisplayName: "Triage Bot"}

	if !agent.Agent() {
		t.Fatal("Account.Agent() does not recognise an agent account")
	}

	if agent.HasPassword() {
		t.Fatal("an agent account reports a password; the schema forbids one and the code must agree")
	}
}

func TestOnlyTheTwoKnownAccountKindsAreValid(t *testing.T) {
	valid := []entity.AccountKind{entity.AccountKindPerson, entity.AccountKindAgent}

	for _, kind := range valid {
		if !kind.Valid() {
			t.Errorf("%q is not valid but should be", kind)
		}
	}

	for _, kind := range []entity.AccountKind{"", "robot", "service", "Person", "AGENT"} {
		if kind.Valid() {
			t.Errorf("%q is treated as a valid account kind; the CHECK constraint would reject it", kind)
		}
	}
}

func TestAPersonIsNotAnAgent(t *testing.T) {
	person := entity.Account{Kind: entity.AccountKindPerson, Email: "rae@northwind.co"}

	if person.Agent() {
		t.Fatal("a person account reports as an agent")
	}
}
