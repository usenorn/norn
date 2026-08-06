package entity

import (
	"time"

	"github.com/google/uuid"
)

// ImportOrigin carries a timestamp and an author that came from somewhere other than
// this instance. The attributed flag is unexported and set only by NewImportOrigin, so
// a value decoded from a request body is inert however its exported fields are filled:
// encoding/json cannot reach the flag, and every repository ignores an origin without it.
type ImportOrigin struct {
	CreatedAt       time.Time
	UpdatedAt       time.Time
	AuthorAccountID uuid.UUID

	attributed bool
}

func NewImportOrigin(createdAt, updatedAt time.Time, author uuid.UUID) ImportOrigin {
	if createdAt.IsZero() {
		return ImportOrigin{}
	}

	if updatedAt.IsZero() || updatedAt.Before(createdAt) {
		updatedAt = createdAt
	}

	return ImportOrigin{
		CreatedAt:       createdAt.UTC(),
		UpdatedAt:       updatedAt.UTC(),
		AuthorAccountID: author,
		attributed:      true,
	}
}

func (o ImportOrigin) Attributed() bool {
	return o.attributed
}

func (o ImportOrigin) Attributes() bool {
	return o.attributed && o.AuthorAccountID != uuid.Nil
}

func (o ImportOrigin) Stamp(fallback time.Time) (time.Time, time.Time) {
	if !o.attributed {
		return fallback, fallback
	}

	return o.CreatedAt, o.UpdatedAt
}

// OriginAttributed is the nil-safe form of Attributed, and it is what every concession
// granted to an import must test. A pointer merely being non-nil is not enough: a value
// decoded from a request body is non-nil and inert, so a nil check would hand the
// concession to any caller who named the field.
func OriginAttributed(origin *ImportOrigin) bool {
	return origin != nil && origin.attributed
}

func OriginStamp(origin *ImportOrigin, fallback time.Time) (time.Time, time.Time) {
	if origin == nil {
		return fallback, fallback
	}

	return origin.Stamp(fallback)
}

func OriginAuthor(origin *ImportOrigin, fallback uuid.UUID) uuid.UUID {
	if origin == nil || !origin.Attributes() {
		return fallback
	}

	return origin.AuthorAccountID
}
