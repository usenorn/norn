package changeset_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func (h *harness) approved(revision int, heads map[string]string) {
	snapshot := entity.ExecutionSnapshot{ExecutionID: h.execution.ID, Revision: revision}

	for repository, head := range heads {
		snapshot.Repositories = append(snapshot.Repositories,
			entity.SnapshotRepository{Repository: repository, HeadSHA: head})
		h.changes = append(h.changes, entity.ExecutionChange{
			ID: uuid.New(), ExecutionID: h.execution.ID, Repository: repository, HeadSHA: head,
		})
	}

	h.recorded = append(h.recorded, snapshot)
}

func TestEachRepositoryKeepsItsOwnPublicationOutcome(t *testing.T) {
	h := newHarness(t)
	h.holding()
	h.links(uuid.New())
	h.approved(2, map[string]string{"backend": "b2", "frontend": "f2"})

	message := h.message(entity.ChannelPublication, channelv1.Publication{
		Revision: 2,
		Repos: []channelv1.RepoPublication{
			{
				Repository: "backend", Branch: "norn/NORN-231", SHA: "b2",
				State:       channelv1.PublicationPublished,
				Step:        channelv1.PublicationStepPullRequest,
				PullRequest: "https://github.com/usenorn/norn/pull/300",
			},
			{
				Repository: "frontend", Branch: "norn/NORN-231", SHA: "f2",
				State:   channelv1.PublicationFailed,
				Step:    channelv1.PublicationStepPush,
				Failure: "remote rejected: protected branch",
			},
		},
	})

	if err := h.service.Published(context.Background(), h.runner, message); err != nil {
		t.Fatalf("record the publication: %v", err)
	}

	backend, _ := h.change("backend")
	if backend.Publication.State != entity.PublicationPublished ||
		backend.PullRequestURL != "https://github.com/usenorn/norn/pull/300" ||
		backend.Publication.PublishedAt == nil {
		t.Fatalf("the published repository came back as %+v", backend)
	}

	if len(h.relayed) != 1 || h.relayed[0].Kind != entity.TelegramDecisionPublication {
		t.Fatalf(
			"an incomplete publication relayed %+v; whoever decides on Telegram never hears it stalled",
			h.relayed,
		)
	}

	if backend.CodeLinkID == uuid.Nil {
		t.Fatal("the pull request publication opened was never linked to the issue")
	}

	frontend, _ := h.change("frontend")
	if !frontend.Publication.Failed() ||
		frontend.Publication.Error != "remote rejected: protected branch" ||
		frontend.Publication.Step != entity.PublicationStepPush {
		t.Fatalf(
			"the failed repository came back as %+v; a partial publication would read as done",
			frontend.Publication,
		)
	}
}

func TestAPublicationOfCommitsNobodyApprovedIsNeverRecorded(t *testing.T) {
	for name, publication := range map[string]channelv1.Publication{
		"an older revision": {
			Revision: 1,
			Repos: []channelv1.RepoPublication{{
				Repository: "backend", SHA: "b2", State: channelv1.PublicationPushed,
			}},
		},
		"a head nobody reviewed": {
			Revision: 2,
			Repos: []channelv1.RepoPublication{{
				Repository: "backend", SHA: "b3", State: channelv1.PublicationPushed,
			}},
		},
		"a repository outside the review": {
			Revision: 2,
			Repos: []channelv1.RepoPublication{{
				Repository: "infra", SHA: "i1", State: channelv1.PublicationPushed,
			}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.holding()
			h.approved(2, map[string]string{"backend": "b2"})

			message := h.message(entity.ChannelPublication, publication)
			if err := h.service.Published(context.Background(), h.runner, message); err != nil {
				t.Fatalf("a mismatched publication should be dropped, not close the channel: %v", err)
			}

			if backend, _ := h.change("backend"); backend.Publication.State != entity.PublicationNone {
				t.Fatalf("%s was recorded as %+v", name, backend.Publication)
			}
		})
	}
}

func TestARetryUnderwayClosesTheStalledPublicationMessage(t *testing.T) {
	h := newHarness(t)
	h.holding()
	h.approved(2, map[string]string{"backend": "b2"})

	message := h.message(entity.ChannelPublication, channelv1.Publication{
		Revision: 2,
		Attempt:  2,
		Repos: []channelv1.RepoPublication{{
			Repository: "backend", SHA: "b2", State: channelv1.PublicationPending,
		}},
	})

	if err := h.service.Published(context.Background(), h.runner, message); err != nil {
		t.Fatalf("record the retry: %v", err)
	}

	if len(h.relayed) != 0 || len(h.settled) != 1 {
		t.Fatalf("a retry underway relayed %+v and settled %+v", h.relayed, h.settled)
	}
}
