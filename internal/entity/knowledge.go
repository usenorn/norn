package entity

import (
	"errors"
	"slices"

	"github.com/google/uuid"
)

var (
	ErrKnowledgeNotFound      = errors.New("knowledge not found")
	ErrKnowledgeNotEditable   = errors.New("only the author, a contributor or a maintainer may change this knowledge")
	ErrKnowledgeNotShareable  = errors.New("this knowledge may not be shared with that audience by you")
	ErrKnowledgeAudienceMixed = errors.New("knowledge drawn from separate audiences has no single audience")
)

type KnowledgeVisibility string

const (
	KnowledgeVisibilityPersonal  KnowledgeVisibility = "personal"
	KnowledgeVisibilityProject   KnowledgeVisibility = "project"
	KnowledgeVisibilityWorkspace KnowledgeVisibility = "workspace"
)

func KnowledgeVisibilities() []KnowledgeVisibility {
	return []KnowledgeVisibility{
		KnowledgeVisibilityPersonal,
		KnowledgeVisibilityProject,
		KnowledgeVisibilityWorkspace,
	}
}

func (v KnowledgeVisibility) Valid() bool {
	return slices.Contains(KnowledgeVisibilities(), v)
}

type KnowledgeApplicability struct {
	ProjectID *uuid.UUID
	Component string
}

type KnowledgeAudience struct {
	Visibility KnowledgeVisibility
	ProjectID  uuid.UUID
}

func PersonalAudience() KnowledgeAudience {
	return KnowledgeAudience{Visibility: KnowledgeVisibilityPersonal}
}

func ProjectAudience(projectID uuid.UUID) KnowledgeAudience {
	return KnowledgeAudience{Visibility: KnowledgeVisibilityProject, ProjectID: projectID}
}

func WorkspaceAudience() KnowledgeAudience {
	return KnowledgeAudience{Visibility: KnowledgeVisibilityWorkspace}
}

func (a KnowledgeAudience) Validate() error {
	switch {
	case !a.Visibility.Valid():
		return NewValidationError(FieldError{Field: "visibility", Code: ValidationCodeUnsupportedValue})
	case a.Visibility == KnowledgeVisibilityProject && a.ProjectID == uuid.Nil:
		return NewValidationError(FieldError{Field: "projectId", Code: ValidationCodeRequired})
	case a.Visibility != KnowledgeVisibilityProject && a.ProjectID != uuid.Nil:
		return NewValidationError(FieldError{Field: "projectId", Code: ValidationCodeUnsupportedValue})
	default:
		return nil
	}
}

func (a KnowledgeAudience) Within(other KnowledgeAudience) bool {
	switch a.Visibility {
	case KnowledgeVisibilityPersonal:
		return other.Visibility.Valid()
	case KnowledgeVisibilityProject:
		return other.Visibility == KnowledgeVisibilityWorkspace ||
			(other.Visibility == KnowledgeVisibilityProject && other.ProjectID == a.ProjectID)
	case KnowledgeVisibilityWorkspace:
		return other.Visibility == KnowledgeVisibilityWorkspace
	default:
		return false
	}
}

func DerivedAudience(first KnowledgeAudience, rest ...KnowledgeAudience) (KnowledgeAudience, error) {
	derived := first

	for _, source := range rest {
		switch {
		case derived.Within(source):
		case source.Within(derived):
			derived = source
		default:
			return KnowledgeAudience{}, ErrKnowledgeAudienceMixed
		}
	}

	if err := derived.Validate(); err != nil {
		return KnowledgeAudience{}, err
	}

	return derived, nil
}

type KnowledgeAccess struct {
	WorkspaceID    uuid.UUID
	AuthorID       uuid.UUID
	Audience       KnowledgeAudience
	ContributorIDs []uuid.UUID
	Applicability  KnowledgeApplicability
}

func (a KnowledgeAccess) AuthoredBy(accountID uuid.UUID) bool {
	return a.AuthorID != uuid.Nil && a.AuthorID == accountID
}

func (a KnowledgeAccess) ContributedBy(accountID uuid.UUID) bool {
	return accountID != uuid.Nil && slices.Contains(a.ContributorIDs, accountID)
}

type KnowledgeIdentity struct {
	Principal uuid.UUID
	Personal  bool
	Shared    bool
	Role      MembershipRole
}

func KnowledgeIdentityOf(actor Actor, role MembershipRole) KnowledgeIdentity {
	identity := KnowledgeIdentity{Principal: actor.AccountID, Role: role}

	switch {
	case actor.Anonymous():
		identity.Shared = true
	case actor.Kind == ActorKindAgent:
		if actor.AgentScope == AgentScopeMember && actor.OwnerAccountID != uuid.Nil {
			identity.Principal = actor.OwnerAccountID
			identity.Personal = true
		} else {
			identity.Shared = true
		}
	case actor.OwnerAccountID == uuid.Nil || actor.OwnerAccountID == actor.AccountID:
		identity.Personal = true
	default:
		identity.Shared = true
	}

	return identity
}

