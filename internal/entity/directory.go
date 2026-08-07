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
	DirectoryTokenPrefix   = "nrnscim_"
	DirectoryTokenBytes    = 32
	DirectoryPageMaxSize   = 200
	DirectoryPageSize      = 50
	DirectoryUserNameMax   = 254
	DirectoryGroupNameMax  = 254
	DirectoryExternalIDMax = 254
)

var (
	ErrDirectoryNotConnected     = errors.New("this workspace has no directory connection")
	ErrDirectoryTokenInvalid     = errors.New("directory credential is not recognised")
	ErrDirectoryDisabled         = errors.New("directory synchronization is turned off for this workspace")
	ErrDirectoryUserNotFound     = errors.New("directory user not found")
	ErrDirectoryUserExists       = errors.New("directory already knows a user with that name")
	ErrDirectoryGroupNotFound    = errors.New("directory group not found")
	ErrDirectoryGroupExists      = errors.New("directory already knows a group with that name")
	ErrDirectoryUserNameRequired = errors.New("directory user must carry a userName")
	ErrDirectoryPatchUnsupported = errors.New("this patch expression is not supported")
	ErrDirectoryRunNotFound      = errors.New("directory sync run not found")

	ErrDirectoryAccountNotClaimable = errors.New(
		"a Norn account already holds that address and this directory does not own it. " +
			"The person signs in themselves and joins by invitation instead",
	)
)

func DirectoryClaimRefusal(account Account, elsewhere bool) error {
	if account.HasPassword() || elsewhere {
		return ErrDirectoryAccountNotClaimable
	}

	return nil
}

type DirectoryUnknownPolicy string

const (
	DirectoryProvisionUnknown DirectoryUnknownPolicy = "provision"
	DirectoryIgnoreUnknown    DirectoryUnknownPolicy = "ignore"
)

func (p DirectoryUnknownPolicy) Valid() bool {
	return p == DirectoryProvisionUnknown || p == DirectoryIgnoreUnknown
}

type DirectoryAbsentPolicy string

const (
	DirectoryDeactivateAbsent DirectoryAbsentPolicy = "deactivate"
	DirectoryFlagAbsent       DirectoryAbsentPolicy = "flag"
)

func (p DirectoryAbsentPolicy) Valid() bool {
	return p == DirectoryDeactivateAbsent || p == DirectoryFlagAbsent
}

const (
	DefaultDirectoryUnknownPolicy = DirectoryProvisionUnknown
	DefaultDirectoryAbsentPolicy  = DirectoryDeactivateAbsent
)

type DirectoryConnection struct {
	WorkspaceID uuid.UUID
	TokenHash   []byte
	Enabled     bool
	OnUnknown   DirectoryUnknownPolicy
	OnAbsent    DirectoryAbsentPolicy
	AdminGroup  string
	LastSyncAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (c DirectoryConnection) Usable() bool {
	return c.WorkspaceID != uuid.Nil && c.Enabled
}

func (c DirectoryConnection) Governs(group string) bool {
	admin := strings.TrimSpace(c.AdminGroup)

	return admin != "" && strings.EqualFold(admin, strings.TrimSpace(group))
}

func (c DirectoryConnection) RoleFor(groups []string) MembershipRole {
	for _, group := range groups {
		if c.Governs(group) {
			return MembershipRoleAdmin
		}
	}

	return MembershipRoleMember
}

type DirectoryUser struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	AccountID   uuid.UUID
	ExternalID  string
	UserName    string
	DisplayName string
	Active      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type DirectoryGroup struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	ExternalID  string
	DisplayName string
	TeamID      *uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (g DirectoryGroup) Mapped() bool {
	return g.TeamID != nil && *g.TeamID != uuid.Nil
}

func MapGroupToTeam(displayName string, teams []Team) *uuid.UUID {
	name := strings.TrimSpace(displayName)
	if name == "" {
		return nil
	}

	key := NormalizeTeamKey(name)

	for _, team := range teams {
		if team.Key == key {
			id := team.ID

			return &id
		}
	}

	for _, team := range teams {
		if strings.EqualFold(strings.TrimSpace(team.Name), name) {
			id := team.ID

			return &id
		}
	}

	return nil
}

type DirectorySyncTrigger string

const (
	DirectorySyncByProvider DirectorySyncTrigger = "scim"
	DirectorySyncByAdmin    DirectorySyncTrigger = "admin"
)

type DirectorySyncOutcome string

const (
	DirectorySyncSucceeded DirectorySyncOutcome = "succeeded"
	DirectorySyncRefused   DirectorySyncOutcome = "refused"
	DirectorySyncFailed    DirectorySyncOutcome = "failed"
)

type DirectoryChangeKind string

const (
	DirectoryProvisioned  DirectoryChangeKind = "provisioned"
	DirectoryAdopted      DirectoryChangeKind = "adopted"
	DirectoryUpdated      DirectoryChangeKind = "updated"
	DirectoryDeactivated  DirectoryChangeKind = "deactivated"
	DirectoryReactivated  DirectoryChangeKind = "reactivated"
	DirectoryRoleChanged  DirectoryChangeKind = "role_changed"
	DirectoryTeamJoined   DirectoryChangeKind = "team_joined"
	DirectoryTeamLeft     DirectoryChangeKind = "team_left"
	DirectoryGroupSkipped DirectoryChangeKind = "group_unmapped"
	DirectoryRefused      DirectoryChangeKind = "refused"
)

type DirectorySyncRun struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	Trigger     DirectorySyncTrigger
	Outcome     DirectorySyncOutcome
	Operation   string
	StartedAt   time.Time
	FinishedAt  time.Time
	Detail      string
	Changes     []DirectorySyncChange
}

type DirectorySyncChange struct {
	ID         uuid.UUID
	RunID      uuid.UUID
	Subject    string
	Kind       DirectoryChangeKind
	Outcome    DirectorySyncOutcome
	Detail     map[string]string
	RecordedAt time.Time
}

type DirectoryRunPage struct {
	Limit  int
	Cursor *time.Time
}

func (p DirectoryRunPage) Lookahead() DirectoryRunPage {
	p.Limit++

	return p
}

func (p DirectoryRunPage) Normalized() DirectoryRunPage {
	if p.Limit <= 0 {
		p.Limit = DirectoryPageSize
	}

	if p.Limit > DirectoryPageMaxSize {
		p.Limit = DirectoryPageMaxSize
	}

	return p
}

const DirectoryWorkloadMax = 50

type DirectoryWorkload struct {
	Issues   []string
	Projects []string
	Teams    []string
}

func (w DirectoryWorkload) Empty() bool {
	return len(w.Issues) == 0 && len(w.Projects) == 0 && len(w.Teams) == 0
}

func AssignedToFilter(accountID uuid.UUID) IssueFilter {
	return IssueFilter{
		Field:  IssueFilterFieldAssignee,
		Op:     IssueFilterOpIs,
		Values: []string{accountID.String()},
	}
}

func NewDirectoryToken() (string, []byte, error) {
	raw := make([]byte, DirectoryTokenBytes)

	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate directory token: %w", err)
	}

	token := DirectoryTokenPrefix + base64.RawURLEncoding.EncodeToString(raw)
	hashed := HashDirectoryToken(token)

	return token, hashed, nil
}

func HashDirectoryToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))

	return sum[:]
}

func LooksLikeDirectoryToken(token string) bool {
	return strings.HasPrefix(token, DirectoryTokenPrefix)
}

func NormalizeDirectoryUserName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
