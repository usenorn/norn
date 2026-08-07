package scm

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (s *connections) reads(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) (entity.Decision, entity.Issue, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return entity.Decision{}, entity.Issue{}, err
	}

	issue, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
	if err != nil {
		return entity.Decision{}, entity.Issue{}, err
	}

	return decision, issue, nil
}

func (s *connections) manages(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) (entity.Decision, entity.Issue, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionManage,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return entity.Decision{}, entity.Issue{}, err
	}

	issue, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
	if err != nil {
		return entity.Decision{}, entity.Issue{}, err
	}

	return decision, issue, nil
}

func (s *connections) Links(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) ([]entity.CodeLink, error) {
	if _, _, err := s.reads(ctx, workspaceID, issueID); err != nil {
		return nil, err
	}

	return s.links.ListByIssue(ctx, workspaceID, issueID)
}

// Link takes the address a person pasted rather than a set of fields, because that is what
// they have in front of them. What the forge calls the thing is read out of the address and
// checked against a connection this workspace actually holds.
func (s *connections) Link(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	input service.LinkIssueCodeInput,
) (entity.CodeLink, error) {
	decision, issue, err := s.manages(ctx, workspaceID, issueID)
	if err != nil {
		return entity.CodeLink{}, err
	}

	parsed, err := parseCodeURL(input.URL)
	if err != nil {
		return entity.CodeLink{}, err
	}

	available, err := s.connections.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return entity.CodeLink{}, err
	}

	connection, found := matching(available, parsed)
	if !found {
		return entity.CodeLink{}, entity.ValidationError{
			Fields: []entity.FieldError{{Field: "url", Code: entity.ValidationCodeUnsupportedValue}},
		}
	}

	if !connection.Covers(issue.TeamID) || !decision.Scope.Covers(issue.TeamID) {
		return entity.CodeLink{}, entity.ErrSCMTeamOutsideConnection
	}

	link, err := s.links.Upsert(ctx, entity.CodeLink{
		WorkspaceID:  workspaceID,
		IssueID:      issueID,
		ConnectionID: connection.ID,
		Provider:     connection.Provider,
		Repository:   connection.Repository,
		Kind:         parsed.kind,
		ExternalID:   parsed.externalID,
		Number:       parsed.number,
		URL:          input.URL,
		State:        entity.CodeChangeOpen,
		DetectedIn:   "a person linked it",
	})
	if err != nil {
		return entity.CodeLink{}, err
	}

	s.recordLinked(ctx, workspaceID, issue, link, decision)

	return link, nil
}

func (s *connections) Unlink(ctx context.Context, workspaceID, issueID, linkID uuid.UUID) error {
	decision, issue, err := s.manages(ctx, workspaceID, issueID)
	if err != nil {
		return err
	}

	link, err := s.links.Delete(ctx, workspaceID, issueID, linkID)
	if err != nil {
		return err
	}

	s.recordUnlinked(ctx, workspaceID, issue, link, decision)

	return nil
}

func (s *connections) recordLinked(
	ctx context.Context,
	workspaceID uuid.UUID,
	issue entity.Issue,
	link entity.CodeLink,
	decision entity.Decision,
) {
	s.recordActivity(ctx, workspaceID, issue, link, decision, entity.ActivityKindCodeLinked)
}

func (s *connections) recordUnlinked(
	ctx context.Context,
	workspaceID uuid.UUID,
	issue entity.Issue,
	link entity.CodeLink,
	decision entity.Decision,
) {
	s.recordActivity(ctx, workspaceID, issue, link, decision, entity.ActivityKindCodeUnlinked)
}

func (s *connections) recordActivity(
	ctx context.Context,
	workspaceID uuid.UUID,
	issue entity.Issue,
	link entity.CodeLink,
	decision entity.Decision,
	kind entity.ActivityKind,
) {
	if err := s.activity.Record(ctx, entity.Activity{
		WorkspaceID: workspaceID,
		Subject:     entity.IssueSubject(issue.ID),
		Actor:       decision.ActivityActor(),
		Kind:        kind,
		Field:       string(link.Kind),
		ToValue:     linkLabel(link),
		Version:     issue.Version,
	}); err != nil {
		logWarn(ctx, "recording a code link on the issue feed failed", link.ID, err)
	}
}

func linkLabel(link entity.CodeLink) string {
	if link.Number > 0 {
		return link.Repository + "#" + strconv.Itoa(link.Number)
	}

	return link.Repository + " " + link.ExternalID
}

type codeAddress struct {
	host       string
	repository string
	kind       entity.CodeLinkKind
	externalID string
	number     int
}