func (i KnowledgeIdentity) Maintainer() bool {
	return !i.Shared && i.Role == MembershipRoleAdmin
}

func (i KnowledgeIdentity) Publishes() bool {
	return i.Role.Valid() && i.Role != MembershipRoleViewer
}

func (i KnowledgeIdentity) Widens() bool {
	return !i.Shared && i.Publishes()
}

type KnowledgeAttribution struct {
	AccountID    uuid.UUID
	AgentID      *uuid.UUID
	ConnectionID *uuid.UUID
	Source       RevisionSource
}

func KnowledgeAttributionOf(decision KnowledgeDecision) KnowledgeAttribution {
	actor := decision.Decision.Actor

	return KnowledgeAttribution{
		AccountID:    decision.Reader.Identity.Principal,
		AgentID:      actor.AgentID,
		ConnectionID: actor.ConnectionID,
		Source:       RevisionSourceOf(actor.Kind, "", nil),
	}
}

type KnowledgeDecision struct {
	Decision Decision
	Reader   KnowledgeReader
}

type KnowledgeReader struct {
	WorkspaceID   uuid.UUID
	Identity      KnowledgeIdentity
	AllProjects   bool
	ProjectIDs    []uuid.UUID
	LedProjectIDs []uuid.UUID
}

func (r KnowledgeReader) Reaches(projectID uuid.UUID) bool {
	return projectID != uuid.Nil && (r.AllProjects || slices.Contains(r.ProjectIDs, projectID))
}

func (r KnowledgeReader) Sees(access KnowledgeAccess) bool {
	if r.WorkspaceID == uuid.Nil || access.WorkspaceID != r.WorkspaceID {
		return false
	}

	switch access.Audience.Visibility {
	case KnowledgeVisibilityWorkspace:
		return true
	case KnowledgeVisibilityProject:
		return r.Reaches(access.Audience.ProjectID)
	case KnowledgeVisibilityPersonal:
		return r.Identity.Personal && access.AuthoredBy(r.Identity.Principal)
	default:
		return false
	}
}

func (r KnowledgeReader) Maintains(access KnowledgeAccess) bool {
	if r.Identity.Shared {
		return false
	}

	switch access.Audience.Visibility {
	case KnowledgeVisibilityWorkspace:
		return r.Identity.Maintainer()
	case KnowledgeVisibilityProject:
		return r.Identity.Maintainer() || slices.Contains(r.LedProjectIDs, access.Audience.ProjectID)
	default:
		return false
	}
}

func (r KnowledgeReader) Edits(access KnowledgeAccess) bool {
	if !r.Sees(access) {
		return false
	}

	principal := r.Identity.Principal

	return access.AuthoredBy(principal) || access.ContributedBy(principal) || r.Maintains(access)
}

func (r KnowledgeReader) Reshares(access KnowledgeAccess) bool {
	return r.Edits(access) && (access.AuthoredBy(r.Identity.Principal) || r.Maintains(access))
}

func (r KnowledgeReader) Read(access KnowledgeAccess) error {
	if !r.Sees(access) {
		return ErrKnowledgeNotFound
	}

	return nil
}

func (r KnowledgeReader) Change(access KnowledgeAccess) error {
	if err := r.Read(access); err != nil {
		return err
	}

	if !r.Edits(access) {
		return ErrKnowledgeNotEditable
	}

	return nil
}

func (r KnowledgeReader) Author(audience KnowledgeAudience) error {
	if err := audience.Validate(); err != nil {
		return err
	}

	switch audience.Visibility {
	case KnowledgeVisibilityPersonal:
		if !r.Identity.Personal {
			return ErrKnowledgeNotShareable
		}
	case KnowledgeVisibilityProject:
		if !r.Identity.Publishes() || !r.Reaches(audience.ProjectID) {
			return ErrKnowledgeNotShareable
		}
	case KnowledgeVisibilityWorkspace:
		if !r.Identity.Publishes() {
			return ErrKnowledgeNotShareable
		}
	}

	return nil
}

func (r KnowledgeReader) Reshare(access KnowledgeAccess, to KnowledgeAudience) error {
	if err := r.Change(access); err != nil {
		return err
	}

	if err := to.Validate(); err != nil {
		return err
	}

	if !r.Reshares(access) {
		return ErrKnowledgeNotShareable
	}

	switch to.Visibility {
	case KnowledgeVisibilityPersonal:
		if !r.Identity.Personal || !access.AuthoredBy(r.Identity.Principal) {
			return ErrKnowledgeNotShareable
		}
	case KnowledgeVisibilityProject:
		if !r.Reaches(to.ProjectID) {
			return ErrKnowledgeNotShareable
		}
	}

	if !to.Within(access.Audience) && !r.Identity.Widens() {
		return ErrKnowledgeNotShareable
	}

	return nil
}
