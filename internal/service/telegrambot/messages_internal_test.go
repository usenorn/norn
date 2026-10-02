package telegrambot

import (
	"strings"
	"testing"

	"github.com/usenorn/norn/internal/entity"
)

func TestAPublishedRunThatWatchesItsPullRequestsSaysSoBesideTheLinks(t *testing.T) {
	text, settled := publicationSettled(
		decisionContext{agentName: "Rae's agent", reference: "NORN-47", title: "Median helper"},
		entity.Execution{State: entity.ExecutionWatching},
		entity.ExecutionChangeSet{Changes: []entity.ExecutionChange{{
			Repository: "api", PullRequestURL: "https://github.com/acme/api/pull/7",
			Publication: entity.ExecutionPublication{State: entity.PublicationPublished},
		}}},
	)

	if !settled {
		t.Fatal("a published run watching its pull request left the publication message unsettled")
	}

	for _, want := range []string{`<a href="https://github.com/acme/api/pull/7">pull request</a>`, "Watching the pull requests"} {
		if !strings.Contains(text, want) {
			t.Fatalf("the settled message lacks %q:\n%s", want, text)
		}
	}
}
