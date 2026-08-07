package entity

import (
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrSCMReleaseNotFound       = errors.New("release not found")
	ErrSCMCapabilityUnsupported = errors.New("this platform does not offer that")
)

type SCMRelease struct {
	ID           uuid.UUID
	RepositoryID uuid.UUID
	WorkspaceID  uuid.UUID
	ExternalID   string
	Tag          string
	Name         string
	URL          string
	CommitSHA    string
	Prerelease   bool
	PublishedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (r SCMRelease) DisplayName() string {
	if trimmed := strings.TrimSpace(r.Name); trimmed != "" {
		return trimmed
	}

	return r.Tag
}

type SCMReleases []SCMRelease

func (releases SCMReleases) Previous(release SCMRelease) (SCMRelease, bool) {
	for _, candidate := range releases {
		if candidate.ID == release.ID || candidate.Prerelease {
			continue
		}

		if publishedBefore(candidate, release) {
			return candidate, true
		}
	}

	return SCMRelease{}, false
}

func publishedBefore(candidate, release SCMRelease) bool {
	if candidate.PublishedAt == nil || release.PublishedAt == nil {
		return false
	}

	return candidate.PublishedAt.Before(*release.PublishedAt)
}

type DeploymentState string

const (
	DeploymentPending   DeploymentState = "pending"
	DeploymentRunning   DeploymentState = "running"
	DeploymentSucceeded DeploymentState = "succeeded"
	DeploymentFailed    DeploymentState = "failed"
	DeploymentInactive  DeploymentState = "inactive"
)

func DeploymentStates() []DeploymentState {
	return []DeploymentState{
		DeploymentPending,
		DeploymentRunning,
		DeploymentSucceeded,
		DeploymentFailed,
		DeploymentInactive,
	}
}

func (s DeploymentState) Valid() bool {
	return slices.Contains(DeploymentStates(), s)
}

func (s DeploymentState) Live() bool {
	return s == DeploymentSucceeded
}

type SCMDeployment struct {
	ID           uuid.UUID
	RepositoryID uuid.UUID
	WorkspaceID  uuid.UUID
	ExternalID   string
	Environment  string
	State        DeploymentState
	URL          string
	CommitSHA    string
	OccurredAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type SCMDeployments []SCMDeployment

func (deployments SCMDeployments) Latest(environment string) (SCMDeployment, bool) {
	var (
		found  SCMDeployment
		picked bool
	)

	for _, deployment := range deployments {
		if !strings.EqualFold(deployment.Environment, environment) {
			continue
		}

		if !picked || occurredAfter(deployment, found) {
			found, picked = deployment, true
		}
	}

	return found, picked
}

func occurredAfter(candidate, held SCMDeployment) bool {
	if candidate.OccurredAt == nil {
		return false
	}

	if held.OccurredAt == nil {
		return true
	}

	return candidate.OccurredAt.After(*held.OccurredAt)
}

func (deployments SCMDeployments) Environments() []string {
	seen := make(map[string]bool, len(deployments))
	found := make([]string, 0, len(deployments))

	for _, deployment := range deployments {
		key := strings.ToLower(strings.TrimSpace(deployment.Environment))
		if key == "" || seen[key] {
			continue
		}

		seen[key] = true
		found = append(found, deployment.Environment)
	}

	return found
}

func MatchReleaseCommits(commits []string, links []CodeLink) []CodeLink {
	shipped := make(map[string]bool, len(commits))

	for _, commit := range commits {
		if trimmed := strings.ToLower(strings.TrimSpace(commit)); trimmed != "" {
			shipped[trimmed] = true
		}
	}

	matched := make([]CodeLink, 0, len(links))

	for _, link := range links {
		if shipped[strings.ToLower(strings.TrimSpace(link.MergeCommitSHA))] {
			matched = append(matched, link)
		}
	}

	return matched
}
