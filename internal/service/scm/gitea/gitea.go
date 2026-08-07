package gitea

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/forge"
	"github.com/usenorn/norn/internal/service"
)

type Forge struct {
	client   *forge.Client
	pageSize int
}

func New(client *forge.Client, cfg config.SourceControl) *Forge {
	return &Forge{client: client, pageSize: cfg.PageSize}
}

func (f *Forge) Provider() entity.SCMProvider {
	return entity.SCMProviderGitea
}

func (f *Forge) Endpoint() string {
	return ""
}

func (f *Forge) base(target entity.SCMTarget) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(target.BaseURL), "/")
	if trimmed == "" {
		return "", entity.SCMDestinationRefusedError{
			Provider: entity.SCMProviderGitea,
			Reason:   "a Gitea or Forgejo connection has to name the address of the instance",
		}
	}

	return trimmed + "/api/v1", nil
}

func (f *Forge) call(
	ctx context.Context,
	target entity.SCMTarget,
	method, path string,
	body any,
) (forge.Response, error) {
	address := path

	if !strings.HasPrefix(path, "http") {
		base, err := f.base(target)
		if err != nil {
			return forge.Response{}, err
		}

		address = base + path
	}

	request := forge.Request{
		Trust:    target.Trust,
		Provider: entity.SCMProviderGitea,
		Method:   method,
		URL:      address,
		Header: http.Header{
			"Accept":        {"application/json"},
			"Authorization": {"token " + target.Token},
		},
	}

	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return forge.Response{}, fmt.Errorf("encode request: %w", err)
		}

		request.Body = encoded
		request.Header.Set("Content-Type", "application/json")
	}

	return f.client.Do(ctx, request)
}

func (f *Forge) decode(response forge.Response, target entity.SCMTarget, into any) error {
	if response.Status == http.StatusUnauthorized || response.Status == http.StatusForbidden {
		return entity.SCMCredentialsRejectedError{
			Provider: entity.SCMProviderGitea,
			Reason:   "the token was refused by the instance",
		}
	}

	if response.Status == http.StatusNotFound {
		return entity.SCMRepositoryUnreachableError{
			Provider:   entity.SCMProviderGitea,
			Repository: target.Repository,
			Reason:     "the repository is not visible to this token",
		}
	}

	if response.Status < 200 || response.Status > 299 {
		return entity.SCMUnavailableError{
			Provider: entity.SCMProviderGitea,
			Reason:   fmt.Sprintf("answered %d", response.Status),
		}
	}

	if into == nil {
		return nil
	}

	if err := json.Unmarshal(response.Body, into); err != nil {
		return fmt.Errorf("read the response from the forge: %w", err)
	}

	return nil
}

