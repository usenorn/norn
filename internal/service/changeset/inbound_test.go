package changeset_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func TestWhatARunChangedIsRecordedRepositoryByRepository(t *testing.T) {
	h := newHarness(t)
	h.holding()

	message := h.message(entity.ChannelChangeSetUpdated, channelv1.ChangeSet{
		Repos: []channelv1.RepoChange{
			repoChange("backend", "norn/NORN-38/backend"),
			repoChange("frontend", "norn/NORN-38/frontend"),
		},
	})

	if err := h.service.Updated(context.Background(), h.runner, message); err != nil {
		t.Fatalf("record what a run changed: %v", err)
	}

	backend, ok := h.change("backend")
	if !ok {
		t.Fatal("the backend repository was not recorded, so the review screen has nothing to show")
	}

	if backend.Branch != "norn/NORN-38/backend" || backend.Commits != 3 ||
		backend.Additions != 412 || backend.Deletions != 77 || backend.FilesChanged != 9 {
		t.Fatalf("the diffstat came back as %+v, which is not what the machine reported", backend)
	}

	if _, ok := h.change("frontend"); !ok {
		t.Fatal("a run touching two repositories recorded only one of them")
	}
}

func TestAnUpdateNamingOneRepositoryLeavesTheOthersAlone(t *testing.T) {
	h := newHarness(t)
	h.holding()

	first := h.message(entity.ChannelChangeSetUpdated, channelv1.ChangeSet{
		Repos: []channelv1.RepoChange{
			repoChange("backend", "norn/NORN-38/backend"),
			repoChange("frontend", "norn/NORN-38/frontend"),
		},
	})
	if err := h.service.Updated(context.Background(), h.runner, first); err != nil {
		t.Fatalf("record the first update: %v", err)
	}

	later := repoChange("backend", "norn/NORN-38/backend")
	later.Commits = 5

	second := h.messageAt(
		entity.ChannelChangeSetUpdated,
		channelv1.ChangeSet{Repos: []channelv1.RepoChange{later}},
		time.Date(2026, 8, 23, 11, 0, 0, 0, time.UTC),
	)
	if err := h.service.Updated(context.Background(), h.runner, second); err != nil {
		t.Fatalf("record the second update: %v", err)
	}

	if _, ok := h.change("frontend"); !ok {
		t.Fatal(
			"an update naming only the backend wiped the frontend row; work that already landed " +
				"would disappear from the result as the run went on",
		)
	}

	backend, _ := h.change("backend")
	if backend.Commits != 5 {
		t.Fatalf("the backend still reads %d commits, so the later report was lost", backend.Commits)
	}
}

func TestValidationResultsSurviveAnUpdateThatCarriesOnlyRepositories(t *testing.T) {
	h := newHarness(t)
	h.holding()

	withChecks := h.message(entity.ChannelChangeSetUpdated, channelv1.ChangeSet{
		Validation: []channelv1.Validation{
			{Check: "backend tests", Status: channelv1.ValidationPassed},
		},
	})
	if err := h.service.Updated(context.Background(), h.runner, withChecks); err != nil {
		t.Fatalf("record a validation result: %v", err)
	}

	reposOnly := h.messageAt(
		entity.ChannelChangeSetUpdated,
		channelv1.ChangeSet{Repos: []channelv1.RepoChange{repoChange("backend", "b")}},
		time.Date(2026, 8, 23, 11, 0, 0, 0, time.UTC),
	)
	if err := h.service.Updated(context.Background(), h.runner, reposOnly); err != nil {
		t.Fatalf("record a repositories-only update: %v", err)
	}

	if _, ok := h.validation("backend tests"); !ok {
		t.Fatal(
			"a repositories-only update dropped the validation results; a person reviewing would " +
				"be told the tests were never run",
		)
	}
}

