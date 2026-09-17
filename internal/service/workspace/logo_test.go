package workspace_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color/palette"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
)

func squarePNG(t *testing.T, side int) []byte {
	t.Helper()

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, side, side))); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	return encoded.Bytes()
}

func TestALogoSmallerThanTheMinimumIsRefusedBeforeItIsStored(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()
	actorID := uuid.New()

	h.expectActorMayAct(workspaceID, actorID, entity.ActionUpdate, activeWorkspace(workspaceID))

	_, err := h.service.UploadLogo(actingAs(actorID), workspaceID, bytes.NewReader(squarePNG(t, entity.WorkspaceLogoMinDimension-1)))
	if !errors.Is(err, entity.ErrWorkspaceLogoTooSmall) {
		t.Fatalf("UploadLogo error = %v, want %v", err, entity.ErrWorkspaceLogoTooSmall)
	}
}

func TestALogoInAFormatOutsideTheAllowedSetIsRefused(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()
	actorID := uuid.New()

	var encoded bytes.Buffer
	if err := gif.Encode(&encoded, image.NewPaletted(image.Rect(0, 0, 256, 256), palette.Plan9), nil); err != nil {
		t.Fatalf("encode gif: %v", err)
	}

	h.expectActorMayAct(workspaceID, actorID, entity.ActionUpdate, activeWorkspace(workspaceID))

	_, err := h.service.UploadLogo(actingAs(actorID), workspaceID, &encoded)
	if !errors.Is(err, entity.ErrWorkspaceLogoUnsupportedType) {
		t.Fatalf("UploadLogo error = %v, want %v", err, entity.ErrWorkspaceLogoUnsupportedType)
	}
}

func TestALogoThatOnlyLooksLikeAnImageIsRefused(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()
	actorID := uuid.New()

	whole := squarePNG(t, 256)
	truncated := whole[:len(whole)/2]

	h.expectActorMayAct(workspaceID, actorID, entity.ActionUpdate, activeWorkspace(workspaceID))

	_, err := h.service.UploadLogo(actingAs(actorID), workspaceID, bytes.NewReader(truncated))
	if !errors.Is(err, entity.ErrWorkspaceLogoUnsupportedType) {
		t.Fatalf("UploadLogo error = %v, want %v", err, entity.ErrWorkspaceLogoUnsupportedType)
	}
}

func TestTheLogoTypeComesFromItsContent(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()
	actorID := uuid.New()

	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 200, 200)), nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}

	h.expectActorMayAct(workspaceID, actorID, entity.ActionUpdate, activeWorkspace(workspaceID))

	var storedKey, storedType string

	h.blobs.EXPECT().
		Put(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, key, contentType string, _ io.Reader, _ int64) error {
			storedKey, storedType = key, contentType

			return nil
		})
	h.workspaces.EXPECT().LockByIDs(gomock.Any(), gomock.Any()).Return(nil)
	h.workspaces.EXPECT().GetByID(gomock.Any(), workspaceID).Return(activeWorkspace(workspaceID), nil)
	h.workspaces.EXPECT().SetLogo(gomock.Any(), workspaceID, gomock.Any()).Return(activeWorkspace(workspaceID), nil)

	if _, err := h.service.UploadLogo(actingAs(actorID), workspaceID, &encoded); err != nil {
		t.Fatalf("UploadLogo: %v", err)
	}

	if storedType != "image/jpeg" || !strings.HasSuffix(storedKey, ".jpg") {
		t.Errorf("stored %q as %q, want a .jpg key typed image/jpeg", storedKey, storedType)
	}

	if !strings.HasPrefix(storedKey, entity.WorkspaceLogoPrefix(workspaceID)+"/") {
		t.Errorf("stored key %q outside the workspace logo prefix", storedKey)
	}
}

func TestReplacingALogoDeletesTheOldObjectOnlyAfterTheNewOneIsSaved(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()
	actorID := uuid.New()

	current := activeWorkspace(workspaceID)
	current.LogoObjectKey = entity.WorkspaceLogoPrefix(workspaceID) + "/previous.png"

	h.expectActorMayAct(workspaceID, actorID, entity.ActionUpdate, current)

	var newKey string

	gomock.InOrder(
		h.blobs.EXPECT().
			Put(gomock.Any(), gomock.Any(), "image/png", gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, key, _ string, _ io.Reader, _ int64) error {
				newKey = key

				return nil
			}),
		h.workspaces.EXPECT().LockByIDs(gomock.Any(), []uuid.UUID{workspaceID}).Return(nil),
		h.workspaces.EXPECT().GetByID(gomock.Any(), workspaceID).Return(current, nil),
		h.workspaces.EXPECT().
			SetLogo(gomock.Any(), workspaceID, gomock.Any()).
			DoAndReturn(func(_ context.Context, _ uuid.UUID, key string) (entity.Workspace, error) {
				if key != newKey {
					t.Errorf("saved key %q, want the stored object %q", key, newKey)
				}

				saved := current
				saved.LogoObjectKey = key

				return saved, nil
			}),
		h.blobs.EXPECT().Delete(gomock.Any(), current.LogoObjectKey).Return(nil),
	)

	if _, err := h.service.UploadLogo(actingAs(actorID), workspaceID, bytes.NewReader(squarePNG(t, 256))); err != nil {
		t.Fatalf("UploadLogo: %v", err)
	}
}

func TestAFailedSaveDiscardsOnlyTheNewLogoAndKeepsTheOldOne(t *testing.T) {
	h := newHarness(t)
	workspaceID := uuid.New()
	actorID := uuid.New()

	current := activeWorkspace(workspaceID)
	current.LogoObjectKey = entity.WorkspaceLogoPrefix(workspaceID) + "/previous.png"

	h.expectActorMayAct(workspaceID, actorID, entity.ActionUpdate, current)

	saveFailed := errors.New("database unavailable")

	var newKey string

	gomock.InOrder(
		h.blobs.EXPECT().
			Put(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, key, _ string, _ io.Reader, _ int64) error {
				newKey = key

				return nil
			}),
		h.workspaces.EXPECT().LockByIDs(gomock.Any(), gomock.Any()).Return(nil),
		h.workspaces.EXPECT().GetByID(gomock.Any(), workspaceID).Return(current, nil),
		h.workspaces.EXPECT().SetLogo(gomock.Any(), workspaceID, gomock.Any()).Return(entity.Workspace{}, saveFailed),
		h.blobs.EXPECT().
			Delete(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, key string) error {
				if key != newKey {
					t.Errorf("deleted %q, want only the unsaved object %q", key, newKey)
				}

				return nil
			}),
	)

	_, err := h.service.UploadLogo(actingAs(actorID), workspaceID, bytes.NewReader(squarePNG(t, 256)))
	if !errors.Is(err, saveFailed) {
		t.Fatalf("UploadLogo error = %v, want %v", err, saveFailed)
	}
}
