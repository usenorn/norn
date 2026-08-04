package dashboard

import (
	"context"

	api "github.com/usenorn/norn/pkg/http/v1/dashboard"

	"github.com/usenorn/norn/internal/service"
)

func (h *handler) ListWorkspaceIssueAttachments(
	ctx context.Context,
	request api.ListWorkspaceIssueAttachmentsRequestObject,
) (api.ListWorkspaceIssueAttachmentsResponseObject, error) {
	attachments, err := h.attachments.List(ctx, request.WorkspaceId, request.IssueId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceIssueAttachments200JSONResponse{
		Attachments: attachmentDTOs(attachments),
	}, nil
}

func (h *handler) ReserveWorkspaceIssueAttachment(
	ctx context.Context,
	request api.ReserveWorkspaceIssueAttachmentRequestObject,
) (api.ReserveWorkspaceIssueAttachmentResponseObject, error) {
	input := service.ReserveAttachmentInput{
		FileName:  request.Body.FileName,
		SizeBytes: request.Body.ByteSize,
	}

	if request.Body.ContentType != nil {
		input.ContentType = *request.Body.ContentType
	}

	reservation, err := h.attachments.Reserve(ctx, request.WorkspaceId, request.IssueId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ReserveWorkspaceIssueAttachment201JSONResponse(
		attachmentReservationDTO(reservation),
	), nil
}

func (h *handler) FinalizeWorkspaceIssueAttachment(
	ctx context.Context,
	request api.FinalizeWorkspaceIssueAttachmentRequestObject,
) (api.FinalizeWorkspaceIssueAttachmentResponseObject, error) {
	attachment, err := h.attachments.Finalize(
		ctx, request.WorkspaceId, request.IssueId, request.AttachmentId,
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.FinalizeWorkspaceIssueAttachment200JSONResponse(attachmentDTO(attachment)), nil
}

func (h *handler) RemoveWorkspaceIssueAttachment(
	ctx context.Context,
	request api.RemoveWorkspaceIssueAttachmentRequestObject,
) (api.RemoveWorkspaceIssueAttachmentResponseObject, error) {
	err := h.attachments.Remove(ctx, request.WorkspaceId, request.IssueId, request.AttachmentId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RemoveWorkspaceIssueAttachment204Response{}, nil
}

func (h *handler) DownloadWorkspaceAttachment(
	ctx context.Context,
	request api.DownloadWorkspaceAttachmentRequestObject,
) (api.DownloadWorkspaceAttachmentResponseObject, error) {
	target, err := h.attachments.Content(ctx, request.WorkspaceId, request.AttachmentId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DownloadWorkspaceAttachment303Response{
		Headers: api.DownloadWorkspaceAttachment303ResponseHeaders{
			Location:     &target,
			CacheControl: &noStore,
		},
	}, nil
}

func (h *handler) DownloadAccountAvatar(
	ctx context.Context,
	request api.DownloadAccountAvatarRequestObject,
) (api.DownloadAccountAvatarResponseObject, error) {
	if _, ok := h.currentAccountID(ctx); !ok {
		return unauthorized(), nil
	}

	target, err := h.accounts.AvatarContent(ctx, request.AccountId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DownloadAccountAvatar303Response{
		Headers: api.DownloadAccountAvatar303ResponseHeaders{
			Location:     &target,
			CacheControl: &noStore,
		},
	}, nil
}

func (h *handler) GetWorkspaceStorage(
	ctx context.Context,
	request api.GetWorkspaceStorageRequestObject,
) (api.GetWorkspaceStorageResponseObject, error) {
	ledger, err := h.attachments.Ledger(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceStorage200JSONResponse(workspaceStorageDTO(ledger)), nil
}

var noStore = "private, no-store"
