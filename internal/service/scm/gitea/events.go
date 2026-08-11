package gitea

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

const (
	signatureHeader = "X-Gitea-Signature"
	eventHeader     = "X-Gitea-Event"
	deliveryHeader  = "X-Gitea-Delivery"
)

func (f *Forge) Verify(
	secret string,
	header http.Header,
	body []byte,
) (entity.SCMDelivery, error) {
	sent := strings.TrimSpace(header.Get(signatureHeader))
	if sent == "" {
		return entity.SCMDelivery{}, entity.ErrSCMSignatureInvalid
	}

	sum := hmac.New(sha256.New, []byte(secret))
	sum.Write(body)

	expected := hex.EncodeToString(sum.Sum(nil))

	if !hmac.Equal([]byte(sent), []byte(expected)) {
		return entity.SCMDelivery{}, entity.ErrSCMSignatureInvalid
	}

	event := strings.TrimSpace(header.Get(eventHeader))
	if event == "" {
		return entity.SCMDelivery{}, entity.ErrSCMSignatureInvalid
	}

	return entity.SCMDelivery{
		ExternalID: strings.TrimSpace(header.Get(deliveryHeader)),
		Event:      event,
		Payload:    body,
	}, nil
}

func (f *Forge) Translate(delivery entity.SCMDelivery) ([]service.ForgeEvent, error) {
	switch delivery.Event {
	case "push":
		return translatePush(delivery.Payload)
	case "pull_request",
		"pull_request_review_approved",
		"pull_request_review_rejected",
		"pull_request_review_comment":
		return translateChange(delivery.Payload, delivery.Event)
	case "issues":
		return translateIssue(delivery.Payload)
	case "issue_comment":
		return translateComment(delivery.Payload)
	default:
		return nil, nil
	}
}

type pushPayload struct {
	Ref        string `json:"ref"`
	Repository struct {
		HTMLURL string `json:"html_url"`
	} `json:"repository"`
	Commits []struct {
		ID      string    `json:"id"`
		Message string    `json:"message"`
		URL     string    `json:"url"`
		Time    time.Time `json:"timestamp"`
		Author  struct {
			Name string `json:"name"`
		} `json:"author"`
	} `json:"commits"`
	Pusher struct {
		Login string `json:"login"`
	} `json:"pusher"`
}

func translatePush(body []byte) ([]service.ForgeEvent, error) {
	var payload pushPayload

	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("read push delivery: %w", err)
	}

	branch := strings.TrimPrefix(payload.Ref, "refs/heads/")
	if branch == payload.Ref || branch == "" {
		return nil, nil
	}

	events := make([]service.ForgeEvent, 0, len(payload.Commits)+1)

	events = append(events, service.ForgeEvent{
		Kind: service.ForgeEventBranchPushed,
		Branch: service.ForgeBranch{
			Name: branch,
			URL:  branchURL(payload.Repository.HTMLURL, branch),
		},
		Author: payload.Pusher.Login,
	})

	for _, commit := range payload.Commits {
		events = append(events, service.ForgeEvent{
			Kind: service.ForgeEventCommitPushed,
			Commit: service.ForgeCommit{
				SHA:     commit.ID,
				Message: commit.Message,
				URL:     commit.URL,
				Author:  commit.Author.Name,
				At:      commit.Time,
			},
			Author: payload.Pusher.Login,
			At:     commit.Time,
		})
	}

	return events, nil
}

func branchURL(repository, branch string) string {
	if repository == "" {
		return ""
	}

	return strings.TrimRight(repository, "/") + "/src/branch/" + branch
}

type changePayload struct {
	Action      string     `json:"action"`
	PullRequest changeBody `json:"pull_request"`
	Sender      struct {
		Login string `json:"login"`
	} `json:"sender"`
}

func translateChange(body []byte, event string) ([]service.ForgeEvent, error) {
	var payload changePayload

	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("read pull request delivery: %w", err)
	}

	change := payload.PullRequest

	return []service.ForgeEvent{{
		Kind: service.ForgeEventChangeChanged,
		Change: service.ForgeChange{
			ExternalID:     strconv.FormatInt(change.ID, 10),
			Number:         change.Number,
			Title:          change.Title,
			Body:           change.Body,
			URL:            change.HTMLURL,
			State:          changeState(change),
			MergeCommitSHA: change.MergedSHA,
			ReviewsMoved:   reviewMoved(event, payload.Action),
			Author:         change.User.Login,
			HeadBranch:     change.Head.Ref,
			HeadSHA:        change.Head.SHA,
			BaseBranch:     change.Base.Ref,
			UpdatedAt:      change.Updated,
			MergedAt:       change.MergedAt,
			ClosedAt:       change.ClosedAt,
		},
		Author: payload.Sender.Login,
		At:     change.Updated,
	}}, nil
}

func reviewMoved(event, action string) bool {
	if strings.HasPrefix(event, "pull_request_review") {
		return true
	}

	switch strings.ToLower(action) {
	case "review_requested", "review_request_removed", "opened", "reopened":
		return true
	default:
		return false
	}
}

type issuePayload struct {
	Action string    `json:"action"`
	Issue  issueBody `json:"issue"`
	Sender struct {
		Login string `json:"login"`
	} `json:"sender"`
	Comment *commentBody `json:"comment"`
}

func translateIssue(body []byte) ([]service.ForgeEvent, error) {
	var payload issuePayload

	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("read issue delivery: %w", err)
	}

	if payload.Issue.PullRequest != nil {
		return nil, nil
	}

	return []service.ForgeEvent{{
		Kind:   service.ForgeEventIssueChanged,
		Issue:  forgeIssue(payload.Issue),
		Author: payload.Sender.Login,
		At:     payload.Issue.Updated,
	}}, nil
}

func translateComment(body []byte) ([]service.ForgeEvent, error) {
	var payload issuePayload

	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("read comment delivery: %w", err)
	}

	if payload.Issue.PullRequest != nil || payload.Comment == nil {
		return nil, nil
	}

	if strings.EqualFold(payload.Action, "deleted") {
		return nil, nil
	}

	return []service.ForgeEvent{{
		Kind:    service.ForgeEventCommented,
		Issue:   forgeIssue(payload.Issue),
		Comment: payload.Comment.forgeComment(),
		Author:  payload.Sender.Login,
		At:      payload.Comment.Updated,
	}}, nil
}
