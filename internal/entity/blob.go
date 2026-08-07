package entity

import (
	"errors"
	"mime"
	"time"
)

var ErrBlobNotFound = errors.New("stored object not found")

type BlobObject struct {
	Size        int64
	ContentType string
	ModifiedAt  time.Time
}

type BlobTicket struct {
	URL       string
	Method    string
	Headers   map[string]string
	ExpiresAt time.Time
}

type ServeSpec struct {
	ContentType string
	Disposition string
	FileName    string
}

type BlobGrantPurpose string

const (
	BlobGrantUpload   BlobGrantPurpose = "upload"
	BlobGrantDownload BlobGrantPurpose = "download"
)

var ErrBlobGrantNotFound = errors.New("this link has expired")

type BlobGrant struct {
	Purpose   BlobGrantPurpose
	Key       string
	MaxBytes  int64
	Serve     ServeSpec
	ExpiresAt time.Time
}

func (g BlobGrant) Spent(purpose BlobGrantPurpose, now time.Time) bool {
	return g.Purpose != purpose || !g.ExpiresAt.After(now)
}

func ContentDisposition(serve ServeSpec) string {
	disposition := serve.Disposition
	if disposition == "" {
		disposition = AttachmentDispositionAt
	}

	if serve.FileName == "" {
		return disposition
	}

	return mime.FormatMediaType(disposition, map[string]string{"filename": serve.FileName})
}
