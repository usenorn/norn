package entity

import (
	"encoding/base64"
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"
)

const (
	DocRevisionPageDefaultSize = 25
	DocRevisionPageMaxSize     = 100
)

var (
	ErrDocRevisionNotFound      = errors.New("doc revision not found")
	ErrDocRevisionCursorInvalid = errors.New("doc revision cursor is invalid")
)

type DocRevisionAction string

const (
	DocRevisionCreated  DocRevisionAction = "created"
	DocRevisionEdited   DocRevisionAction = "edited"
	DocRevisionMoved    DocRevisionAction = "moved"
	DocRevisionReshared DocRevisionAction = "reshared"
	DocRevisionRestored DocRevisionAction = "restored"
	DocRevisionArchived DocRevisionAction = "archived"
)

func DocRevisionActions() []DocRevisionAction {
	return []DocRevisionAction{
		DocRevisionCreated,
		DocRevisionEdited,
		DocRevisionMoved,
		DocRevisionReshared,
		DocRevisionRestored,
		DocRevisionArchived,
	}
}

func (a DocRevisionAction) Valid() bool {
	return slices.Contains(DocRevisionActions(), a)
}

type DocAttribution struct {
	AccountID    uuid.UUID
	AgentID      *uuid.UUID
	ConnectionID *uuid.UUID
	Source       RevisionSource
}

func DocAttributionOf(decision KnowledgeDecision) DocAttribution {
	actor := decision.Decision.Actor

	return DocAttribution{
		AccountID:    decision.Reader.Identity.Principal,
		AgentID:      actor.AgentID,
		ConnectionID: actor.ConnectionID,
		Source:       RevisionSourceOf(actor.Kind, "", nil),
	}
}

type DocRevisionSummary struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	DocID       uuid.UUID
	Version     int
	Action      DocRevisionAction
	Title       string
	ParentID    *uuid.UUID
	Audience    KnowledgeAudience
	By          DocAttribution
	AccountName string
	CreatedAt   time.Time
}

type DocRevision struct {
	DocRevisionSummary
	Content  Document
	Markdown string
}

func (d Doc) Revision(action DocRevisionAction, by DocAttribution) DocRevision {
	if action == DocRevisionRestored {
		by.Source = RevisionSourceRestore
	}

	return DocRevision{
		DocRevisionSummary: DocRevisionSummary{
			ID:          uuid.New(),
			WorkspaceID: d.WorkspaceID,
			DocID:       d.ID,
			Version:     d.Version,
			Action:      action,
			Title:       d.Title,
			ParentID:    d.ParentID,
			Audience:    d.Audience,
			By:          by,
			CreatedAt:   d.UpdatedAt,
		},
		Content:  d.Content,
		Markdown: d.Markdown,
	}
}

func (r DocRevisionSummary) Cursor() DocRevisionCursor {
	return DocRevisionCursor{Version: r.Version}
}

type DocRevisionCursor struct {
	Version int
}

func (c DocRevisionCursor) Encode() string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(c.Version)))
}

func DecodeDocRevisionCursor(raw string) (DocRevisionCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return DocRevisionCursor{}, ErrDocRevisionCursorInvalid
	}

	version, err := strconv.Atoi(string(decoded))
	if err != nil || version < 1 {
		return DocRevisionCursor{}, ErrDocRevisionCursorInvalid
	}

	return DocRevisionCursor{Version: version}, nil
}

type DocRevisionPage struct {
	Cursor *DocRevisionCursor
	Limit  int
}

func (p DocRevisionPage) Normalized() DocRevisionPage {
	if p.Limit <= 0 {
		p.Limit = DocRevisionPageDefaultSize
	}

	if p.Limit > DocRevisionPageMaxSize {
		p.Limit = DocRevisionPageMaxSize
	}

	return p
}

func (p DocRevisionPage) Lookahead() DocRevisionPage {
	p.Limit++

	return p
}