// parseCodeURL reads what both forges put in the address bar. GitHub numbers a change under
// /pull and GitLab under /-/merge_requests, and each carries the repository path before it.
func parseCodeURL(raw string) (codeAddress, error) {
	invalid := entity.ValidationError{
		Fields: []entity.FieldError{{Field: "url", Code: entity.ValidationCodeUnsupportedValue}},
	}

	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return codeAddress{}, invalid
	}

	// The address is stored and later rendered as a link on the issue, so a scheme the
	// browser would execute rather than follow must never get that far.
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return codeAddress{}, invalid
	}

	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")

	for i, segment := range segments {
		var (
			kind entity.CodeLinkKind
			skip int
		)

		switch segment {
		case "pull", "pulls", "merge_requests":
			kind, skip = entity.CodeLinkChange, 1
		case "commit", "commits":
			kind, skip = entity.CodeLinkCommit, 1
		case "tree":
			kind, skip = entity.CodeLinkBranch, 1
		default:
			continue
		}

		if i+skip >= len(segments) {
			return codeAddress{}, invalid
		}

		value := segments[i+skip]
		repository := strings.Join(trimGitLabMarker(segments[:i]), "/")

		if repository == "" || value == "" {
			return codeAddress{}, invalid
		}

		address := codeAddress{
			host:       parsed.Host,
			repository: repository,
			kind:       kind,
			externalID: value,
		}

		if number, err := strconv.Atoi(value); err == nil {
			address.number = number
		}

		return address, nil
	}

	return codeAddress{}, invalid
}

// trimGitLabMarker drops the "/-/" segment GitLab puts between a project path and what
// follows, so the repository reads the same on both forges.
func trimGitLabMarker(segments []string) []string {
	kept := make([]string, 0, len(segments))

	for _, segment := range segments {
		if segment == "-" {
			continue
		}

		kept = append(kept, segment)
	}

	return kept
}

func matching(available []entity.SCMConnection, address codeAddress) (entity.SCMConnection, bool) {
	for _, connection := range available {
		if !strings.EqualFold(connection.Repository, address.repository) {
			continue
		}

		if connection.BaseURL != "" {
			if parsed, err := url.Parse(connection.BaseURL); err == nil &&
				!strings.EqualFold(parsed.Host, address.host) {
				continue
			}
		}

		return connection, true
	}

	return entity.SCMConnection{}, false
}

func (s *connections) MirrorOf(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) (entity.IssueMirror, error) {
	if _, _, err := s.reads(ctx, workspaceID, issueID); err != nil {
		return entity.IssueMirror{}, err
	}

	return s.mirrors.GetByIssue(ctx, workspaceID, issueID)
}

func (s *connections) Mirror(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	input service.MirrorIssueInput,
) (entity.IssueMirror, error) {
	decision, issue, err := s.manages(ctx, workspaceID, issueID)
	if err != nil {
		return entity.IssueMirror{}, err
	}

	connection, err := s.connections.GetByID(ctx, workspaceID, input.ConnectionID)
	if err != nil {
		return entity.IssueMirror{}, err
	}

	if !connection.Covers(issue.TeamID) || !decision.Scope.Covers(issue.TeamID) {
		return entity.IssueMirror{}, entity.ErrSCMTeamOutsideConnection
	}

	number, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(input.Reference), "#"))
	if err != nil || number < 1 {
		return entity.IssueMirror{}, entity.ValidationError{
			Fields: []entity.FieldError{{
				Field: "reference",
				Code:  entity.ValidationCodeUnsupportedValue,
			}},
		}
	}

	credentials, err := s.connections.Credentials(ctx, connection.ID)
	if err != nil {
		return entity.IssueMirror{}, err
	}

	forge, err := s.forges.Lookup(connection.Provider)
	if err != nil {
		return entity.IssueMirror{}, err
	}

	found, err := forge.Issue(ctx, connection.Target(credentials.Token), number)
	if err != nil {
		s.breakOn(ctx, connection, err)

		return entity.IssueMirror{}, err
	}

	mirror, err := s.mirrors.Create(ctx, entity.IssueMirror{
		WorkspaceID:    workspaceID,
		IssueID:        issueID,
		ConnectionID:   connection.ID,
		Provider:       connection.Provider,
		Repository:     connection.Repository,
		ExternalID:     found.ExternalID,
		ExternalNumber: found.Number,
		URL:            found.URL,
		Origin:         entity.MirrorOriginNorn,
	})
	if err != nil {
		return entity.IssueMirror{}, err
	}

	hashes := entity.HashesOf(found.Title, found.Body, found.State)

	if err := s.mirrors.RecordPull(
		ctx,
		mirror.ID,
		hashes,
		found.UpdatedAt,
		issue.Version,
		time.Now().UTC(),
	); err != nil {
		return entity.IssueMirror{}, err
	}

	return s.mirrors.GetByIssue(ctx, workspaceID, issueID)
}

func (s *connections) Unmirror(ctx context.Context, workspaceID, issueID uuid.UUID) error {
	if _, _, err := s.manages(ctx, workspaceID, issueID); err != nil {
		return err
	}

	return s.mirrors.Delete(ctx, workspaceID, issueID)
}
