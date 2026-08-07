package github_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"testing"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	"github.com/usenorn/norn/internal/service/scm/github"
)

const secret = "nrnscm_a-shared-secret"

func adapter() *github.Forge {
	return github.New(nil, config.SourceControl{GitHubEndpoint: "https://api.github.com", PageSize: 100})
}

func signed(body []byte) http.Header {
	sum := hmac.New(sha256.New, []byte(secret))
	sum.Write(body)

	header := http.Header{}
	header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(sum.Sum(nil)))
	header.Set("X-GitHub-Event", "pull_request")
	header.Set("X-GitHub-Delivery", "d-1")

	return header
}

func TestADeliveryIsAcceptedOnlyWhenItsSignatureCoversTheExactBytesThatArrived(t *testing.T) {
	body := []byte(`{"action":"closed"}`)

	delivery, err := adapter().Verify(secret, signed(body), body)
	if err != nil {
		t.Fatalf("a correctly signed delivery was refused: %v", err)
	}

	if delivery.ExternalID != "d-1" || delivery.Event != "pull_request" {
		t.Fatalf("Verify = %+v, want the delivery id and event carried through", delivery)
	}

	cases := []struct {
		name   string
		body   []byte
		header func() http.Header
	}{
		{
			name:   "a body altered after signing",
			body:   []byte(`{"action":"opened"}`),
			header: func() http.Header { return signed(body) },
		},
		{
			name: "a body re-encoded on the way in",
			body: []byte(`{ "action": "closed" }`),
			header: func() http.Header {
				return signed([]byte(`{"action":"closed"}`))
			},
		},
		{
			name: "no signature at all",
			body: body,
			header: func() http.Header {
				header := signed(body)
				header.Del("X-Hub-Signature-256")

				return header
			},
		},
		{
			name: "a signature in a scheme we do not compute",
			body: body,
			header: func() http.Header {
				header := signed(body)
				header.Set("X-Hub-Signature-256", "sha1=deadbeef")

				return header
			},
		},
		{
			name: "a signed delivery that names no event",
			body: body,
			header: func() http.Header {
				header := signed(body)
				header.Del("X-GitHub-Event")

				return header
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := adapter().Verify(secret, testCase.header(), testCase.body); !errors.Is(err, entity.ErrSCMSignatureInvalid) {
				t.Fatalf("Verify accepted %s (%v)", testCase.name, err)
			}
		})
	}

	if _, err := adapter().Verify("another-secret", signed(body), body); !errors.Is(err, entity.ErrSCMSignatureInvalid) {
		t.Fatal("Verify accepted a delivery signed with a different secret")
	}
}

func TestAMergedChangeIsNotReadAsAbandonedBecauseGithubCallsItClosed(t *testing.T) {
	body := []byte(`{
      "action": "closed",
      "pull_request": {
        "id": 900123, "number": 41, "title": "Drop the cache on write",
        "html_url": "https://github.com/acme/api/pull/41",
        "state": "closed", "draft": false,
        "merged_at": "2026-08-07T10:05:00Z",
        "closed_at": "2026-08-07T10:05:00Z",
        "updated_at": "2026-08-07T10:05:00Z",
        "user": {"login": "octocat"},
        "head": {"ref": "eng-12-drop-the-cache"},
        "base": {"ref": "main"}
      },
      "sender": {"login": "octocat"}
    }`)

	events, err := adapter().Translate(entity.SCMDelivery{Event: "pull_request", Payload: body})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Translate produced %d events, want 1", len(events))
	}

	change := events[0].Change

	if change.State != entity.CodeChangeMerged {
		t.Fatalf(
			"a merged pull request read as %q. GitHub reports a merge as state \"closed\" with a "+
				"merge timestamp, so reading the state alone records every merge as abandoned and "+
				"no issue ever advances",
			change.State,
		)
	}

	if change.ExternalID != "900123" || change.Number != 41 || change.HeadBranch != "eng-12-drop-the-cache" {
		t.Fatalf("Translate = %+v, want the identity and branch carried through", change)
	}
}

func TestAChangeThatClosedWithoutMergingIsNotAMerge(t *testing.T) {
	body := []byte(`{
      "action": "closed",
      "pull_request": {
        "id": 5, "number": 5, "state": "closed", "draft": false,
        "merged_at": null, "closed_at": "2026-08-07T10:05:00Z",
        "updated_at": "2026-08-07T10:05:00Z",
        "user": {"login": "octocat"}, "head": {"ref": "eng-12"}, "base": {"ref": "main"}
      }
    }`)

	events, err := adapter().Translate(entity.SCMDelivery{Event: "pull_request", Payload: body})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}

	if events[0].Change.State != entity.CodeChangeClosed {
		t.Fatalf("an abandoned change read as %q, want closed", events[0].Change.State)
	}
}