type repositoryBody struct {
	ID            int64  `json:"id"`
	FullName      string `json:"full_name"`
	HTMLURL       string `json:"html_url"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
	Permissions   struct {
		Admin bool `json:"admin"`
	} `json:"permissions"`
}

func (f *Forge) Repository(
	ctx context.Context,
	target entity.SCMTarget,
) (entity.SCMRemoteRepository, error) {
	response, err := f.call(ctx, target, http.MethodGet, "/repos/"+target.Repository, nil)
	if err != nil {
		return entity.SCMRemoteRepository{}, err
	}

	var body repositoryBody

	if err := f.decode(response, target, &body); err != nil {
		return entity.SCMRemoteRepository{}, err
	}

	return entity.SCMRemoteRepository{
		ExternalID:    strconv.FormatInt(body.ID, 10),
		FullName:      body.FullName,
		URL:           body.HTMLURL,
		DefaultBranch: body.DefaultBranch,
		Private:       body.Private,
		CanAdmin:      body.Permissions.Admin,
	}, nil
}

func (f *Forge) Identity(ctx context.Context, target entity.SCMTarget) (string, error) {
	response, err := f.call(ctx, target, http.MethodGet, "/user", nil)
	if err != nil {
		return "", err
	}

	var body struct {
		Login string `json:"login"`
	}

	if err := f.decode(response, target, &body); err != nil {
		return "", err
	}

	return body.Login, nil
}

var hookEvents = []string{
	"push",
	"pull_request",
	"pull_request_review_approved",
	"pull_request_review_rejected",
	"pull_request_review_comment",
	"issues",
	"issue_comment",
}

func (f *Forge) InstallHook(
	ctx context.Context,
	request service.ForgeHookRequest,
) (string, error) {
	payload := map[string]any{
		"type":   "gitea",
		"active": true,
		"events": hookEvents,
		"config": map[string]any{
			"url":          request.CallbackURL,
			"content_type": "json",
			"secret":       request.Secret,
		},
	}

	response, err := f.call(
		ctx,
		request.Target,
		http.MethodPost,
		"/repos/"+request.Target.Repository+"/hooks",
		payload,
	)
	if err != nil {
		return "", err
	}

	var body struct {
		ID int64 `json:"id"`
	}

	if err := f.decode(response, request.Target, &body); err != nil {
		return "", err
	}

	return strconv.FormatInt(body.ID, 10), nil
}

func (f *Forge) RepairHook(
	ctx context.Context,
	request service.ForgeHookRequest,
	hookID string,
) (bool, error) {
	path := "/repos/" + request.Target.Repository + "/hooks/" + hookID

	response, err := f.call(ctx, request.Target, http.MethodGet, path, nil)
	if err != nil {
		return false, err
	}

	var installed struct {
		Events []string `json:"events"`
	}

	if err := f.decode(response, request.Target, &installed); err != nil {
		return false, err
	}

	held := make(map[string]bool, len(installed.Events))
	for _, event := range installed.Events {
		held[event] = true
	}

	short := false

	for _, event := range hookEvents {
		if !held[event] {
			short = true

			break
		}
	}

	if !short {
		return false, nil
	}

	patched, err := f.call(ctx, request.Target, http.MethodPatch, path, map[string]any{
		"active": true,
		"events": hookEvents,
		"config": map[string]any{
			"url":          request.CallbackURL,
			"content_type": "json",
			"secret":       request.Secret,
		},
	})
	if err != nil {
		return false, err
	}

	return true, f.decode(patched, request.Target, nil)
}

func (f *Forge) RemoveHook(
	ctx context.Context,
	target entity.SCMTarget,
	hookID string,
) error {
	response, err := f.call(
		ctx,
		target,
		http.MethodDelete,
		"/repos/"+target.Repository+"/hooks/"+hookID,
		nil,
	)
	if err != nil {
		return err
	}

	return f.decode(response, target, nil)
}

type changeBody struct {
	ID        int64      `json:"id"`
	Number    int        `json:"number"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	HTMLURL   string     `json:"html_url"`
	State     string     `json:"state"`
	Draft     bool       `json:"draft"`
	Mergeable bool       `json:"mergeable"`
	Merged    bool       `json:"merged"`
	MergedAt  *time.Time `json:"merged_at"`
	ClosedAt  *time.Time `json:"closed_at"`
	Updated   time.Time  `json:"updated_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
	Head struct {
		Ref string `json:"ref"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
	RequestedReviewers []struct {
		Login string `json:"login"`
	} `json:"requested_reviewers"`
}

func changeState(change changeBody) entity.CodeChangeState {
	switch {
	case change.Merged || change.MergedAt != nil:
		return entity.CodeChangeMerged
	case strings.EqualFold(change.State, "closed"):
		return entity.CodeChangeClosed
	case change.Draft:
		return entity.CodeChangeDraft
	default:
		return entity.CodeChangeOpen
	}
}

func (f *Forge) forgeChange(change changeBody) service.ForgeChange {
	return service.ForgeChange{
		ExternalID:   strconv.FormatInt(change.ID, 10),
		Number:       change.Number,
		Title:        change.Title,
		Body:         change.Body,
		URL:          change.HTMLURL,
		State:        changeState(change),
		ReviewsMoved: true,
		Author:       change.User.Login,
		HeadBranch:   change.Head.Ref,
		BaseBranch:   change.Base.Ref,
		UpdatedAt:    change.Updated,
		MergedAt:     change.MergedAt,
		ClosedAt:     change.ClosedAt,
	}
}