func TestAReportThatArrivesOutOfOrderNeverUndoesTheNewerOne(t *testing.T) {
	h := newHarness(t)
	h.holding()

	newer := repoChange("backend", "norn/NORN-38/backend")
	newer.Commits = 5

	if err := h.service.Updated(context.Background(), h.runner, h.messageAt(
		entity.ChannelChangeSetUpdated,
		channelv1.ChangeSet{Repos: []channelv1.RepoChange{newer}},
		time.Date(2026, 8, 23, 11, 0, 0, 0, time.UTC),
	)); err != nil {
		t.Fatalf("record the newer report: %v", err)
	}

	older := repoChange("backend", "norn/NORN-38/backend")
	older.Commits = 2

	if err := h.service.Updated(context.Background(), h.runner, h.messageAt(
		entity.ChannelChangeSetUpdated,
		channelv1.ChangeSet{Repos: []channelv1.RepoChange{older}},
		time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC),
	)); err != nil {
		t.Fatalf("a replayed older report refused the connection: %v", err)
	}

	backend, _ := h.change("backend")
	if backend.Commits != 5 {
		t.Fatalf(
			"a message replayed after a reconnect rolled the backend back to %d commits; "+
				"delivery is at-least-once and out of order, so the newer report has to win",
			backend.Commits,
		)
	}
}

func TestTheResultSettlesTheSummaryAndTheChangesTogether(t *testing.T) {
	h := newHarness(t)
	h.holding()

	message := h.message(entity.ChannelExecutionResult, channelv1.Result{
		Summary: "added the changeset ingest",
		ChangeSet: channelv1.ChangeSet{
			Repos:      []channelv1.RepoChange{repoChange("backend", "norn/NORN-38/backend")},
			Validation: []channelv1.Validation{{Check: "go test", Status: channelv1.ValidationPassed}},
		},
	})

	if err := h.service.Resulted(context.Background(), h.runner, message); err != nil {
		t.Fatalf("record a run's result: %v", err)
	}

	if h.result.Summary != "added the changeset ingest" {
		t.Fatalf("the run's summary came back as %q", h.result.Summary)
	}

	if _, ok := h.change("backend"); !ok {
		t.Fatal("the result carried a repository that was not recorded")
	}

	if _, ok := h.validation("go test"); !ok {
		t.Fatal("the result carried a validation result that was not recorded")
	}
}

func TestAPullRequestBecomesACodeLinkSoItsStateStaysCurrent(t *testing.T) {
	h := newHarness(t)
	h.holding()

	linkID := uuid.New()
	h.links(linkID)

	change := repoChange("backend", "norn/NORN-38/backend")
	change.PullRequest = "https://github.com/usenorn/norn/pull/231"

	message := h.message(
		entity.ChannelChangeSetUpdated,
		channelv1.ChangeSet{Repos: []channelv1.RepoChange{change}},
	)

	if err := h.service.Updated(context.Background(), h.runner, message); err != nil {
		t.Fatalf("record a change carrying a pull request: %v", err)
	}

	stored, _ := h.change("backend")
	if stored.CodeLinkID != linkID {
		t.Fatal(
			"the pull request did not become a code link, so nothing would ever refresh its " +
				"state, checks or reviewers as people reviewed it",
		)
	}

	if len(h.linked) != 1 || h.linked[0].URL != change.PullRequest {
		t.Fatalf("source control was asked to link %+v", h.linked)
	}

	if h.linked[0].DetectedIn == "" {
		t.Fatal(
			"a pull request a run opened would be recorded on the issue feed as though a person " +
				"had linked it by hand",
		)
	}
}

