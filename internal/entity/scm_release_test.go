package entity_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func TestOnlyAChangeWhoseMergeCommitShippedIsInTheRelease(t *testing.T) {
	shipped := entity.CodeLink{ID: uuid.New(), MergeCommitSHA: "AbC123"}
	held := entity.CodeLink{ID: uuid.New(), MergeCommitSHA: "def456"}
	unmerged := entity.CodeLink{ID: uuid.New()}

	matched := entity.MatchReleaseCommits(
		[]string{"abc123", "  ", "999999"},
		[]entity.CodeLink{shipped, held, unmerged},
	)

	if len(matched) != 1 || matched[0].ID != shipped.ID {
		t.Fatalf(
			"matched %d change(s); a release contains the changes whose merge commit is in it "+
				"and nothing else, and forges do not agree on the case of a sha",
			len(matched),
		)
	}
}

func TestAChangeWithNoMergeCommitNeverShips(t *testing.T) {
	matched := entity.MatchReleaseCommits(
		[]string{""},
		[]entity.CodeLink{{ID: uuid.New()}, {ID: uuid.New(), MergeCommitSHA: ""}},
	)

	if len(matched) != 0 {
		t.Fatalf(
			"matched %d change(s) against an empty commit; an unmerged change would appear in "+
				"every release",
			len(matched),
		)
	}
}

func TestThePreviousReleaseIsTheOneBeforeItAndNotAPrerelease(t *testing.T) {
	at := func(day int) *time.Time {
		moment := time.Date(2026, 8, day, 12, 0, 0, 0, time.UTC)

		return &moment
	}

	older := entity.SCMRelease{ID: uuid.New(), Tag: "v1.2.0", PublishedAt: at(1)}
	candidate := entity.SCMRelease{ID: uuid.New(), Tag: "v1.3.0-rc1", Prerelease: true, PublishedAt: at(4)}
	current := entity.SCMRelease{ID: uuid.New(), Tag: "v1.3.0", PublishedAt: at(5)}

	releases := entity.SCMReleases{current, candidate, older}

	previous, found := releases.Previous(current)
	if !found || previous.ID != older.ID {
		t.Fatalf(
			"Previous = %q, want v1.2.0. A release candidate is not what the last release was, "+
				"and comparing against one would report the wrong set of issues as shipped",
			previous.Tag,
		)
	}

	if _, found := releases.Previous(older); found {
		t.Fatal("the first release has nothing before it")
	}
}

func TestAnEnvironmentReportsItsMostRecentDeployment(t *testing.T) {
	at := func(hour int) *time.Time {
		moment := time.Date(2026, 8, 7, hour, 0, 0, 0, time.UTC)

		return &moment
	}

	deployments := entity.SCMDeployments{
		{Environment: "production", State: entity.DeploymentSucceeded, OccurredAt: at(9)},
		{Environment: "production", State: entity.DeploymentFailed, OccurredAt: at(14)},
		{Environment: "staging", State: entity.DeploymentSucceeded, OccurredAt: at(11)},
	}

	live, found := deployments.Latest("Production")
	if !found || live.State != entity.DeploymentFailed {
		t.Fatalf(
			"Latest = %q; the newest deployment is what an environment is running, and forges "+
				"do not agree on the case of an environment name",
			live.State,
		)
	}

	if _, found := deployments.Latest("nowhere"); found {
		t.Error("an environment nothing was deployed to reported a deployment")
	}

	if got := deployments.Environments(); len(got) != 2 {
		t.Errorf("Environments = %v, want production and staging once each", got)
	}
}
