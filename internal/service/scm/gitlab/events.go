package gitlab

import (
	"crypto/subtle"
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
	tokenHeader     = "X-Gitlab-Token"
	eventHeader     = "X-Gitlab-Event"
	deliveryHeader  = "X-Gitlab-Event-UUID"
	refBranchPrefix = "refs/heads/"
	deletedCommit   = "0000000000000000000000000000000000000000"
)

func (f *Forge) Verify(secret string, header http.Header, body []byte) (entity.SCMDelivery, error) {
	sent := strings.TrimSpace(header.Get(tokenHeader))

	if sent == "" || subtle.ConstantTimeCompare([]byte(sent), []byte(secret)) != 1 {
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
	Project struct {
		WebURL string `json:"web_url"`
	} `json:"project"`
	UserUsername string `json:"user_username"`
	Commits      []struct {
		ID        string    `json:"id"`
		Message   string    `json:"message"`
		URL       string    `json:"url"`
		Timestamp time.Time `json:"timestamp"`
		Author    struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"author"`
	} `json:"commits"`
}

type changePayload struct {
	User struct {
		Username string `json:"username"`
	} `json:"user"`
	ObjectAttributes struct {
		ID             int64  `json:"id"`
		IID            int    `json:"iid"`
		Title          string `json:"title"`
		Description    string `json:"description"`
		State          string `json:"state"`
		URL            string `json:"url"`
		SourceBranch   string `json:"source_branch"`
		TargetBranch   string `json:"target_branch"`
		Draft          bool   `json:"draft"`
		WorkInProg     bool   `json:"work_in_progress"`
		MergeStatus    string `json:"merge_status"`
		MergeCommitSHA string `json:"merge_commit_sha"`
		LastCommit     struct {
			ID string `json:"id"`
		} `json:"last_commit"`
		DetailedMerge string `json:"detailed_merge_status"`
		Action        string `json:"action"`
		UpdatedAt     string `json:"updated_at"`
		Author        struct {
			Username string `json:"username"`
		} `json:"author"`
	} `json:"object_attributes"`
	Reviewers []struct {
		Username string `json:"username"`
	} `json:"reviewers"`
}

type issuePayload struct {
	User struct {
		Username string `json:"username"`
	} `json:"user"`
	ObjectAttributes struct {
		ID          int64  `json:"id"`
		IID         int    `json:"iid"`
		Title       string `json:"title"`
		Description string `json:"description"`
		State       string `json:"state"`
		URL         string `json:"url"`
		Note        string `json:"note"`
		NoteableID  int64  `json:"noteable_id"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
	} `json:"object_attributes"`
	Labels []struct {
		Title string `json:"title"`
	} `json:"labels"`
	Assignees []struct {
		Username string `json:"username"`
	} `json:"assignees"`
	Issue struct {
		ID        int64  `json:"id"`
		IID       int    `json:"iid"`
		Title     string `json:"title"`
		State     string `json:"state"`
		URL       string `json:"url"`
		UpdatedAt string `json:"updated_at"`
	} `json:"issue"`
}

func (f *Forge) Translate(delivery entity.SCMDelivery) ([]service.ForgeEvent, error) {
	switch delivery.Event {
	case "Push Hook":
		return translatePush(delivery.Payload)
	case "Merge Request Hook":
		return translateChange(delivery.Payload)
	case "Issue Hook":
		return translateIssue(delivery.Payload)
	case "Pipeline Hook":
		return translatePipeline(delivery.Payload)
	case "Note Hook":
		return translateNote(delivery.Payload)
	default:
		return nil, nil
	}
}

func translatePush(body []byte) ([]service.ForgeEvent, error) {
	var payload pushPayload

	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("read push delivery: %w", err)
	}

	if payload.After == deletedCommit {
		return nil, nil
	}

	events := make([]service.ForgeEvent, 0, len(payload.Commits)+1)

	if branch, found := strings.CutPrefix(payload.Ref, refBranchPrefix); found {
		events = append(events, service.ForgeEvent{
			Kind:   service.ForgeEventBranchPushed,
			Branch: service.ForgeBranch{Name: branch, URL: branchURL(payload.Project.WebURL, branch)},
			Author: payload.UserUsername,
		})
	}

	for _, commit := range payload.Commits {
		events = append(events, service.ForgeEvent{
			Kind: service.ForgeEventCommitPushed,
			Commit: service.ForgeCommit{
				SHA:     commit.ID,
				Message: commit.Message,
				URL:     commit.URL,
				Author:  commit.Author.Name,
				At:      commit.Timestamp,
			},
			Author: payload.UserUsername,
			At:     commit.Timestamp,
		})
	}

	return events, nil
}

