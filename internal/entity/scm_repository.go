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
	SyncDirection    MirrorDirection
	WebhooksDisabled bool
	PollInterval     time.Duration
	ReconcileCursor  string
	ReconciledAt     *time.Time
	ReconcileAfter   *time.Time
	LastSeenAt       *time.Time
	BackfilledAt     *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time

	// How many routes narrow this repository. Zero is the permissive state, not the broken one:
	// a repository with no routes reaches every team.
	RouteCount int
}

// Routes only ever narrow what a repository reaches, so a repository carrying none reaches every
// team in the workspace.
func (r SCMRepository) NarrowedByRoutes() bool {
	return r.RouteCount > 0
}

func (r SCMRepository) Parked(now time.Time) bool {
	return r.ReconcileAfter != nil && now.Before(*r.ReconcileAfter)
}

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

func (r SCMRepository) Direction() MirrorDirection {
	if !r.SyncDirection.Valid() || r.SyncDirection == "" {
		return MirrorBoth
	}

	return r.SyncDirection
}

func (r SCMRepository) HookInstalled() bool {
	return r.ExternalHookID != ""
}

func (r SCMRepository) PollsOnly() bool {
	return r.WebhooksDisabled
}

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

func (r SCMRoute) Covers(path string) bool {
	if r.PathPrefix == "" {
		return true
	}

	cleaned := NormalizeSCMPathPrefix(path)

	return cleaned == r.PathPrefix || strings.HasPrefix(cleaned, r.PathPrefix+"/")
}

type SCMRouting struct {
	EveryTeam bool
	Teams     []uuid.UUID
}

func (r SCMRouting) Covers(teamID uuid.UUID) bool {
	return r.EveryTeam || containsID(r.Teams, teamID)
}

func (r SCMRouting) Single() (uuid.UUID, bool) {
	if r.EveryTeam || len(r.Teams) != 1 {
		return uuid.Nil, false
	}

	return r.Teams[0], true
}

type SCMRoutes []SCMRoute

func (routes SCMRoutes) Route(paths []string) SCMRouting {
	if len(routes) == 0 {
		return SCMRouting{EveryTeam: true}
	}

	return SCMRouting{Teams: routes.Teams(paths)}
}

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

func (routes SCMRoutes) Reaches(teamID uuid.UUID) bool {
	if len(routes) == 0 {
		return true
	}

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
