package entity_test

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/usenorn/norn/internal/entity"
)

func TestAPreviewOutcomeIsOneOfReadyFailedOrUnsupported(t *testing.T) {
	valid := map[entity.SnapshotPreviewState]bool{
		entity.SnapshotPreviewReady:       true,
		entity.SnapshotPreviewFailed:      true,
		entity.SnapshotPreviewUnsupported: true,
		"":                                false,
		"open":                            false,
		"skipped":                         false,
	}

	for state, want := range valid {
		if state.Valid() != want {
			t.Errorf("%q reports Valid() as %t", state, !want)
		}
	}
}

func TestAPreviewThatNamesNoServiceIsRefused(t *testing.T) {
	err := entity.ValidateSnapshotPreview("previews[0]", entity.SnapshotPreview{
		Name:  "Application",
		State: entity.SnapshotPreviewFailed,
	})

	var invalid entity.ValidationError

	if !errors.As(err, &invalid) || invalid.Fields[0].Field != "previews[0].service" {
		t.Fatalf("a preview with no service answered %v", err)
	}
}

func TestAPreviewOnAPortNoMachineHasIsRefused(t *testing.T) {
	err := entity.ValidateSnapshotPreview("previews[0]", entity.SnapshotPreview{
		Name:    "Application",
		Service: "web",
		State:   entity.SnapshotPreviewReady,
		Port:    70000,
	})

	var invalid entity.ValidationError

	if !errors.As(err, &invalid) || invalid.Fields[0].Code != entity.ValidationCodeOutOfRange {
		t.Fatalf("port 70000 answered %v", err)
	}
}

func TestASnapshotsHeadsAreItsRepositoriesHeads(t *testing.T) {
	heads := entity.ExecutionSnapshot{Repositories: []entity.SnapshotRepository{
		{Repository: "backend", HeadSHA: "b1"},
		{Repository: "frontend", HeadSHA: "f1"},
	}}.Heads()

	if !heads.Matches(entity.ReviewHeads{"backend": "b1", "frontend": "f1"}) {
		t.Fatalf("the heads read %v", heads)
	}
}

func TestAPreviewFixRequestNamesEachFailedPreviewAndWhyAndStaysWithinAReview(t *testing.T) {
	request := entity.PreviewFixRequest([]entity.SnapshotPreview{
		{Name: "Greeting page", Reason: "directory not found"},
		{Name: "Admin", Reason: strings.Repeat("x", 5000)},
		{Name: "Docs"},
	})

	for _, want := range []string{"- Greeting page: directory not found", "- Docs: no reason was given", "Make them start."} {
		if !strings.Contains(request, want) {
			t.Errorf("the request lacks %q:\n%s", want, request)
		}
	}

	if utf8.RuneCountInString(request) > entity.ReviewSummaryMaxLen {
		t.Errorf("the request is %d runes, longer than a review summary may be", utf8.RuneCountInString(request))
	}

	if strings.Contains(request, strings.Repeat("x", entity.PreviewFixReasonMax+1)) {
		t.Error("one long reason was not clipped, so it could crowd the others out")
	}
}

func TestARevisionRepeatsTheOneBeforeOnlyWhenEveryRepositoryHoldsTheSameCommit(t *testing.T) {
	pass := func(revision int, heads ...string) entity.ExecutionSnapshot {
		snapshot := entity.ExecutionSnapshot{Revision: revision}
		for index, head := range heads {
			snapshot.Repositories = append(snapshot.Repositories, entity.SnapshotRepository{
				Repository: []string{"api", "web"}[index], HeadSHA: head,
			})
		}

		return snapshot
	}

	for name, tc := range map[string]struct {
		latest, previous entity.ExecutionSnapshot
		want             bool
	}{
		"the same commits":            {pass(2, "a1", "w1"), pass(1, "a1", "w1"), true},
		"one repository moved on":     {pass(2, "a2", "w1"), pass(1, "a1", "w1"), false},
		"a repository was added":      {pass(2, "a1", "w1"), pass(1, "a1"), false},
		"the first pass":              {pass(1, "a1"), entity.ExecutionSnapshot{}, false},
		"nothing changed either time": {pass(2), pass(1), false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := tc.latest.Repeats(tc.previous); got != tc.want {
				t.Fatalf("Repeats = %v, want %v", got, tc.want)
			}
		})
	}
}
