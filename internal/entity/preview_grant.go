package entity

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	PreviewGrantAudience = "preview"
	PreviewSessionPath   = "/__norn/session"
	PreviewSharePath     = "/__norn/share/"
	PreviewAuthorizePath = "/v1/previews/authorize"
	PreviewGrantCookie   = "norn_preview"
)

var ErrPreviewGrantNotFound = errors.New("this preview session has expired")

type PreviewVerdict string

const (
	PreviewAllowed PreviewVerdict = "allowed"
	PreviewSignIn  PreviewVerdict = "sign_in"
	PreviewRefused PreviewVerdict = "refused"
)

type PreviewAccess struct {
	Verdict   PreviewVerdict
	Preview   PreviewSession
	RunnerID  uuid.UUID
	Token     string
	Path      string
	ExpiresAt time.Time
	Reason    string
	Redirect  string
}

type PreviewRoute struct {
	Preview  PreviewSession
	RunnerID uuid.UUID
}

type PreviewGrant struct {
	Audience    string
	ExecutionID string
	WorkspaceID uuid.UUID
	PreviewID   uuid.UUID
	Path        string
	AccountID   uuid.UUID
	LinkID      uuid.UUID
	IssuedAt    time.Time
	ExpiresAt   time.Time
}

func NewPreviewGrant(preview PreviewSession, accountID, linkID uuid.UUID, issued, expires time.Time) PreviewGrant {
	return PreviewGrant{
		Audience:    PreviewGrantAudience,
		ExecutionID: preview.ExecutionID,
		WorkspaceID: preview.WorkspaceID,
		PreviewID:   preview.ID,
		Path:        preview.Path,
		AccountID:   accountID,
		LinkID:      linkID,
		IssuedAt:    issued,
		ExpiresAt:   expires,
	}
}

func (g PreviewGrant) Spent(previewID uuid.UUID, now time.Time) bool {
	return g.Audience != PreviewGrantAudience ||
		g.PreviewID != previewID ||
		!g.ExpiresAt.After(now)
}

func (g PreviewGrant) Viewer() string {
	if g.LinkID != uuid.Nil {
		return "link:" + g.LinkID.String()
	}

	return "account:" + g.AccountID.String()
}
