package entity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	APITokenPrefix     = "nrn_"
	APITokenBytes      = 32
	APITokenNameMaxLen = 80
	APITokenMaxTTL     = 365 * 24 * time.Hour
)

var (
	ErrAPITokenNotFound      = errors.New("api token not found")
	ErrAPITokenNameTaken     = errors.New("api token name already used by this account")
	ErrAPITokenScopeInvalid  = errors.New("api token scope is not recognised")
	ErrAPITokenScopeExceeds  = errors.New("api token scope exceeds what its creator may do")
	ErrAPITokenMintForbidden = errors.New("api tokens may not mint or manage api tokens")
	ErrAPITokenGrantInvalid  = errors.New("api token grant names a workspace or team its creator cannot reach")
	ErrAPITokenGrantMissing  = errors.New("api token must grant access to at least one workspace")
)

type APITokenGrant struct {
	WorkspaceID uuid.UUID
	AllTeams    bool
	TeamIDs     []uuid.UUID
}

type RequestedAuthority struct {
	AllTeams bool
	TeamIDs  []uuid.UUID
	Scopes   APIScopeSet
}

func AuthorityOf(actor Actor, workspaceID uuid.UUID) RequestedAuthority {
	authority := RequestedAuthority{
		AllTeams: true,
		TeamIDs:  []uuid.UUID{},
		Scopes:   actor.Scopes,
	}

	if grant, ok := actor.Grants.For(workspaceID); ok {
		authority.AllTeams = grant.AllTeams
		authority.TeamIDs = grant.TeamIDs
	}

	return authority
}

func NewRequestedAuthority(allTeams bool, teamIDs, scopeNames []string) RequestedAuthority {
	authority := RequestedAuthority{AllTeams: allTeams, TeamIDs: make([]uuid.UUID, 0, len(teamIDs))}

	for _, raw := range teamIDs {
		if parsed, err := uuid.Parse(raw); err == nil {
			authority.TeamIDs = append(authority.TeamIDs, parsed)
		}
	}

	if scopeNames != nil {
		authority.Scopes = NewAPIScopeSet(scopeNames)
	}

	return authority
}

func (a RequestedAuthority) TeamStrings() []string {
	values := make([]string, 0, len(a.TeamIDs))

	for _, teamID := range a.TeamIDs {
		values = append(values, teamID.String())
	}

	return values
}

func (a RequestedAuthority) ScopeStrings() []string {
	if a.Scopes == nil {
		return nil
	}

	return a.Scopes.Strings()
}

func (a RequestedAuthority) Replay(
	kind ActorKind,
	accountID uuid.UUID,
	method SessionAuthMethod,
	workspaceID uuid.UUID,
) Actor {
	if kind == "" {
		kind = ActorKindUser
	}

	actor := Actor{
		Kind:       kind,
		AccountID:  accountID,
		AuthMethod: method,
		Scopes:     a.Scopes,
		Grants: APITokenGrants{{
			WorkspaceID: workspaceID,
			AllTeams:    a.AllTeams,
			TeamIDs:     a.TeamIDs,
		}},
	}

	if method == SessionAuthMethodSSO {
		actor.SSOWorkspaceID = workspaceID
	}

	return actor
}

type APITokenGrants []APITokenGrant

func (g APITokenGrants) Covers(workspaceID uuid.UUID) bool {
	_, ok := g.For(workspaceID)

	return ok
}

func (g APITokenGrants) For(workspaceID uuid.UUID) (APITokenGrant, bool) {
	for _, grant := range g {
		if grant.WorkspaceID == workspaceID {
			return grant, true
		}
	}

	return APITokenGrant{}, false
}

func (g APITokenGrants) WorkspaceIDs() []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(g))

	for _, grant := range g {
		ids = append(ids, grant.WorkspaceID)
	}

	return ids
}

type APIToken struct {
	ID               uuid.UUID
	AccountID        uuid.UUID
	Name             string
	TokenHash        []byte
	Scopes           APIScopeSet
	Grants           APITokenGrants
	ExpiresAt        *time.Time
	RevokedAt        *time.Time
	LastUsedAt       *time.Time
	ExpiryNoticeDays *int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (t APIToken) Revoked() bool {
	return t.RevokedAt != nil
}

func (t APIToken) ExpiredAt(now time.Time) bool {
	return t.ExpiresAt != nil && !now.Before(*t.ExpiresAt)
}

func (t APIToken) Usable(now time.Time) bool {
	return !t.Revoked() && !t.ExpiredAt(now)
}

func (t APIToken) NeedsUsageStamp(now time.Time, interval time.Duration) bool {
	return t.LastUsedAt == nil || now.Sub(*t.LastUsedAt) >= interval
}

func NewAPIToken() (string, []byte, error) {
	raw := make([]byte, APITokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate api token: %w", err)
	}

	token := APITokenPrefix + base64.RawURLEncoding.EncodeToString(raw)

	return token, HashAPIToken(token), nil
}

func HashAPIToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))

	return sum[:]
}

func LooksLikeAPIToken(token string) bool {
	return strings.HasPrefix(token, APITokenPrefix)
}

func ValidateAPITokenName(field, name string) FieldError {
	trimmed := strings.TrimSpace(name)

	switch {
	case trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case len(trimmed) > APITokenNameMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}
