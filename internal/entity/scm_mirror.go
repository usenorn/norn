package entity

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

type MirrorOrigin string

const (
	MirrorOriginPlatform MirrorOrigin = "platform"
	MirrorOriginNorn     MirrorOrigin = "norn"
)

func MirrorOrigins() []MirrorOrigin {
	return []MirrorOrigin{MirrorOriginPlatform, MirrorOriginNorn}
}

func (o MirrorOrigin) Valid() bool {
	return slices.Contains(MirrorOrigins(), o)
}

// MirrorDirection bounds which way a pair syncs. A repository Norn only reads from, and one
// it only writes to, are both ordinary arrangements, and a pair that syncs both ways is the
// one that needs arbitration.
type MirrorDirection string

const (
	MirrorInbound  MirrorDirection = "inbound"
	MirrorOutbound MirrorDirection = "outbound"
	MirrorBoth     MirrorDirection = "both"
)

func MirrorDirections() []MirrorDirection {
	return []MirrorDirection{MirrorInbound, MirrorOutbound, MirrorBoth}
}

func (d MirrorDirection) Valid() bool {
	return slices.Contains(MirrorDirections(), d)
}

func (d MirrorDirection) Pulls() bool {
	return d == MirrorInbound || d == MirrorBoth
}

func (d MirrorDirection) Pushes() bool {
	return d == MirrorOutbound || d == MirrorBoth
}

type IssueMirror struct {
	ID              uuid.UUID
	WorkspaceID     uuid.UUID
	IssueID         uuid.UUID
	RepositoryID    uuid.UUID
	Provider        SCMProvider
	RepositoryName  string
	ExternalID      string
	ExternalNumber  int
	URL             string
	Origin          MirrorOrigin
	Direction       MirrorDirection
	TitleHash       string
	BodyHash        string
	StateHash       string
	SyncedVersion   int
	SourceUpdatedAt *time.Time
	PulledAt        *time.Time
	PushedAt        *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CommentMirror struct {
	ID              uuid.UUID
	WorkspaceID     uuid.UUID
	IssueID         uuid.UUID
	CommentID       uuid.UUID
	MirrorID        uuid.UUID
	Provider        SCMProvider
	RepositoryName  string
	ExternalID      string
	ExternalAuthor  string
	Origin          MirrorOrigin
	BodyHash        string
	SourceUpdatedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// MirrorHash normalises line endings before hashing. A forge returns a body with CRLF that
// Norn stored with LF, so without this every field we push reads back as a foreign edit and
// the two systems overwrite each other for as long as both are running.
func MirrorHash(value string) string {
	normalised := strings.ReplaceAll(strings.TrimSpace(value), "\r\n", "\n")

	sum := sha256.Sum256([]byte(normalised))

	return hex.EncodeToString(sum[:])
}

type MirrorHashes struct {
	Title string
	Body  string
	State string
}

func HashesOf(title, body, state string) MirrorHashes {
	return MirrorHashes{
		Title: MirrorHash(title),
		Body:  MirrorHash(body),
		State: MirrorHash(state),
	}
}

func (m IssueMirror) HashFor(field string) string {
	switch field {
	case IssueFieldTitle:
		return m.TitleHash
	case IssueFieldDescription:
		return m.BodyHash
	case IssueFieldState:
		return m.StateHash
	default:
		return ""
	}
}

func (m IssueMirror) NornChanged(field string, issue Issue) bool {
	if at, stamped := issue.FieldVersions[field]; stamped {
		return at > m.SyncedVersion
	}

	return m.SyncedVersion == 0
}

func (m IssueMirror) SourceChanged(field, value string) bool {
	return m.HashFor(field) != MirrorHash(value)
}

type MirrorWinner string

const (
	MirrorWinnerNeither MirrorWinner = "neither"
	MirrorWinnerNorn    MirrorWinner = "norn"
	MirrorWinnerSource  MirrorWinner = "source"
)

func ArbitrateMirror(nornChanged, sourceChanged bool, nornAt, sourceAt time.Time) MirrorWinner {
	switch {
	case !nornChanged && !sourceChanged:
		return MirrorWinnerNeither
	case nornChanged && !sourceChanged:
		return MirrorWinnerNorn
	case !nornChanged && sourceChanged:
		return MirrorWinnerSource
	case sourceAt.After(nornAt):
		return MirrorWinnerSource
	default:
		return MirrorWinnerNorn
	}
}

func (m IssueMirror) Echoes(field, value string) bool {
	return m.HashFor(field) == MirrorHash(value)
}

func (m CommentMirror) Echoes(body string) bool {
	return m.BodyHash == MirrorHash(body)
}
