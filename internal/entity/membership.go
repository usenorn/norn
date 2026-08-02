package entity

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrMembershipNotFound = errors.New("workspace membership not found")
	ErrMembershipExists   = errors.New("workspace membership already exists")
)

type MembershipRole string

const (
	MembershipRoleAdmin  MembershipRole = "admin"
	MembershipRoleMember MembershipRole = "member"
)

func (r MembershipRole) Valid() bool {
	switch r {
	case MembershipRoleAdmin, MembershipRoleMember:
		return true
	default:
		return false
	}
}

type Membership struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	AccountID   uuid.UUID
	Role        MembershipRole
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
