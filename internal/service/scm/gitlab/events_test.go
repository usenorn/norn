package gitlab_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	"github.com/usenorn/norn/internal/service/scm/gitlab"
)

const secret = "nrnscm_a-shared-secret"

func adapter() *gitlab.Forge {
	return gitlab.New(nil, config.SourceControl{GitLabEndpoint: "https://gitlab.com", PageSize: 100})
}

func header(token, event string) http.Header {
	built := http.Header{}

	if token != "" {
		built.Set("X-Gitlab-Token", token)
	}

	if event != "" {
		built.Set("X-Gitlab-Event", event)
	}

	built.Set("X-Gitlab-Event-UUID", "u-1")

	return built
}

func TestADeliveryIsAcceptedOnlyWhenItCarriesTheTokenThisConnectionIssued(t *testing.T) {
	delivery, err := adapter().Verify(secret, header(secret, "Push Hook"), []byte(`{}`))
	if err != nil {
		t.Fatalf("a correctly tokenised delivery was refused: %v", err)
	}

	if delivery.ExternalID != "u-1" || delivery.Event != "Push Hook" {
		t.Fatalf("Verify = %+v, want the delivery id and event carried through", delivery)
	}

	for _, testCase := range []struct {
		name   string
		header http.Header
	}{
		{"a token from another connection", header("nrnscm_someone-elses", "Push Hook")},
		{"no token at all", header("", "Push Hook")},
		{"a prefix of the real token", header(secret[:10], "Push Hook")},
		{"a tokenised delivery naming no event", header(secret, "")},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := adapter().Verify(secret, testCase.header, []byte(`{}`)); !errors.Is(err, entity.ErrSCMSignatureInvalid) {
				t.Fatalf("Verify accepted %s (%v)", testCase.name, err)
			}
		})
	}
}

func TestAMergedRequestSaysSoInGitlabsOwnVocabulary(t *testing.T) {
	body := []byte(`{
      "user": {"username": "octocat"},
      "object_attributes": {
        "id": 900123, "iid": 41, "title": "Drop the cache",
        "state": "merged", "url": "https://gitlab.com/acme/api/-/merge_requests/41",
        "source_branch": "eng-12-drop-the-cache", "target_branch": "main",
        "updated_at": "2026-08-07 10:05:00 UTC",
        "author": {"username": "octocat"}
      }
    }`)

	events, err := adapter().Translate(entity.SCMDelivery{Event: "Merge Request Hook", Payload: body})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}

	change := events[0].Change

	if change.State != entity.CodeChangeMerged {
		t.Fatalf("a merged request read as %q, want merged", change.State)
	}

	if change.MergedAt == nil {
		t.Fatal("a merged request must carry when it merged, or nothing can order it against a reopen")
	}

	if change.HeadBranch != "eng-12-drop-the-cache" || change.Number != 41 {
		t.Fatalf("Translate = %+v, want the branch and number carried through", change)
	}
}

func TestGitlabsOwnTimestampFormatIsReadRatherThanDiscarded(t *testing.T) {
	body := []byte(`{
      "object_attributes": {"id": 1, "iid": 1, "state": "opened",
        "updated_at": "2026-08-07 10:05:00 UTC"}
    }`)

	events, err := adapter().Translate(entity.SCMDelivery{Event: "Merge Request Hook", Payload: body})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}

	if events[0].Change.UpdatedAt.IsZero() {
		t.Fatal(
			"gitlab's webhooks send a space-separated stamp with a named zone rather than " +
				"rfc 3339. Read as zero it is older than every watermark, so the change is " +
				"skipped by the sweep and the link never updates",
		)
	}

	if got := events[0].Change.UpdatedAt.Format("2006-01-02T15:04:05Z"); got != "2026-08-07T10:05:00Z" {
		t.Fatalf("UpdatedAt = %s, want the stamp the forge sent", got)
	}
}

func TestGitlabNarratingItselfIsNotAComment(t *testing.T) {
	body := []byte(`{
      "user": {"username": "octocat"},
      "object_attributes": {"id": 5, "note": "changed the description",
        "updated_at": "2026-08-07 10:05:00 UTC"},
      "issue": {"id": 77, "iid": 7, "title": "Drop the cache",
        "updated_at": "2026-08-07 10:05:00 UTC"}
    }`)

	events, err := adapter().Translate(entity.SCMDelivery{Event: "Note Hook", Payload: body})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}

	if len(events) != 1 || events[0].Kind != service.ForgeEventCommented {
		t.Fatalf("a note on an issue produced %+v, want one comment", events)
	}

	if events[0].Issue.ExternalID != "77" || events[0].Issue.Number != 7 {
		t.Fatalf(
			"the comment names issue %q number %d; the identity it is stored under and the "+
				"number it is addressed by are different values and both are needed",
			events[0].Issue.ExternalID, events[0].Issue.Number,
		)
	}
}

func TestANoteLeftOnSomethingThatIsNotAnIssueNamesNothingToMirror(t *testing.T) {
	body := []byte(`{
      "user": {"username": "octocat"},
      "object_attributes": {"id": 5, "note": "looks good",
        "updated_at": "2026-08-07 10:05:00 UTC"},
      "merge_request": {"id": 3}
    }`)

	events, err := adapter().Translate(entity.SCMDelivery{Event: "Note Hook", Payload: body})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}

	if len(events) != 0 {
		t.Fatalf("a note on a merge request produced %d events; it names no issue to carry it to", len(events))
	}
}

func TestAPushCarriesTheBranchAndItsCommits(t *testing.T) {
	body := []byte(`{
      "ref": "refs/heads/eric/ENG-12-rewrite", "after": "abc123",
      "project": {"web_url": "https://gitlab.com/acme/api"},
      "user_username": "octocat",
      "commits": [{"id": "abc123", "message": "ENG-12 rewrite",
        "url": "https://gitlab.com/acme/api/-/commit/abc123",
        "timestamp": "2026-08-07T10:00:00Z", "author": {"name": "Octo Cat"}}]
    }`)

	events, err := adapter().Translate(entity.SCMDelivery{Event: "Push Hook", Payload: body})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("Translate produced %d events, want a branch and one commit", len(events))
	}

	if events[0].Branch.Name != "eric/ENG-12-rewrite" {
		t.Fatalf("branch = %q, want the pushed ref without its prefix", events[0].Branch.Name)
	}
}

func TestDeletingABranchLinksNothing(t *testing.T) {
	body := []byte(`{"ref": "refs/heads/eng-12",
      "after": "0000000000000000000000000000000000000000", "commits": []}`)

	events, err := adapter().Translate(entity.SCMDelivery{Event: "Push Hook", Payload: body})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}

	if len(events) != 0 {
		t.Fatalf("deleting a branch produced %d events", len(events))
	}
}
