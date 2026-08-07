package entity

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

const SCMDefaultPollInterval = 5 * time.Minute

type SCMRepository struct {
	ID               uuid.UUID
	ConnectionID     uuid.UUID
	WorkspaceID      uuid.UUID
	Provider         SCMProvider
	FullName         string
	ExternalID       string
	DefaultBranch    string
	URL              string
	WebhookSecretSet bool
	ExternalHookID   string
	MirrorLabel      string
	PollInterval     time.Duration
	ReconcileCursor  string
	ReconciledAt     *time.Time
	ReconcileAfter   *time.Time
	LastSeenAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (r SCMRepository) Parked(now time.Time) bool {
	return r.ReconcileAfter != nil && now.Before(*r.ReconcileAfter)
}

// Due reports that the sweep should read this repository again. A repository whose webhook
// never arrives is the case the sweep exists for, so the interval is per repository rather
// than one number for the whole workspace.
func (r SCMRepository) Due(now time.Time) bool {
	if r.Parked(now) {
		return false
	}

	if r.ReconciledAt == nil {
		return true
	}

	interval := r.PollInterval
	if interval <= 0 {
		interval = SCMDefaultPollInterval
	}

	return !now.Before(r.ReconciledAt.Add(interval))
}

func (r SCMRepository) HookInstalled() bool {
	return r.ExternalHookID != ""
}

// SCMRoute sends the changes under one path to one team. A repository holding several
// products cannot be owned by a single team, and a route whose prefix is empty is the
// repository's default rather than a separate column that could disagree with it.
type SCMRoute struct {
	ID           uuid.UUID
	RepositoryID uuid.UUID
	WorkspaceID  uuid.UUID
	TeamID       uuid.UUID
	PathPrefix   string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (r SCMRoute) Default() bool {
	return r.PathPrefix == ""
}

// Covers reports that a changed file lies under this route. Matching is by path segment, so
// `api` owns `api/main.go` and leaves `apiary/main.go` to whoever owns that.
func (r SCMRoute) Covers(path string) bool {
	if r.PathPrefix == "" {
		return true
	}

	cleaned := NormalizeSCMPathPrefix(path)

	return cleaned == r.PathPrefix || strings.HasPrefix(cleaned, r.PathPrefix+"/")
}

type SCMRoutes []SCMRoute

// Teams reports every team a change reaches. Each path is resolved by longest prefix, so a
// team owning `api` does not receive a change confined to `web` when both are routed. Two
// teams may share a prefix deliberately, and then both receive it.
func (routes SCMRoutes) Teams(paths []string) []uuid.UUID {
	if len(paths) == 0 {
		return routes.teamsFor("")
	}

	found := make([]uuid.UUID, 0, len(routes))

	for _, path := range paths {
		for _, team := range routes.teamsFor(path) {
			if !containsID(found, team) {
				found = append(found, team)
			}
		}
	}

	return found
}

func (routes SCMRoutes) teamsFor(path string) []uuid.UUID {
	longest := -1

	for _, route := range routes {
		if route.Covers(path) && len(route.PathPrefix) > longest {
			longest = len(route.PathPrefix)
		}
	}

	if longest < 0 {
		return nil
	}

	teams := make([]uuid.UUID, 0, 2)

	for _, route := range routes {
		if route.Covers(path) && len(route.PathPrefix) == longest && !containsID(teams, route.TeamID) {
			teams = append(teams, route.TeamID)
		}
	}

	return teams
}

// Reaches answers whether this repository may act on an issue owned by the given team. It
// is the bound on what a connection can touch, so an unrouted team is out of reach however
// the issue was named.
func (routes SCMRoutes) Reaches(teamID uuid.UUID) bool {
	for _, route := range routes {
		if route.TeamID == teamID {
			return true
		}
	}

	return false
}

func containsID(values []uuid.UUID, wanted uuid.UUID) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}

	return false
}

func NormalizeSCMPathPrefix(prefix string) string {
	return strings.Trim(strings.TrimSpace(prefix), "/")
}

func ValidateSCMPathPrefix(field, prefix string) FieldError {
	trimmed := NormalizeSCMPathPrefix(prefix)

	switch {
	case len(trimmed) > SCMPathPrefixMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	case strings.Contains(trimmed, ".."):
		return FieldError{Field: field, Code: ValidationCodeUnsupportedValue}
	default:
		return FieldError{}
	}
}