func (f *Forge) Changes(
	ctx context.Context,
	target entity.SCMTarget,
	since time.Time,
	cursor string,
) (service.ForgeChangePage, error) {
	query := url.Values{}
	query.Set("state", "all")
	query.Set("sort", "recentupdate")
	query.Set("limit", strconv.Itoa(f.pageSize))

	page := 1
	if cursor != "" {
		if parsed, err := strconv.Atoi(cursor); err == nil && parsed > 1 {
			page = parsed
		}
	}

	query.Set("page", strconv.Itoa(page))

	response, err := f.call(
		ctx,
		target,
		http.MethodGet,
		"/repos/"+target.Repository+"/pulls?"+query.Encode(),
		nil,
	)
	if err != nil {
		return service.ForgeChangePage{}, err
	}

	var body []changeBody

	if err := f.decode(response, target, &body); err != nil {
		return service.ForgeChangePage{}, err
	}

	var found service.ForgeChangePage

	for _, change := range body {
		if !since.IsZero() && change.Updated.Before(since) {
			return found, nil
		}

		found.Changes = append(found.Changes, f.forgeChange(change))
	}

	if len(body) == f.pageSize {
		found.Cursor = strconv.Itoa(page + 1)
	}

	return found, nil
}

func (f *Forge) ChangedPaths(
	ctx context.Context,
	target entity.SCMTarget,
	number int,
) ([]string, error) {
	query := url.Values{}
	query.Set("limit", strconv.Itoa(f.pageSize))

	path := "/repos/" + target.Repository + "/pulls/" + strconv.Itoa(number) +
		"/files?" + query.Encode()

	response, err := f.call(ctx, target, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var body []struct {
		Filename         string `json:"filename"`
		PreviousFilename string `json:"previous_filename"`
	}

	if err := f.decode(response, target, &body); err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(body))

	for _, file := range body {
		if file.Filename != "" {
			paths = append(paths, file.Filename)
		}

		if file.PreviousFilename != "" && file.PreviousFilename != file.Filename {
			paths = append(paths, file.PreviousFilename)
		}
	}

	return paths, nil
}

func (f *Forge) Reviews(
	ctx context.Context,
	target entity.SCMTarget,
	number int,
) ([]service.ForgeReviewer, error) {
	path := "/repos/" + target.Repository + "/pulls/" + strconv.Itoa(number) + "/reviews"

	response, err := f.call(ctx, target, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var body []struct {
		State     string    `json:"state"`
		HTMLURL   string    `json:"html_url"`
		Submitted time.Time `json:"submitted_at"`
		Dismissed bool      `json:"dismissed"`
		User      struct {
			Login string `json:"login"`
		} `json:"user"`
	}

	if err := f.decode(response, target, &body); err != nil {
		return nil, err
	}

	at := make(map[string]int, len(body))
	reviewers := make([]service.ForgeReviewer, 0, len(body))

	for _, review := range body {
		if review.User.Login == "" {
			continue
		}

		verdict, known := reviewVerdict(review.State, review.Dismissed)
		if !known {
			continue
		}

		submitted := review.Submitted
		reviewer := service.ForgeReviewer{
			Login:      review.User.Login,
			Verdict:    verdict,
			URL:        review.HTMLURL,
			ReviewedAt: &submitted,
		}

		if index, seen := at[review.User.Login]; seen {
			reviewers[index] = reviewer

			continue
		}

		at[review.User.Login] = len(reviewers)
		reviewers = append(reviewers, reviewer)
	}

	change, err := f.change(ctx, target, number)
	if err != nil {
		return reviewers, nil
	}

	for _, requested := range change.RequestedReviewers {
		if requested.Login == "" {
			continue
		}

		if _, answered := at[requested.Login]; answered {
			continue
		}

		at[requested.Login] = len(reviewers)
		reviewers = append(reviewers, service.ForgeReviewer{
			Login:   requested.Login,
			Verdict: entity.ReviewRequested,
		})
	}

	return reviewers, nil
}

func reviewVerdict(state string, dismissed bool) (entity.ReviewVerdict, bool) {
	if dismissed {
		return entity.ReviewDismissed, true
	}

	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "APPROVED":
		return entity.ReviewApproved, true
	case "REQUEST_CHANGES":
		return entity.ReviewChangesRequested, true
	case "COMMENT":
		return entity.ReviewCommented, true
	default:
		return "", false
	}
}

