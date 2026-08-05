package entity

import (
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	MembershipPageDefaultSize = 50
	MembershipPageMaxSize     = 200
	MembershipCursorIDLen     = 36
)

var (
	ErrMembershipNotFound         = errors.New("workspace membership not found")
	ErrMembershipExists           = errors.New("workspace membership already exists")
	ErrMembershipSelfRoleChange   = errors.New("workspace membership role cannot be changed by its own account")
	ErrMembershipDirectoryManaged = errors.New("workspace membership is managed by directory sync")
	ErrMembershipDeactivated      = errors.New("workspace membership has been deactivated by directory sync")
	ErrMembershipCursorInvalid    = errors.New("workspace membership cursor is invalid")
)

type MembershipRole string

const (
	MembershipRoleAdmin  MembershipRole = "admin"
	MembershipRoleMember MembershipRole = "member"
	MembershipRoleViewer MembershipRole = "viewer"
)

func (r MembershipRole) Valid() bool {
	switch r {
	case MembershipRoleAdmin, MembershipRoleMember, MembershipRoleViewer:
		return true
	default:
		return false
	}
}

type MembershipSource string

const (
	MembershipSourceManual    MembershipSource = "manual"
	MembershipSourceDirectory MembershipSource = "directory"
)

const DefaultMembershipSource = MembershipSourceManual

func (s MembershipSource) Valid() bool {
	switch s {
	case MembershipSourceManual, MembershipSourceDirectory:
		return true
	default:
		return false
	}
}

func (s MembershipSource) Managed() bool {
	return s == MembershipSourceDirectory
}

type Membership struct {
	ID             uuid.UUID
	WorkspaceID    uuid.UUID
	AccountID      uuid.UUID
	Role           MembershipRole
	Source         MembershipSource
	LastActiveAt   *time.Time
	LastAuthMethod SessionAuthMethod
	ReadsAudit     bool
	DeactivatedAt  *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (m Membership) Deactivated() bool {
	return m.DeactivatedAt != nil
}

type WorkspaceMember struct {
	AccountKind AccountKind
	Membership  Membership
	DisplayName string
	Email       string
	SortName    string
}

func (m WorkspaceMember) Cursor() MembershipCursor {
	return MembershipCursor{Name: m.SortName, AccountID: m.Membership.AccountID}
}

type MembershipCursor struct {
	Name      string
	AccountID uuid.UUID
}

func (c MembershipCursor) Encode() string {
	return base64.RawURLEncoding.EncodeToString([]byte(c.AccountID.String() + c.Name))
}

func DecodeMembershipCursor(raw string) (MembershipCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) < MembershipCursorIDLen {
		return MembershipCursor{}, ErrMembershipCursorInvalid
	}

	accountID, err := uuid.Parse(string(decoded[:MembershipCursorIDLen]))
	if err != nil {
		return MembershipCursor{}, ErrMembershipCursorInvalid
	}

	return MembershipCursor{
		Name:      string(decoded[MembershipCursorIDLen:]),
		AccountID: accountID,
	}, nil
}

type MembershipPage struct {
	Query  string
	Cursor *MembershipCursor
	Limit  int
}

func (p MembershipPage) Normalized() MembershipPage {
	p.Query = strings.TrimSpace(p.Query)

	if p.Limit <= 0 {
		p.Limit = MembershipPageDefaultSize
	}

	if p.Limit > MembershipPageMaxSize {
		p.Limit = MembershipPageMaxSize
	}

	return p
}

func (p MembershipPage) Lookahead() MembershipPage {
	p.Limit++

	return p
}
