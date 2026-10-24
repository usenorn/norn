package entity

import (
	"slices"

	"github.com/google/uuid"
)

type MemoryEvidenceKind string

const (
	MemoryEvidenceIssue       MemoryEvidenceKind = "issue"
	MemoryEvidenceComment     MemoryEvidenceKind = "comment"
	MemoryEvidenceDocRevision MemoryEvidenceKind = "doc_revision"
	MemoryEvidenceExcerpt     MemoryEvidenceKind = "excerpt"
)

func MemoryEvidenceKinds() []MemoryEvidenceKind {
	return []MemoryEvidenceKind{
		MemoryEvidenceIssue,
		MemoryEvidenceComment,
		MemoryEvidenceDocRevision,
		MemoryEvidenceExcerpt,
	}
}

func (k MemoryEvidenceKind) Valid() bool {
	return slices.Contains(MemoryEvidenceKinds(), k)
}

type MemoryEvidence struct {
	Kind          MemoryEvidenceKind
	IssueID       *uuid.UUID
	CommentID     *uuid.UUID
	DocID         *uuid.UUID
	DocRevisionID *uuid.UUID
	Excerpt       string
	DocAudience   KnowledgeAudience
	DocAuthorID   uuid.UUID
}

func (e MemoryEvidence) Validate(field string) FieldError {
	cites := func(issue, comment, doc bool) bool {
		return (e.IssueID != nil) == issue &&
			(e.CommentID != nil) == comment &&
			(e.DocID != nil) == doc && (e.DocRevisionID != nil) == doc
	}

	switch e.Kind {
	case MemoryEvidenceIssue:
		if !cites(true, false, false) || e.Excerpt != "" {
			return FieldError{Field: field, Code: ValidationCodeMalformed}
		}
	case MemoryEvidenceComment:
		if e.CommentID == nil || e.DocID != nil || e.DocRevisionID != nil || e.Excerpt != "" {
			return FieldError{Field: field, Code: ValidationCodeMalformed}
		}
	case MemoryEvidenceDocRevision:
		if !cites(false, false, true) || e.Excerpt != "" {
			return FieldError{Field: field, Code: ValidationCodeMalformed}
		}
	case MemoryEvidenceExcerpt:
		if !cites(false, false, false) {
			return FieldError{Field: field, Code: ValidationCodeMalformed}
		}

		return validateMemoryText(field, e.Excerpt, MemoryExcerptMaxLen, true)
	default:
		return FieldError{Field: field, Code: ValidationCodeUnsupportedValue}
	}

	return FieldError{}
}

func (e MemoryEvidence) DocAccess(workspaceID uuid.UUID) KnowledgeAccess {
	return KnowledgeAccess{WorkspaceID: workspaceID, AuthorID: e.DocAuthorID, Audience: e.DocAudience}
}

func (e MemoryEvidence) SeenBy(reader KnowledgeReader, visibleIssues []uuid.UUID) bool {
	switch e.Kind {
	case MemoryEvidenceIssue, MemoryEvidenceComment:
		return e.IssueID != nil && slices.Contains(visibleIssues, *e.IssueID)
	case MemoryEvidenceDocRevision:
		return reader.Sees(e.DocAccess(reader.WorkspaceID))
	default:
		return true
	}
}

func (e MemoryEvidence) Cites(other MemoryEvidence) bool {
	return e.Kind == other.Kind &&
		sameID(e.IssueID, other.IssueID) &&
		sameID(e.CommentID, other.CommentID) &&
		sameID(e.DocID, other.DocID) &&
		sameID(e.DocRevisionID, other.DocRevisionID) &&
		e.Excerpt == other.Excerpt
}

func sameID(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}
