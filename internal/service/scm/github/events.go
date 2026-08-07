package github

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
	signatureHeader  = "X-Hub-Signature-256"
	eventHeader      = "X-GitHub-Event"
	deliveryHeader   = "X-GitHub-Delivery"
	signaturePrefix  = "sha256="
	refBranchPrefix  = "refs/heads/"
	deletedCommitSHA = "0000000000000000000000000000000000000000"
)

func (f *Forge) Verify(secret string, header http.Header, body []byte) (entity.SCMDelivery, error) {
	sent := strings.TrimSpace(header.Get(signatureHeader))
	if !strings.HasPrefix(sent, signaturePrefix) {
		return entity.SCMDelivery{}, entity.ErrSCMSignatureInvalid
	}

	sum := hmac.New(sha256.New, []byte(secret))
	sum.Write(body)

	expected := hex.EncodeToString(sum.Sum(nil))

	if !hmac.Equal([]byte(strings.TrimPrefix(sent, signaturePrefix)), []byte(expected)) {
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

type pushPayload struct {
	Ref     string `json:"ref"`
	After   string `json:"after"`
	Deleted bool   `json:"deleted"`
	Commits []struct {
		ID        string    `json:"id"`
		Message   string    `json:"message"`
		URL       string    `json:"url"`
		Timestamp time.Time `json:"timestamp"`
		Author    struct {
			Username string `json:"username"`
			Name     string `json:"name"`
		} `json:"author"`
	} `json:"commits"`
	Repository struct {
		HTMLURL string `json:"html_url"`
	} `json:"repository"`
	Sender struct {
		Login string `json:"login"`
	} `json:"sender"`
}

type changePayload struct {
	Action      string `json:"action"`
	PullRequest struct {
		ID                 int64      `json:"id"`
		Number             int        `json:"number"`
		Title              string     `json:"title"`
		Body               string     `json:"body"`
		HTMLURL            string     `json:"html_url"`
		State              string     `json:"state"`
		Draft              bool       `json:"draft"`
		MergedAt           *time.Time `json:"merged_at"`
		ClosedAt           *time.Time `json:"closed_at"`
		UpdatedAt          time.Time  `json:"updated_at"`
		MergeableState     string     `json:"mergeable_state"`
		MergeCommitSHA     string     `json:"merge_commit_sha"`
		RequestedReviewers []struct {
			Login string `json:"login"`
		} `json:"requested_reviewers"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		Head struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
	} `json:"pull_request"`
	Sender struct {
		Login string `json:"login"`
	} `json:"sender"`
}

type issuePayload struct {
	Action string `json:"action"`
	Issue  struct {
		ID        int64     `json:"id"`
		Number    int       `json:"number"`
		Title     string    `json:"title"`
		Body      string    `json:"body"`
		HTMLURL   string    `json:"html_url"`
		State     string    `json:"state"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		User      struct {
			Login string `json:"login"`
		} `json:"user"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		Assignees []struct {
			Login string `json:"login"`
		} `json:"assignees"`
		PullRequest *struct{} `json:"pull_request"`
	} `json:"issue"`
	Comment struct {
		ID        int64     `json:"id"`
		Body      string    `json:"body"`
		HTMLURL   string    `json:"html_url"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		User      struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"comment"`
	Sender struct {
		Login string `json:"login"`
	} `json:"sender"`
}

func (f *Forge) Translate(delivery entity.SCMDelivery) ([]service.ForgeEvent, error) {
	switch delivery.Event {
	case "push":
		return translatePush(delivery.Payload)
	case "pull_request":
		return translateChange(delivery.Payload)
	case "issues":
		return translateIssue(delivery.Payload)
	case "issue_comment":
		return translateComment(delivery.Payload)
	case "pull_request_review":
		return translateReview(delivery.Payload)
	case "check_suite", "check_run":
		return translateChecks(delivery.Payload)
	default:
		return nil, nil
	}
}

func translatePush(body []byte) ([]service.ForgeEvent, error) {
	var payload pushPayload

	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("read push delivery: %w", err)
	}

	if payload.Deleted || payload.After == deletedCommitSHA {
		return nil, nil
	}

	events := make([]service.ForgeEvent, 0, len(payload.Commits)+1)

	if branch, found := strings.CutPrefix(payload.Ref, refBranchPrefix); found {
		events = append(events, service.ForgeEvent{
			Kind:   service.ForgeEventBranchPushed,
			Branch: service.ForgeBranch{Name: branch, URL: branchURL(payload.Repository.HTMLURL, branch)},
			Author: payload.Sender.Login,
		})
	}

	for _, commit := range payload.Commits {
		events = append(events, service.ForgeEvent{
			Kind: service.ForgeEventCommitPushed,
			Commit: service.ForgeCommit{
				SHA:     commit.ID,
				Message: commit.Message,
				URL:     commit.URL,
				Author:  firstNonEmpty(commit.Author.Username, commit.Author.Name),
				At:      commit.Timestamp,
			},
			Author: firstNonEmpty(commit.Author.Username, payload.Sender.Login),
			At:     commit.Timestamp,
		})
	}

	return events, nil
}

func translateChange(body []byte) ([]service.ForgeEvent, error) {
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
			State:          changeState(change.Draft, change.State, change.MergedAt, change.MergeableState),
			MergeCommitSHA: change.MergeCommitSHA,
			ReviewsMoved: payload.Action == "review_requested" ||
				payload.Action == "review_request_removed" ||
				payload.Action == "ready_for_review",
			Author:     change.User.Login,
			HeadBranch: change.Head.Ref,
			BaseBranch: change.Base.Ref,
			UpdatedAt:  change.UpdatedAt,
			MergedAt:   change.MergedAt,
			ClosedAt:   change.ClosedAt,
		},
		Author: payload.Sender.Login,
		At:     change.UpdatedAt,
	}}, nil
}

func changeState(
	draft bool,
	state string,
	mergedAt *time.Time,
	mergeable string,
) entity.CodeChangeState {
	switch {
	case mergedAt != nil:
		return entity.CodeChangeMerged
	case strings.EqualFold(state, "closed"):
		return entity.CodeChangeClosed
	case draft:
		return entity.CodeChangeDraft
	case strings.EqualFold(mergeable, "dirty"):
		return entity.CodeChangeConflicted
	default:
		return entity.CodeChangeOpen
	}
}

type reviewPayload struct {
	Action string `json:"action"`
	Review struct {
		State       string    `json:"state"`
		HTMLURL     string    `json:"html_url"`
		SubmittedAt time.Time `json:"submitted_at"`
		User        struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"review"`
	PullRequest struct {
		ID                 int64      `json:"id"`
		Number             int        `json:"number"`
		Title              string     `json:"title"`
		Body               string     `json:"body"`
		HTMLURL            string     `json:"html_url"`
		State              string     `json:"state"`
		Draft              bool       `json:"draft"`
		MergeableState     string     `json:"mergeable_state"`
		MergedAt           *time.Time `json:"merged_at"`
		ClosedAt           *time.Time `json:"closed_at"`
		UpdatedAt          time.Time  `json:"updated_at"`
		RequestedReviewers []struct {
			Login string `json:"login"`
		} `json:"requested_reviewers"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		Head struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
	} `json:"pull_request"`
	Sender struct {
		Login string `json:"login"`
	} `json:"sender"`
}

func translateReview(body []byte) ([]service.ForgeEvent, error) {
	var payload reviewPayload

	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("read pull request review delivery: %w", err)
	}

	change := payload.PullRequest

	return []service.ForgeEvent{{
		Kind: service.ForgeEventChangeChanged,
		Change: service.ForgeChange{
			ExternalID:   strconv.FormatInt(change.ID, 10),
			Number:       change.Number,
			Title:        change.Title,
			Body:         change.Body,
			URL:          change.HTMLURL,
			State:        changeState(change.Draft, change.State, change.MergedAt, change.MergeableState),
			ReviewsMoved: true,
			Author:       change.User.Login,
			HeadBranch:   change.Head.Ref,
			BaseBranch:   change.Base.Ref,
			UpdatedAt:    change.UpdatedAt,
			MergedAt:     change.MergedAt,
			ClosedAt:     change.ClosedAt,
		},
		Author: payload.Sender.Login,
		At:     change.UpdatedAt,
	}}, nil
}

func reviewVerdict(state string) (entity.ReviewVerdict, bool) {
	switch strings.ToLower(state) {
	case "approved":
		return entity.ReviewApproved, true
	case "changes_requested":
		return entity.ReviewChangesRequested, true
	case "commented":
		return entity.ReviewCommented, true
	case "dismissed":
		return entity.ReviewDismissed, true
	default:
		return "", false
	}
}

type checkPayload struct {
	CheckSuite *struct {
		Status       string `json:"status"`
		Conclusion   string `json:"conclusion"`
		PullRequests []struct {
			ID     int64 `json:"id"`
			Number int   `json:"number"`
		} `json:"pull_requests"`
	} `json:"check_suite"`
	CheckRun *struct {
		Status       string `json:"status"`
		Conclusion   string `json:"conclusion"`
		PullRequests []struct {
			ID     int64 `json:"id"`
			Number int   `json:"number"`
		} `json:"pull_requests"`
	} `json:"check_run"`
	Sender struct {
		Login string `json:"login"`
	} `json:"sender"`
}

func translateChecks(body []byte) ([]service.ForgeEvent, error) {
	var payload checkPayload

	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("read checks delivery: %w", err)
	}

	status, conclusion := "", ""

	var changes []struct {
		ID     int64 `json:"id"`
		Number int   `json:"number"`
	}

	switch {
	case payload.CheckSuite != nil:
		status, conclusion, changes = payload.CheckSuite.Status, payload.CheckSuite.Conclusion, payload.CheckSuite.PullRequests
	case payload.CheckRun != nil:
		status, conclusion, changes = payload.CheckRun.Status, payload.CheckRun.Conclusion, payload.CheckRun.PullRequests
	default:
		return nil, nil
	}

	checks := checkState(status, conclusion)

	events := make([]service.ForgeEvent, 0, len(changes))

	for _, change := range changes {
		events = append(events, service.ForgeEvent{
			Kind: service.ForgeEventChangeChanged,
			Change: service.ForgeChange{
				ExternalID:  strconv.FormatInt(change.ID, 10),
				Number:      change.Number,
				Checks:      checks,
				KnowsChecks: true,
			},
			Author: payload.Sender.Login,
		})
	}

	return events, nil
}

func checkState(status, conclusion string) entity.CodeChecks {
	if !strings.EqualFold(status, "completed") {
		return entity.CodeChecksPending
	}

	switch strings.ToLower(conclusion) {
	case "success", "neutral", "skipped":
		return entity.CodeChecksPassing
	case "failure", "timed_out", "action_required", "cancelled", "stale", "startup_failure":
		return entity.CodeChecksFailing
	default:
		return entity.CodeChecksPending
	}
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
		Issue:  forgeIssue(payload),
		Author: payload.Sender.Login,
		At:     payload.Issue.UpdatedAt,
	}}, nil
}

func translateComment(body []byte) ([]service.ForgeEvent, error) {
	var payload issuePayload

	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("read comment delivery: %w", err)
	}

	if payload.Issue.PullRequest != nil {
		return nil, nil
	}

	if payload.Action == "deleted" {
		return nil, nil
	}

	return []service.ForgeEvent{{
		Kind:  service.ForgeEventCommented,
		Issue: forgeIssue(payload),
		Comment: service.ForgeComment{
			ExternalID: strconv.FormatInt(payload.Comment.ID, 10),
			Body:       payload.Comment.Body,
			Author:     payload.Comment.User.Login,
			URL:        payload.Comment.HTMLURL,
			CreatedAt:  payload.Comment.CreatedAt,
			UpdatedAt:  payload.Comment.UpdatedAt,
		},
		Author: payload.Comment.User.Login,
		At:     payload.Comment.UpdatedAt,
	}}, nil
}

func forgeIssue(payload issuePayload) service.ForgeIssue {
	labels := make([]string, 0, len(payload.Issue.Labels))
	for _, label := range payload.Issue.Labels {
		labels = append(labels, label.Name)
	}

	assignees := make([]string, 0, len(payload.Issue.Assignees))
	for _, assignee := range payload.Issue.Assignees {
		if assignee.Login != "" {
			assignees = append(assignees, assignee.Login)
		}
	}

	return service.ForgeIssue{
		ExternalID: strconv.FormatInt(payload.Issue.ID, 10),
		Number:     payload.Issue.Number,
		Title:      payload.Issue.Title,
		Body:       payload.Issue.Body,
		URL:        payload.Issue.HTMLURL,
		State:      payload.Issue.State,
		Author:     payload.Issue.User.Login,
		Assignees:  assignees,
		Labels:     labels,
		CreatedAt:  payload.Issue.CreatedAt,
		UpdatedAt:  payload.Issue.UpdatedAt,
	}
}

func branchURL(repository, branch string) string {
	if repository == "" {
		return ""
	}

	return repository + "/tree/" + branch
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}

	return ""
}
