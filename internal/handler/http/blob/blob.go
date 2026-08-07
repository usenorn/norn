package blob

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/middleware"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/repository"
)

const (
	BasePath     = "/v1/blobs"
	UploadPath   = BasePath + "/upload/{grant}"
	DownloadPath = BasePath + "/download/{grant}"

	sandbox = "default-src 'none'; sandbox; frame-ancestors 'none'"
)

type Edge struct {
	blobs  repository.Blob
	grants repository.BlobGrant
	cfg    config.Attachments
}

func New(blobs repository.Blob, grants repository.BlobGrant, cfg config.Attachments) *Edge {
	return &Edge{blobs: blobs, grants: grants, cfg: cfg}
}

func (e *Edge) grant(
	w http.ResponseWriter,
	r *http.Request,
	purpose entity.BlobGrantPurpose,
) (entity.BlobGrant, string, bool) {
	token := chi.URLParam(r, "grant")

	grant, err := e.grants.Read(r.Context(), token)
	if err != nil {
		if errors.Is(err, entity.ErrBlobGrantNotFound) {
			middleware.WriteProblem(w, r, http.StatusGone, entity.ErrBlobGrantNotFound.Error())

			return entity.BlobGrant{}, "", false
		}

		middleware.WriteProblem(w, r, http.StatusInternalServerError, "")

		return entity.BlobGrant{}, "", false
	}

	if grant.Spent(purpose, time.Now().UTC()) {
		middleware.WriteProblem(w, r, http.StatusGone, entity.ErrBlobGrantNotFound.Error())

		return entity.BlobGrant{}, "", false
	}

	return grant, token, true
}

func (e *Edge) Receive(w http.ResponseWriter, r *http.Request) {
	grant, token, ok := e.grant(w, r, entity.BlobGrantUpload)
	if !ok {
		return
	}

	body := http.MaxBytesReader(w, r.Body, e.uploadLimit(grant))
	defer func() { _ = body.Close() }()

	if err := e.blobs.Put(r.Context(), grant.Key, entity.AttachmentGenericType, body, -1); err != nil {
		var oversized *http.MaxBytesError
		if errors.As(err, &oversized) {
			middleware.WriteProblem(w, r, http.StatusRequestEntityTooLarge, entity.ErrAttachmentTooLarge.Error())

			return
		}

		logging.From(r.Context()).ErrorContext(r.Context(), "receiving a file failed", "error", err.Error())
		middleware.WriteProblem(w, r, http.StatusInternalServerError, "")

		return
	}

	if err := e.grants.Revoke(r.Context(), token); err != nil {
		logging.From(r.Context()).ErrorContext(r.Context(), "spending an upload link failed", "error", err.Error())
	}

	w.WriteHeader(http.StatusNoContent)
}

func (e *Edge) uploadLimit(grant entity.BlobGrant) int64 {
	if grant.MaxBytes > 0 && grant.MaxBytes < e.cfg.MaxFileBytes {
		return grant.MaxBytes
	}

	return e.cfg.MaxFileBytes
}

func (e *Edge) Serve(w http.ResponseWriter, r *http.Request) {
	grant, _, ok := e.grant(w, r, entity.BlobGrantDownload)
	if !ok {
		return
	}

	object, err := e.blobs.Stat(r.Context(), grant.Key)
	if err != nil {
		middleware.WriteProblem(w, r, http.StatusNotFound, entity.ErrBlobNotFound.Error())

		return
	}

	file, err := e.blobs.Open(r.Context(), grant.Key)
	if err != nil {
		middleware.WriteProblem(w, r, http.StatusNotFound, entity.ErrBlobNotFound.Error())

		return
	}

	defer func() { _ = file.Close() }()

	header := w.Header()
	header.Set("Content-Type", grant.Serve.ContentType)
	header.Set("Content-Disposition", entity.ContentDisposition(grant.Serve))
	header.Set("Content-Length", strconv.FormatInt(object.Size, 10))
	header.Set("Content-Security-Policy", sandbox)
	header.Set("Cross-Origin-Resource-Policy", "same-origin")
	header.Set("X-Frame-Options", "DENY")
	header.Set("Cache-Control", "private, no-store")
	header.Set("Accept-Ranges", "bytes")

	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(e.cfg.TransferTimeout))

	http.ServeContent(w, r, "", object.ModifiedAt, file)
}