func (f *Forge) change(
	ctx context.Context,
	target entity.SCMTarget,
	number int,
) (changeBody, error) {
	response, err := f.call(
		ctx,
		target,
		http.MethodGet,
		"/repos/"+target.Repository+"/pulls/"+strconv.Itoa(number),
		nil,
	)
	if err != nil {
		return changeBody{}, err
	}

	var body changeBody

	if err := f.decode(response, target, &body); err != nil {
		return changeBody{}, err
	}

	return body, nil
}

type issueBody struct {
	ID      int64     `json:"id"`
	Number  int       `json:"number"`
	Title   string    `json:"title"`
	Body    string    `json:"body"`
	HTMLURL string    `json:"html_url"`
	State   string    `json:"state"`
	Created time.Time `json:"created_at"`
	Updated time.Time `json:"updated_at"`
	User    struct {
		Login string `json:"login"`
	} `json:"user"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Assignees []struct {
		Login string `json:"login"`
	} `json:"assignees"`
	PullRequest *struct{} `json:"pull_request"`
}

func forgeIssue(body issueBody) service.ForgeIssue {
	labels := make([]string, 0, len(body.Labels))
	for _, label := range body.Labels {
		labels = append(labels, label.Name)
	}

	assignees := make([]string, 0, len(body.Assignees))

	for _, assignee := range body.Assignees {
		if assignee.Login != "" {
			assignees = append(assignees, assignee.Login)
		}
	}

	return service.ForgeIssue{
		ExternalID: strconv.FormatInt(body.ID, 10),
		Number:     body.Number,
		Title:      body.Title,
		Body:       body.Body,
		URL:        body.HTMLURL,
		State:      body.State,
		Author:     body.User.Login,
		Assignees:  assignees,
		Labels:     labels,
		CreatedAt:  body.Created,
		UpdatedAt:  body.Updated,
	}
}

func (f *Forge) Issues(
	ctx context.Context,
	target entity.SCMTarget,
	label string,
	since time.Time,
	cursor string,
) (service.ForgeIssuePage, error) {
	query := url.Values{}
	query.Set("state", "all")
	query.Set("type", "issues")
	query.Set("limit", strconv.Itoa(f.pageSize))

	if label != "" {
		query.Set("labels", label)
	}

	if !since.IsZero() {
		query.Set("since", since.UTC().Format(time.RFC3339))
	}

	page := 1
	if cursor != "" {
		if parsed, err := strconv.Atoi(cursor); err == nil && parsed > 1 {
			page = parsed
		}
	}

	query.Set("page", strconv.Itoa(page))

	response, err := f.call(
		ctx,
		target,
		http.MethodGet,
		"/repos/"+target.Repository+"/issues?"+query.Encode(),
		nil,
	)
	if err != nil {
		return service.ForgeIssuePage{}, err
	}

	var body []issueBody

	if err := f.decode(response, target, &body); err != nil {
		return service.ForgeIssuePage{}, err
	}

	var found service.ForgeIssuePage

	for _, issue := range body {
		if issue.PullRequest != nil {
			continue
		}

		found.Issues = append(found.Issues, forgeIssue(issue))
	}

	if len(body) == f.pageSize {
		found.Cursor = strconv.Itoa(page + 1)
	}

	return found, nil
}

func (f *Forge) Issue(
	ctx context.Context,
	target entity.SCMTarget,
	number int,
) (service.ForgeIssue, error) {
	response, err := f.call(ctx, target, http.MethodGet, f.issuePath(target, number), nil)
	if err != nil {
		return service.ForgeIssue{}, err
	}

	var body issueBody

	if err := f.decode(response, target, &body); err != nil {
		return service.ForgeIssue{}, err
	}

	return forgeIssue(body), nil
}

func (f *Forge) issuePath(target entity.SCMTarget, number int) string {
	return "/repos/" + target.Repository + "/issues/" + strconv.Itoa(number)
}

func (f *Forge) AmendIssue(
	ctx context.Context,
	target entity.SCMTarget,
	number int,
	patch service.ForgeIssuePatch,
) (service.ForgeIssue, error) {
	payload := map[string]any{}

	if patch.Title != nil {
		payload["title"] = *patch.Title
	}

	if patch.Body != nil {
		payload["body"] = *patch.Body
	}

	if patch.Assignee != nil {
		payload["assignees"] = []string{*patch.Assignee}
	}

	if patch.Labels != nil {
		payload["labels"] = patch.Labels
	}

	if patch.Closed != nil {
		payload["state"] = "open"
		if *patch.Closed {
			payload["state"] = "closed"
		}
	}

	if len(payload) == 0 {
		return f.Issue(ctx, target, number)
	}

	response, err := f.call(ctx, target, http.MethodPatch, f.issuePath(target, number), payload)
	if err != nil {
		return service.ForgeIssue{}, err
	}

	var body issueBody

	if err := f.decode(response, target, &body); err != nil {
		return service.ForgeIssue{}, err
	}

	return forgeIssue(body), nil
}

type commentBody struct {
	ID      int64     `json:"id"`
	Body    string    `json:"body"`
	HTMLURL string    `json:"html_url"`
	Created time.Time `json:"created_at"`
	Updated time.Time `json:"updated_at"`
	User    struct {
		Login string `json:"login"`
	} `json:"user"`
}

func (c commentBody) forgeComment() service.ForgeComment {
	return service.ForgeComment{
		ExternalID: strconv.FormatInt(c.ID, 10),
		Body:       c.Body,
		Author:     c.User.Login,
		URL:        c.HTMLURL,
		CreatedAt:  c.Created,
		UpdatedAt:  c.Updated,
	}
}

func (f *Forge) Comments(
	ctx context.Context,
	target entity.SCMTarget,
	number int,
	since time.Time,
) ([]service.ForgeComment, error) {
	query := url.Values{}
	query.Set("limit", strconv.Itoa(f.pageSize))

	if !since.IsZero() {
		query.Set("since", since.UTC().Format(time.RFC3339))
	}

	path := f.issuePath(target, number) + "/comments?" + query.Encode()

	response, err := f.call(ctx, target, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var body []commentBody

	if err := f.decode(response, target, &body); err != nil {
		return nil, err
	}

	comments := make([]service.ForgeComment, 0, len(body))
	for _, comment := range body {
		comments = append(comments, comment.forgeComment())
	}

	return comments, nil
}

func (f *Forge) PostComment(
	ctx context.Context,
	target entity.SCMTarget,
	number int,
	body string,
) (service.ForgeComment, error) {
	response, err := f.call(
		ctx,
		target,
		http.MethodPost,
		f.issuePath(target, number)+"/comments",
		map[string]any{"body": body},
	)
	if err != nil {
		return service.ForgeComment{}, err
	}

	var created commentBody

	if err := f.decode(response, target, &created); err != nil {
		return service.ForgeComment{}, err
	}

	return created.forgeComment(), nil
}

func (f *Forge) Capabilities() entity.SCMCapabilitySet {
	return entity.SCMCapabilitySet{
		entity.CapabilityWebhooks,
		entity.CapabilityReviews,
		entity.CapabilityChangedPaths,
		entity.CapabilityIssues,
		entity.CapabilityLabels,
		entity.CapabilityAssignees,
	}
}
