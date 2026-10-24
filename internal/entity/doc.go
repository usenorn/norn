package entity

import (
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	DocTitleMaxLen   = 200
	DocContentMaxLen = 200000
	DocTreeMaxDepth  = 10

	DocPageDefaultSize = 50
	DocPageMaxSize     = 200

	DocSearchDefaultSize = 10
	DocSearchMaxSize     = 25

	docCursorIDLen = 36
)

const (
	DocFieldTitle     = "title"
	DocFieldContent   = "content"
	DocFieldParentID  = "parentId"
	DocFieldProjectID = "projectId"
	DocFieldSection   = "section"
)

var DocLimits = DocumentLimits{MaxDepth: 12, MaxNodes: 50000, MaxTextRunes: DocContentMaxLen}

var (
	ErrDocNotFound      = errors.New("doc not found")
	ErrDocStale         = errors.New("doc changed since it was read")
	ErrDocArchived      = errors.New("doc is archived")
	ErrDocHasChildren   = errors.New("doc still holds docs beneath it")
	ErrDocParentCycle   = errors.New("doc cannot sit beneath itself")
	ErrDocTreeTooDeep   = errors.New("doc tree nests deeper than this instance holds")
	ErrDocCursorInvalid = errors.New("doc cursor is invalid")

	ErrDocSectionNotFound  = errors.New("doc has no section under that heading")
	ErrDocSectionAmbiguous = errors.New("doc has more than one section under that heading")
)

type DocStaleError struct {
	Version int
}

func (e DocStaleError) Error() string {
	return fmt.Sprintf("%s: now at version %d", ErrDocStale, e.Version)
}

func (e DocStaleError) Unwrap() error {
	return ErrDocStale
}

type DocStatus string

const (
	DocStatusActive   DocStatus = "active"
	DocStatusArchived DocStatus = "archived"
)

func DocStatuses() []DocStatus {
	return []DocStatus{DocStatusActive, DocStatusArchived}
}

func (s DocStatus) Valid() bool {
	return slices.Contains(DocStatuses(), s)
}

type DocSummary struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	ParentID    *uuid.UUID
	Title       string
	Audience    KnowledgeAudience
	AuthorID    uuid.UUID
	AuthorName  string
	Status      DocStatus
	ArchivedAt  *time.Time
	Version     int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type DocMatch struct {
	DocSummary
	Excerpt string
}

type DocViewer struct {
	Edits    bool
	Reshares bool
}

type Doc struct {
	DocSummary
	Content        Document
	Markdown       string
	ContributorIDs []uuid.UUID
	Viewer         DocViewer
}

func (d Doc) ViewedBy(reader KnowledgeReader) Doc {
	access := d.Access()
	d.Viewer = DocViewer{Edits: reader.Edits(access), Reshares: reader.Reshares(access)}

	return d
}

func (s DocSummary) Access() KnowledgeAccess {
	access := KnowledgeAccess{
		WorkspaceID: s.WorkspaceID,
		AuthorID:    s.AuthorID,
		Audience:    s.Audience,
	}

	if s.Audience.Visibility == KnowledgeVisibilityProject {
		projectID := s.Audience.ProjectID
		access.Applicability.ProjectID = &projectID
	}

	return access
}

func (d Doc) Access() KnowledgeAccess {
	access := d.DocSummary.Access()
	access.ContributorIDs = d.ContributorIDs

	return access
}

func (s DocSummary) Archived() bool {
	return s.Status == DocStatusArchived
}

func (s DocSummary) Expect(version int) error {
	if s.Version != version {
		return DocStaleError{Version: s.Version}
	}

	if s.Archived() {
		return ErrDocArchived
	}

	return nil
}

func (s DocSummary) Cursor() DocCursor {
	return DocCursor{UpdatedAt: s.UpdatedAt, DocID: s.ID}
}

func ValidateDocPlacement(docID uuid.UUID, parentAncestry []uuid.UUID, subtreeHeight int) error {
	if slices.Contains(parentAncestry, docID) {
		return ErrDocParentCycle
	}

	if len(parentAncestry)+1+subtreeHeight > DocTreeMaxDepth {
		return ErrDocTreeTooDeep
	}

	return nil
}

func ValidateDocTitle(field, title string) FieldError {
	trimmed := strings.TrimSpace(title)

	switch {
	case trimmed == "":
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case strings.ContainsRune(title, 0):
		return FieldError{Field: field, Code: ValidationCodeMalformed}
	case utf8.RuneCountInString(trimmed) > DocTitleMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func ValidateDocContent(field, markdown string) FieldError {
	switch {
	case strings.ContainsRune(markdown, 0):
		return FieldError{Field: field, Code: ValidationCodeMalformed}
	case utf8.RuneCountInString(markdown) > DocContentMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

type DocCursor struct {
	UpdatedAt time.Time
	DocID     uuid.UUID
}

func (c DocCursor) Encode() string {
	return base64.RawURLEncoding.EncodeToString(
		[]byte(c.DocID.String() + c.UpdatedAt.UTC().Format(time.RFC3339Nano)),
	)
}

func DecodeDocCursor(raw string) (DocCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) < docCursorIDLen {
		return DocCursor{}, ErrDocCursorInvalid
	}

	docID, err := uuid.Parse(string(decoded[:docCursorIDLen]))
	if err != nil {
		return DocCursor{}, ErrDocCursorInvalid
	}

	updatedAt, err := time.Parse(time.RFC3339Nano, string(decoded[docCursorIDLen:]))
	if err != nil {
		return DocCursor{}, ErrDocCursorInvalid
	}

	return DocCursor{UpdatedAt: updatedAt, DocID: docID}, nil
}

type DocPage struct {
	Cursor *DocCursor
	Limit  int
}

func (p DocPage) Normalized() DocPage {
	if p.Limit <= 0 {
		p.Limit = DocPageDefaultSize
	}

	if p.Limit > DocPageMaxSize {
		p.Limit = DocPageMaxSize
	}

	return p
}

func (p DocPage) Lookahead() DocPage {
	p.Limit++

	return p
}
