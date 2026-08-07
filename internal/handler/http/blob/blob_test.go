package blob_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/handler/http/blob"
	blobrepo "github.com/usenorn/norn/internal/repository/blob"
	grantrepo "github.com/usenorn/norn/internal/repository/blobgrant"
)

type edge struct {
	blobs  *blobrepo.MockBlob
	grants *grantrepo.MockBlobGrant
	router chi.Router
}

func newEdge(t *testing.T, maxFileBytes int64) *edge {
	t.Helper()

	ctrl := gomock.NewController(t)

	e := &edge{
		blobs:  blobrepo.NewMockBlob(ctrl),
		grants: grantrepo.NewMockBlobGrant(ctrl),
		router: chi.NewRouter(),
	}

	handler := blob.New(e.blobs, e.grants, config.Attachments{
		MaxFileBytes:    maxFileBytes,
		TransferTimeout: time.Minute,
	})

	e.router.Put(blob.UploadPath, handler.Receive)

	return e
}

func (e *edge) upload(token string, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPut, "/v1/blobs/upload/"+token, strings.NewReader(body))
	recorder := httptest.NewRecorder()

	e.router.ServeHTTP(recorder, request)

	return recorder
}

func live(maxBytes int64) entity.BlobGrant {
	return entity.BlobGrant{
		Purpose:   entity.BlobGrantUpload,
		Key:       "workspaces/one/attachments/file",
		MaxBytes:  maxBytes,
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
}

func TestAnUploadLinkCannotBeUsedTwice(t *testing.T) {
	e := newEdge(t, 1<<20)

	e.grants.EXPECT().Read(gomock.Any(), "the-grant").Return(live(0), nil)
	e.blobs.EXPECT().
		Put(gomock.Any(), "workspaces/one/attachments/file", gomock.Any(), gomock.Any(), int64(-1)).
		DoAndReturn(func(_ context.Context, _, _ string, body io.Reader, _ int64) error {
			_, err := io.ReadAll(body)

			return err
		})

	var revoked string

	e.grants.EXPECT().
		Revoke(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, token string) error {
			revoked = token

			return nil
		})

	if status := e.upload("the-grant", "hello").Code; status != http.StatusNoContent {
		t.Fatalf("uploading answered %d, want %d", status, http.StatusNoContent)
	}

	if revoked != "the-grant" {
		t.Fatal(
			"the upload link outlived the upload it authorised. The link is the whole credential " +
				"and it travels in a URL, so anyone who later reads it could overwrite the file.",
		)
	}
}

func TestAnExpiredUploadLinkIsRefusedEvenIfTheStoreStillHasIt(t *testing.T) {
	e := newEdge(t, 1<<20)

	stale := live(0)
	stale.ExpiresAt = time.Now().UTC().Add(-time.Second)

	e.grants.EXPECT().Read(gomock.Any(), "the-grant").Return(stale, nil)

	if status := e.upload("the-grant", "hello").Code; status != http.StatusGone {
		t.Fatalf(
			"an expired link answered %d, want %d. The stored expiry has to be checked here "+
				"rather than trusted to the store's own eviction.",
			status, http.StatusGone,
		)
	}
}

func TestADownloadLinkCannotBeUsedToUpload(t *testing.T) {
	e := newEdge(t, 1<<20)

	reading := live(0)
	reading.Purpose = entity.BlobGrantDownload

	e.grants.EXPECT().Read(gomock.Any(), "the-grant").Return(reading, nil)

	if status := e.upload("the-grant", "hello").Code; status != http.StatusGone {
		t.Fatalf("a download link was accepted for an upload with %d", status)
	}
}

func TestAnUploadIsCappedByItsOwnLinkNotJustTheGlobalLimit(t *testing.T) {
	e := newEdge(t, 1<<20)

	e.grants.EXPECT().Read(gomock.Any(), "the-grant").Return(live(4), nil)
	e.blobs.EXPECT().
		Put(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ string, body io.Reader, _ int64) error {
			_, err := io.ReadAll(body)

			return err
		})

	if status := e.upload("the-grant", "far too much").Code; status != http.StatusRequestEntityTooLarge {
		t.Fatalf(
			"a link capped at 4 bytes accepted a larger body with %d. The cap travels with the "+
				"link so it cannot drift from the one the reservation was admitted against.",
			status,
		)
	}
}
