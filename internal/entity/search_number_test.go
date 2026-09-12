package entity_test

import (
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestANumberOnItsOwnNamesAnIssue(t *testing.T) {
	for _, raw := range []string{"55", "#55", " 55 "} {
		query := entity.ParseSearchQuery(raw)

		if query.Number != 55 {
			t.Errorf(
				"%q parsed with the number lane at %d. People quote an issue by its number "+
					"alone, and the text lanes only match a title that happens to contain "+
					"that word.",
				raw, query.Number,
			)
		}

		if query.Reference != nil {
			t.Errorf("%q claimed the team key %q it was never given", raw, query.Reference.Key)
		}
	}
}

func TestAKeyAndANumberAreOneTargetHoweverTheyAreSeparated(t *testing.T) {
	for _, raw := range []string{"ENG-412", "eng-412", "ENG 412", "eng412", "#ENG-412"} {
		query := entity.ParseSearchQuery(raw)

		if query.Reference == nil {
			t.Errorf("%q did not parse as a reference", raw)

			continue
		}

		if query.Reference.Key != "ENG" || query.Reference.Number != 412 {
			t.Errorf("%q parsed as %+v", raw, *query.Reference)
		}

		if query.Number != 0 {
			t.Errorf("%q filled the number lane as well, which would pin every team's 412", raw)
		}
	}
}

func TestTextThatOnlyLooksLikeANumberIsLeftToTheTextLanes(t *testing.T) {
	for _, raw := range []string{"0", "55 payments", "v55", "55.2", "1234567890"} {
		if number := entity.ParseSearchQuery(raw).Number; number != 0 {
			t.Errorf(
				"%q filled the number lane with %d. Only a bare number is a reference; "+
					"anything else is words somebody is searching for.",
				raw, number,
			)
		}
	}
}

func TestANumberStillSearchesTheTextAsWell(t *testing.T) {
	query := entity.ParseSearchQuery("55")

	if query.Stemmed == "" || query.Prefix == "" {
		t.Fatal(
			"the number lane emptied the text lanes. An issue titled \"error 55\" is still a " +
				"result worth returning under the pinned one.",
		)
	}
}