func translateChange(body []byte) ([]service.ForgeEvent, error) {
	var payload changePayload

	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("read merge request delivery: %w", err)
	}

	change := payload.ObjectAttributes
	updatedAt := parseTime(change.UpdatedAt)
	state := changeState(
		change.Draft || change.WorkInProg,
		change.State,
		change.MergeStatus,
		change.DetailedMerge,
	)

	event := service.ForgeEvent{
		Kind: service.ForgeEventChangeChanged,
		Change: service.ForgeChange{
			ExternalID:     strconv.FormatInt(change.ID, 10),
			Number:         change.IID,
			Title:          change.Title,
			Body:           change.Description,
			URL:            change.URL,
			State:          state,
			Author:         change.Author.Username,
			HeadBranch:     change.SourceBranch,
			HeadSHA:        change.LastCommit.ID,
			BaseBranch:     change.TargetBranch,
			UpdatedAt:      updatedAt,
			MergeCommitSHA: change.MergeCommitSHA,
			ReviewsMoved:   reviewMoved(change.Action),
		},
		Author: payload.User.Username,
		At:     updatedAt,
	}

	switch state {
	case entity.CodeChangeMerged:
		event.Change.MergedAt = &updatedAt
		event.Change.ClosedAt = &updatedAt
	case entity.CodeChangeClosed:
		event.Change.ClosedAt = &updatedAt
	}

	return []service.ForgeEvent{event}, nil
}

func changeState(draft bool, state, mergeStatus, detailed string) entity.CodeChangeState {
	switch {
	case strings.EqualFold(state, "merged"):
		return entity.CodeChangeMerged
	case strings.EqualFold(state, "closed"):
		return entity.CodeChangeClosed
	case draft:
		return entity.CodeChangeDraft
	case conflicted(mergeStatus) || conflicted(detailed):
		return entity.CodeChangeConflicted
	default:
		return entity.CodeChangeOpen
	}
}

func reviewMoved(action string) bool {
	switch strings.ToLower(action) {
	case "approved", "unapproved", "approval", "unapproval", "reviewer_updated", "open", "reopen":
		return true
	default:
		return false
	}
}

func conflicted(status string) bool {
	return strings.EqualFold(status, "cannot_be_merged") ||
		strings.EqualFold(status, "conflict")
}

type pipelinePayload struct {
	ObjectAttributes struct {
		Status string `json:"status"`
	} `json:"object_attributes"`
	MergeRequest struct {
		ID  int64 `json:"id"`
		IID int   `json:"iid"`
	} `json:"merge_request"`
	User struct {
		Username string `json:"username"`
	} `json:"user"`
}

func translatePipeline(body []byte) ([]service.ForgeEvent, error) {
	var payload pipelinePayload

	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("read pipeline delivery: %w", err)
	}

	if payload.MergeRequest.ID == 0 {
		return nil, nil
	}

	return []service.ForgeEvent{{
		Kind: service.ForgeEventChangeChanged,
		Change: service.ForgeChange{
			ExternalID:  strconv.FormatInt(payload.MergeRequest.ID, 10),
			Number:      payload.MergeRequest.IID,
			Checks:      pipelineChecks(payload.ObjectAttributes.Status),
			KnowsChecks: true,
		},
		Author: payload.User.Username,
	}}, nil
}

func pipelineChecks(status string) entity.CodeChecks {
	switch strings.ToLower(status) {
	case "success":
		return entity.CodeChecksPassing
	case "failed", "canceled", "cancelled":
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

	issue := payload.ObjectAttributes
	labels := make([]string, 0, len(payload.Labels))

	for _, label := range payload.Labels {
		labels = append(labels, label.Title)
	}

	assignees := make([]string, 0, len(payload.Assignees))

	for _, assignee := range payload.Assignees {
		if assignee.Username != "" {
			assignees = append(assignees, assignee.Username)
		}
	}

	return []service.ForgeEvent{{
		Kind: service.ForgeEventIssueChanged,
		Issue: service.ForgeIssue{
			ExternalID: strconv.FormatInt(issue.ID, 10),
			Number:     issue.IID,
			Title:      issue.Title,
			Body:       issue.Description,
			URL:        issue.URL,
			State:      issue.State,
			Author:     payload.User.Username,
			Assignees:  assignees,
			Labels:     labels,
			CreatedAt:  parseTime(issue.CreatedAt),
			UpdatedAt:  parseTime(issue.UpdatedAt),
		},
		Author: payload.User.Username,
		At:     parseTime(issue.UpdatedAt),
	}}, nil
}

func translateNote(body []byte) ([]service.ForgeEvent, error) {
	var payload issuePayload

	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("read note delivery: %w", err)
	}

	if payload.Issue.ID == 0 {
		return nil, nil
	}

	note := payload.ObjectAttributes
	at := parseTime(note.UpdatedAt)

	return []service.ForgeEvent{{
		Kind: service.ForgeEventCommented,
		Issue: service.ForgeIssue{
			ExternalID: strconv.FormatInt(payload.Issue.ID, 10),
			Number:     payload.Issue.IID,
			Title:      payload.Issue.Title,
			URL:        payload.Issue.URL,
			State:      payload.Issue.State,
			UpdatedAt:  parseTime(payload.Issue.UpdatedAt),
		},
		Comment: service.ForgeComment{
			ExternalID: strconv.FormatInt(note.ID, 10),
			Body:       note.Note,
			Author:     payload.User.Username,
			URL:        note.URL,
			CreatedAt:  parseTime(note.CreatedAt),
			UpdatedAt:  at,
		},
		Author: payload.User.Username,
		At:     at,
	}}, nil
}

func parseTime(value string) time.Time {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}
	}

	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02 15:04:05 MST",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02T15:04:05.000Z",
	} {
		if parsed, err := time.Parse(layout, trimmed); err == nil {
			return parsed.UTC()
		}
	}

	return time.Time{}
}

func branchURL(project, branch string) string {
	if project == "" {
		return ""
	}

	return project + "/-/tree/" + branch
}
