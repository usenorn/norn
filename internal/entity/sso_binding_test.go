package entity_test

import (
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestASignInIsOnlyFinishedByTheBrowserThatStartedIt(t *testing.T) {
	correlator, err := entity.NewSSOCorrelator()
	if err != nil {
		t.Fatalf("NewSSOCorrelator: %v", err)
	}

	stored := entity.HashSSOCorrelator(correlator)

	if stored == correlator {
		t.Fatal("the correlator is stored verbatim, so anyone who reads the store can replay a sign-in")
	}

	cases := map[string]struct {
		expected  string
		presented string
		refused   bool
	}{
		"the same browser":        {expected: stored, presented: correlator},
		"a different browser":     {expected: stored, presented: "somebody-else", refused: true},
		"no cookie at all":        {expected: stored, presented: "", refused: true},
		"the hash, not the value": {expected: stored, presented: stored, refused: true},
		"an attempt with none":    {expected: "", presented: "anything"},
	}

	for name, tc := range cases {
		refusal := entity.SSOCorrelatorRefusal(tc.expected, tc.presented)

		if tc.refused && refusal == nil {
			t.Errorf("%s: the sign-in was allowed to finish", name)
		}

		if !tc.refused && refusal != nil {
			t.Errorf("%s: the sign-in was refused with %v", name, refusal)
		}
	}
}

func TestAnAssertionMustAnswerTheRequestItIsBeingUsedFor(t *testing.T) {
	cases := map[string]struct {
		expected string
		answered string
		refused  bool
	}{
		"answers our request":     {expected: "id-1", answered: "id-1"},
		"answers another request": {expected: "id-1", answered: "id-2", refused: true},
		"answers no request":      {expected: "id-1", answered: "", refused: true},
		"we started no request":   {expected: "", answered: "id-1", refused: true},
		"neither names a request": {expected: "", answered: "", refused: true},
	}

	for name, tc := range cases {
		refusal := entity.SAMLRequestMismatch(
			tc.expected,
			entity.SAMLAssertion{InResponseTo: tc.answered},
		)

		if tc.refused && refusal == nil {
			t.Errorf("%s: the assertion was accepted", name)
		}

		if !tc.refused && refusal != nil {
			t.Errorf("%s: the assertion was refused with %v", name, refusal)
		}
	}
}