func TestWhatTheChangeItselfSaysIsReadOffThePayload(t *testing.T) {
	draft := []byte(`{"pull_request":{"id":1,"state":"open","draft":true,"updated_at":"2026-08-07T10:00:00Z"}}`)
	open := []byte(`{"pull_request":{"id":3,"state":"open","draft":false,"updated_at":"2026-08-07T10:00:00Z"}}`)
	dirty := []byte(`{"pull_request":{"id":4,"state":"open","draft":false,
      "mergeable_state":"dirty","updated_at":"2026-08-07T10:00:00Z"}}`)
	unknown := []byte(`{"pull_request":{"id":5,"state":"open","draft":false,
      "mergeable_state":"unknown","updated_at":"2026-08-07T10:00:00Z"}}`)

	for _, testCase := range []struct {
		name string
		body []byte
		want entity.CodeChangeState
	}{
		{"a draft", draft, entity.CodeChangeDraft},
		{"simply open", open, entity.CodeChangeOpen},
		{"one that will not merge", dirty, entity.CodeChangeConflicted},
		{"one GitHub has not worked out yet", unknown, entity.CodeChangeOpen},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			events, err := adapter().Translate(entity.SCMDelivery{Event: "pull_request", Payload: testCase.body})
			if err != nil {
				t.Fatalf("Translate: %v", err)
			}

			if events[0].Change.State != testCase.want {
				t.Fatalf("state = %q, want %q", events[0].Change.State, testCase.want)
			}
		})
	}
}

// A review arrives on its own event carrying one review, not the set. Rebuilding the set
// from it would erase every other answer, so the event says only that the reviews moved and
// the whole set is read back from the forge.
func TestAReviewEventAsksForTheWholeSetRatherThanCarryingIt(t *testing.T) {
	body := []byte(`{"action":"submitted",
      "review":{"state":"changes_requested","user":{"login":"rae"}},
      "pull_request":{"id":9,"number":7,"state":"open","draft":false,
        "updated_at":"2026-08-07T11:00:00Z"},
      "sender":{"login":"rae"}}`)

	events, err := adapter().Translate(
		entity.SCMDelivery{Event: "pull_request_review", Payload: body},
	)
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}

	if len(events) != 1 || events[0].Change.Number != 7 {
		t.Fatalf("a review must carry the change it is about, got %+v", events)
	}

	if !events[0].Change.ReviewsMoved {
		t.Fatal(
			"a review event that does not report the reviews as moved leaves the stored set " +
				"stale, and the issue keeps showing an answer nobody holds any more",
		)
	}
}

// A pull request event that has nothing to do with review must not send the integration
// back to the forge for a set that cannot have changed.
func TestAnOrdinaryPullRequestEventDoesNotReReadTheReviews(t *testing.T) {
	body := []byte(`{"action":"edited",
      "pull_request":{"id":9,"number":7,"state":"open","updated_at":"2026-08-07T11:00:00Z"}}`)

	events, err := adapter().Translate(entity.SCMDelivery{Event: "pull_request", Payload: body})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}

	if events[0].Change.ReviewsMoved {
		t.Fatal("an edit asked for the reviews to be read again; that is a call per edit for nothing")
	}
}

func TestChecksAreReflectedAndNeverComputed(t *testing.T) {
	cases := map[string]entity.CodeChecks{
		`{"check_suite":{"status":"queued","pull_requests":[{"id":9,"number":7}]}}`:                             entity.CodeChecksPending,
		`{"check_suite":{"status":"in_progress","pull_requests":[{"id":9,"number":7}]}}`:                        entity.CodeChecksPending,
		`{"check_suite":{"status":"completed","conclusion":"success","pull_requests":[{"id":9,"number":7}]}}`:   entity.CodeChecksPassing,
		`{"check_suite":{"status":"completed","conclusion":"skipped","pull_requests":[{"id":9,"number":7}]}}`:   entity.CodeChecksPassing,
		`{"check_suite":{"status":"completed","conclusion":"failure","pull_requests":[{"id":9,"number":7}]}}`:   entity.CodeChecksFailing,
		`{"check_suite":{"status":"completed","conclusion":"timed_out","pull_requests":[{"id":9,"number":7}]}}`: entity.CodeChecksFailing,
		// A re-run arrives carrying the previous run's conclusion while the new one is queued.
		// Reading that as green says the change passed when nothing has run yet.
		`{"check_suite":{"status":"queued","conclusion":"success","pull_requests":[{"id":9,"number":7}]}}`:      entity.CodeChecksPending,
		`{"check_suite":{"status":"in_progress","conclusion":"failure","pull_requests":[{"id":9,"number":7}]}}`: entity.CodeChecksPending,
	}

	for payload, want := range cases {
		t.Run(string(want)+" "+payload[:40], func(t *testing.T) {
			events, err := adapter().Translate(
				entity.SCMDelivery{Event: "check_suite", Payload: []byte(payload)},
			)
			if err != nil {
				t.Fatalf("Translate: %v", err)
			}

			if len(events) != 1 {
				t.Fatalf("got %d events, want one per pull request named", len(events))
			}

			if !events[0].Change.KnowsChecks {
				t.Fatal("a checks event must say it knows them, or nothing is written")
			}

			if events[0].Change.Checks != want {
				t.Fatalf("checks = %q, want %q", events[0].Change.Checks, want)
			}
		})
	}
}