func TestAPullRequestInARepositoryNornDoesNotFollowIsStillRecorded(t *testing.T) {
	h := newHarness(t)
	h.holding()
	h.refusesLinks(entity.ValidationError{
		Fields: []entity.FieldError{{Field: "url", Code: entity.ValidationCodeUnsupportedValue}},
	})

	change := repoChange("backend", "norn/NORN-38/backend")
	change.PullRequest = "https://example.invalid/acme/tools/pull/7"

	message := h.message(
		entity.ChannelChangeSetUpdated,
		channelv1.ChangeSet{Repos: []channelv1.RepoChange{change}},
	)

	if err := h.service.Updated(context.Background(), h.runner, message); err != nil {
		t.Fatalf(
			"a pull request in a repository norn does not follow failed the whole message: %v; "+
				"everything else the run reported would be lost with it",
			err,
		)
	}

	stored, ok := h.change("backend")
	if !ok {
		t.Fatal("the repository was not recorded at all")
	}

	if stored.PullRequestURL != change.PullRequest {
		t.Fatalf(
			"the pull request address came back as %q; without a code link the address is the "+
				"only thing pointing a person at the work",
			stored.PullRequestURL,
		)
	}

	if stored.CodeLinkID != uuid.Nil {
		t.Fatal("a repository norn does not follow was given a code link anyway")
	}
}

func TestAReportAgainstARunThisMachineIsNotHoldingIsRefused(t *testing.T) {
	h := newHarness(t)

	h.executions.EXPECT().
		Held(gomock.Any(), h.runner, h.execution.ID).
		Return(entity.Execution{}, entity.ErrExecutionNotFound)

	message := h.message(entity.ChannelChangeSetUpdated, channelv1.ChangeSet{
		Repos: []channelv1.RepoChange{repoChange("backend", "b")},
	})

	err := h.service.Updated(context.Background(), h.runner, message)
	if !errors.Is(err, entity.ErrExecutionNotFound) {
		t.Fatalf(
			"a machine wrote a result onto a run it does not hold and got %v; one runner could "+
				"rewrite another's",
			err,
		)
	}

	if len(h.changes) != 0 {
		t.Fatal("the refused message still wrote a row")
	}
}

func TestAPayloadTheServerCannotReadEndsTheConnectionRatherThanBeingStored(t *testing.T) {
	h := newHarness(t)
	h.holding()

	message := h.message(entity.ChannelChangeSetUpdated, channelv1.ChangeSet{})
	message.Payload = []byte(`{"repos": "not a list"}`)

	err := h.service.Updated(context.Background(), h.runner, message)
	if !errors.Is(err, entity.ErrChannelEnvelopeInvalid) {
		t.Fatalf(
			"a payload the server could not read answered %v; the transport turns an invalid "+
				"envelope into close 1003, and anything else leaves the runner retrying it forever",
			err,
		)
	}
}

func TestAValidationStatusNobodyDefinedIsRefused(t *testing.T) {
	h := newHarness(t)
	h.holding()

	message := h.message(entity.ChannelChangeSetUpdated, channelv1.ChangeSet{
		Validation: []channelv1.Validation{{Check: "backend tests", Status: "probably fine"}},
	})

	var invalid entity.ValidationError

	if err := h.service.Updated(context.Background(), h.runner, message); !errors.As(err, &invalid) {
		t.Fatalf("a status outside passed, failed and skipped was stored, answering %v", err)
	}
}

func TestRecordingWhatChangedTellsWhoeverIsWatchingTheIssue(t *testing.T) {
	h := newHarness(t)
	h.holding()

	message := h.message(entity.ChannelChangeSetUpdated, channelv1.ChangeSet{
		Repos: []channelv1.RepoChange{repoChange("backend", "norn/NORN-38/backend")},
	})

	if err := h.service.Updated(context.Background(), h.runner, message); err != nil {
		t.Fatalf("record what a run changed: %v", err)
	}

	for _, event := range h.published {
		if event.Kind == entity.EventExecutionChangeSet && event.IssueID == h.issue.ID {
			return
		}
	}

	t.Fatalf(
		"nothing was published for the issue, so a review screen open on it would sit on stale "+
			"diffstat until somebody reloaded; %d events went out",
		len(h.published),
	)
}
