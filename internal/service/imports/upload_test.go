package imports_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

const uploadedRows = "title,state\nProration is off by one day,Todo\n"

func (h *harness) upload() (service.ImportFile, error) {
	return h.imports.Upload(context.Background(), h.run().WorkspaceID, h.run().ID, service.ImportUpload{
		FileName: "issues.csv",
		Body:     strings.NewReader(uploadedRows),
	})
}

func TestAnImportFileIsChargedToTheWorkspaceBeforeItsBytesAreWritten(t *testing.T) {
	h := newHarness(t).backed().allow(entity.Decision{})

	charged := ""

	h.fileWriter.EXPECT().
		ChargeImportFile(gomock.Any(), h.run().WorkspaceID, gomock.Any(), int64(len(uploadedRows))).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, key string, _ int64) (int64, error) {
			charged = key

			return 0, nil
		})
	h.blobs.EXPECT().
		Put(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), int64(len(uploadedRows))).
		DoAndReturn(func(_ context.Context, key, _ string, _ io.Reader, _ int64) error {
			if charged != key {
				t.Fatalf(
					"the file was written to %q before the workspace was charged for it (charged %q). "+
						"Charging is what refuses a full workspace, so writing first stores bytes "+
						"there is no room for.",
					key, charged,
				)
			}

			return nil
		})

	file, err := h.upload()
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	if file.ObjectKey != charged {
		t.Fatalf("the run was handed %q, but the workspace was charged for %q", file.ObjectKey, charged)
	}
}

func TestAnImportFileIntoAFullWorkspaceIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	h := newHarness(t).backed().allow(entity.Decision{})

	h.fileWriter.EXPECT().
		ChargeImportFile(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(int64(0), entity.StorageExhaustedError{SizeBytes: 50, StoredBytes: 990, MaxBytes: 1_000})

	if _, err := h.upload(); !errors.Is(err, entity.ErrStorageExhausted) {
		t.Fatalf(
			"uploading into a full workspace returned %v. The person uploading has to be told "+
				"the workspace is full, and nothing may reach storage.",
			err,
		)
	}
}

func TestAReplacementTheStoreRefusesPutsTheImportFileBackToItsEarlierSize(t *testing.T) {
	h := newHarness(t).backed().allow(entity.Decision{})

	h.fileWriter.EXPECT().
		ChargeImportFile(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(int64(300), nil)
	h.blobs.EXPECT().
		Put(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("storage is unreachable"))

	var restoredTo int64 = -1

	h.fileWriter.EXPECT().
		RestoreImportFile(gomock.Any(), h.run().WorkspaceID, gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, _ string, previous int64) error {
			restoredTo = previous

			return nil
		})

	if _, err := h.upload(); err == nil {
		t.Fatal("an upload the store refused reported success")
	}

	if restoredTo != 300 {
		t.Fatalf(
			"the refused replacement was put back with %d bytes, want the 300 the earlier upload "+
				"stored. That object is still in storage, so dropping its charge would read the "+
				"workspace as holding less than it does.",
			restoredTo,
		)
	}
}
