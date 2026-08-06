package linear

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/importrun"
	"github.com/usenorn/norn/internal/service"
)

var ErrFileTooLarge = errors.New("the source holds a file larger than this instance stores")

func (s *Source) fetchAttachments(
	ctx context.Context,
	request service.ImportFetchRequest,
	settings Settings,
) (service.ImportFetchPage, error) {
	run, addressed := importrun.From(ctx)
	if !addressed {
		return service.ImportFetchPage{}, entity.ImportSourceUnavailableError{
			Resource: request.Resource,
			Reason: "this staging pass did not say which run it belongs to, so a file pulled " +
				"from the source would have nowhere to be stored",
		}
	}

	reply, err := query[issuesReply](ctx, s, request, settings, attachmentsQuery)
	if err != nil {
		return service.ImportFetchPage{}, err
	}

	records := make([]entity.ImportRecord, 0, len(reply.Issues.Nodes))

	for _, node := range reply.Issues.Nodes {
		warnTruncated(ctx, "attachments", node.ID, node.Attachments)
		warnTruncated(ctx, "comments", node.ID, node.Comments)

		for _, held := range node.Attachments.Nodes {
			record, err := s.hanging(ctx, request, run, node.ID, held)
			if err != nil {
				return service.ImportFetchPage{}, err
			}

			records = append(records, record)
		}

		inside, err := s.embedded(ctx, request, run, node.ID, "", node.Description, node.CreatedAt, node.UpdatedAt)
		if err != nil {
			return service.ImportFetchPage{}, err
		}

		records = append(records, inside...)

		for _, comment := range node.Comments.Nodes {
			said, err := s.embedded(
				ctx, request, run, node.ID, comment.ID, comment.Body, comment.CreatedAt, comment.UpdatedAt,
			)
			if err != nil {
				return service.ImportFetchPage{}, err
			}

			records = append(records, said...)
		}
	}

	return pageOf(records, reply.Issues.PageInfo), nil
}

// hanging renders one of the rows Linear calls an attachment. Most of them are links — a pull
// request, a Sentry issue, a Slack thread — and a link is nothing this workspace can store, so
// it arrives naming where it came from and no object: the apply pass refuses it and the report
// becomes the list of things somebody has to re-link by hand.
func (s *Source) hanging(
	ctx context.Context,
	request service.ImportFetchRequest,
	run importrun.Run,
	issueID string,
	node attachmentNode,
) (entity.ImportRecord, error) {
	payload := service.ImportAttachmentPayload{
		Issue:     issueID,
		FileName:  fileNameOf(node.Title, node.URL),
		SourceURL: node.URL,
	}

	if upload(node.URL) {
		s.carry(ctx, request, run, blobName(node.ID, payload.FileName), node.URL, &payload)
	} else {
		logging.From(ctx).InfoContext(ctx, "a linear attachment is a link rather than a file",
			"external_id", node.ID,
			"issue", issueID,
			"source_url", node.URL,
		)
	}

	return recordOf(node.ID, "", node.CreatedAt, node.UpdatedAt, payload)
}

func (s *Source) embedded(
	ctx context.Context,
	request service.ImportFetchRequest,
	run importrun.Run,
	issueID, commentID, body, created, updated string,
) ([]entity.ImportRecord, error) {
	sources := inlineUploads(body)
	records := make([]entity.ImportRecord, 0, len(sources))

	owner := issueID
	if commentID != "" {
		owner = commentID
	}

	for _, source := range sources {
		payload := service.ImportAttachmentPayload{
			Issue:     issueID,
			Comment:   commentID,
			FileName:  fileNameOf("", source),
			SourceURL: source,
			Inline:    true,
		}

		s.carry(ctx, request, run, blobName(uploadPath(source), payload.FileName), source, &payload)

		record, err := recordOf(embedKey(source, owner), "", created, updated, payload)
		if err != nil {
			return nil, err
		}

		records = append(records, record)
	}

	return records, nil
}

// carry pulls the bytes while the source's signed URL is still alive. Execute runs after a
// person has read a preview and decided, which is hours or days later and long past the point
// where the same URL answers anything. A file this instance cannot take is left named rather
// than fatal: rows are unbounded and one screenshot says nothing about the backlog behind it.
func (s *Source) carry(
	ctx context.Context,
	request service.ImportFetchRequest,
	run importrun.Run,
	name, source string,
	payload *service.ImportAttachmentPayload,
) {
	body, contentType, err := s.download(ctx, request.Config.Secret, source)
	if err != nil {
		logging.From(ctx).WarnContext(ctx, "a linear file was left where it is",
			"issue", payload.Issue,
			"source_url", source,
			"reason", err.Error(),
		)

		return
	}

	key := entity.ImportBlobKey(run.WorkspaceID, run.RunID, name)

	if err := s.blobs.Put(ctx, key, contentType, bytes.NewReader(body), int64(len(body))); err != nil {
		logging.From(ctx).WarnContext(ctx, "a linear file could not be stored",
			"issue", payload.Issue,
			"source_url", source,
			"reason", err.Error(),
		)

		return
	}

	payload.ObjectKey = key
	payload.ContentType = contentType
	payload.SizeBytes = int64(len(body))
}

func (s *Source) download(ctx context.Context, key, source string) ([]byte, string, error) {
	call, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, "", fmt.Errorf("build file request: %w", err)
	}

	call.Header.Set("Authorization", key)

	response, err := s.files.Do(call)
	if err != nil {
		return nil, "", err
	}

	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("the source answered %d for the file", response.StatusCode)
	}

	read, err := io.ReadAll(io.LimitReader(response.Body, s.limits.MaxAttachmentBytes+1))
	if err != nil {
		return nil, "", err
	}

	if int64(len(read)) > s.limits.MaxAttachmentBytes {
		return nil, "", fmt.Errorf("%w (%d bytes)", ErrFileTooLarge, s.limits.MaxAttachmentBytes)
	}

	return read, response.Header.Get("Content-Type"), nil
}
