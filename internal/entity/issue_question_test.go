package entity_test

import (
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestOptionsOnAQuestionAlwaysIncludeTheOneTheAgentWillActOn(t *testing.T) {
	cases := map[string]struct {
		options       []string
		defaultAnswer string
		valid         bool
	}{
		"none at all is a question that takes any answer": {
			options: nil, defaultAnswer: "admins only", valid: true,
		},
		"two, one of them the default": {
			options: []string{"admins only", "members too"}, defaultAnswer: "admins only", valid: true,
		},
		"four, one of them the default": {
			options:       []string{"admins only", "members too", "nobody", "ask again"},
			defaultAnswer: "nobody",
			valid:         true,
		},
		"a default nobody can pick": {
			options: []string{"members too", "nobody"}, defaultAnswer: "admins only", valid: false,
		},
		"one option is not a choice": {
			options: []string{"admins only"}, defaultAnswer: "admins only", valid: false,
		},
		"five is a list, not a decision": {
			options:       []string{"a", "b", "c", "d", "e"},
			defaultAnswer: "a",
			valid:         false,
		},
		"the same answer twice": {
			options: []string{"admins only", "admins only"}, defaultAnswer: "admins only", valid: false,
		},
		"an empty option": {
			options: []string{"admins only", "  "}, defaultAnswer: "admins only", valid: false,
		},
	}

	for name, want := range cases {
		failure := entity.ValidateQuestionOptions("options", want.options, want.defaultAnswer)

		if want.valid && failure.Code != "" {
			t.Errorf("%s: refused with %q, and it should have been accepted", name, failure.Code)
		}

		if !want.valid && failure.Code == "" {
			t.Errorf(
				"%s: accepted. A person is offered these as the whole of the decision, so an "+
					"option they cannot pick or a default missing from the list leaves them stuck",
				name,
			)
		}
	}
}

func TestAnOptionIsShortEnoughToBeAButton(t *testing.T) {
	long := make([]rune, entity.QuestionOptionMaxLen+1)

	for i := range long {
		long[i] = 'a'
	}

	failure := entity.ValidateQuestionOptions(
		"options", []string{string(long), "no"}, string(long),
	)

	if failure.Code != entity.ValidationCodeTooLong {
		t.Errorf(
			"an option of %d characters was accepted as %q; options are picked at a glance, and "+
				"prose in them is a question asked twice",
			len(long), failure.Code,
		)
	}
}