func TestASuiteAttachedToNoPullRequestChangesNothing(t *testing.T) {
	events, err := adapter().Translate(entity.SCMDelivery{
		Event:   "check_suite",
		Payload: []byte(`{"check_suite":{"status":"completed","conclusion":"failure","pull_requests":[]}}`),
	})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}

	if len(events) != 0 {
		t.Fatalf("got %d events for a suite on a branch with no pull request, want none", len(events))
	}
}

func TestAPushCarriesTheBranchAndEveryCommitOnIt(t *testing.T) {
	body := []byte(`{
      "ref": "refs/heads/eric/ENG-12-rewrite",
      "after": "abc123",
      "repository": {"html_url": "https://github.com/acme/api"},
      "sender": {"login": "octocat"},
      "commits": [
        {"id": "abc123", "message": "ENG-12 rewrite the importer",
         "url": "https://github.com/acme/api/commit/abc123",
         "timestamp": "2026-08-07T10:00:00Z", "author": {"username": "octocat"}},
        {"id": "def456", "message": "PLT-3 tidy up",
         "url": "https://github.com/acme/api/commit/def456",
         "timestamp": "2026-08-07T10:01:00Z", "author": {"username": "octocat"}}
      ]
    }`)

	events, err := adapter().Translate(entity.SCMDelivery{Event: "push", Payload: body})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("Translate produced %d events, want a branch and two commits", len(events))
	}

	if events[0].Kind != service.ForgeEventBranchPushed || events[0].Branch.Name != "eric/ENG-12-rewrite" {
		t.Fatalf("first event = %+v, want the pushed branch", events[0])
	}

	if events[1].Kind != service.ForgeEventCommitPushed || events[1].Commit.SHA != "abc123" {
		t.Fatalf("second event = %+v, want the first commit", events[1])
	}
}

func TestDeletingABranchLinksNothing(t *testing.T) {
	body := []byte(`{
      "ref": "refs/heads/eng-12", "deleted": true,
      "after": "0000000000000000000000000000000000000000", "commits": []
    }`)

	events, err := adapter().Translate(entity.SCMDelivery{Event: "push", Payload: body})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}

	if len(events) != 0 {
		t.Fatalf(
			"deleting a branch produced %d events. A deletion pushes the same ref, so reading it "+
				"as a push would link an issue to a branch that no longer exists",
			len(events),
		)
	}
}

func TestAPullRequestArrivingOnTheIssueEventsIsNotMirroredAsAnIssue(t *testing.T) {
	body := []byte(`{
      "action": "opened",
      "issue": {"id": 7, "number": 7, "title": "Drop the cache",
                "updated_at": "2026-08-07T10:00:00Z", "pull_request": {}}
    }`)

	events, err := adapter().Translate(entity.SCMDelivery{Event: "issues", Payload: body})
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}

	if len(events) != 0 {
		t.Fatal(
			"a pull request is also an issue on GitHub's api, so mirroring one would create a " +
				"second Norn issue for a change that is already carried as a link",
		)
	}
}

func TestAnEventThisIntegrationDoesNotActOnIsIgnoredRatherThanFailing(t *testing.T) {
	events, err := adapter().Translate(entity.SCMDelivery{Event: "star", Payload: []byte(`{}`)})
	if err != nil {
		t.Fatalf("an unhandled event failed the delivery: %v", err)
	}

	if len(events) != 0 {
		t.Fatalf("Translate produced %d events for an unhandled kind", len(events))
	}
}
