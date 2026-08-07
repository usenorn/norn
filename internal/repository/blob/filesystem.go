package blob

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
)

const (
	uploadPathPrefix   = "/v1/blobs/upload/"
	downloadPathPrefix = "/v1/blobs/download/"
	temporaryPrefix    = "tmp"
	directoryMode      = 0o750
	fileMode           = 0o640
)

type filesystemRepository struct {
	root         *os.Root
	base         string
	grants       repository.BlobGrant
	uploadTTL    time.Duration
	maxFileBytes int64
}

func newFilesystem(
	cfg config.Storage,
	attachments config.Attachments,
	grants repository.BlobGrant,
) (repository.Blob, error) {
	base, err := filepath.Abs(cfg.Root)
	if err != nil {
		return nil, fmt.Errorf("resolve storage root: %w", err)
	}

	if err := os.MkdirAll(filepath.Join(base, temporaryPrefix), directoryMode); err != nil {
		return nil, fmt.Errorf("create storage root: %w", err)
	}

	root, err := os.OpenRoot(base)
	if err != nil {
		return nil, fmt.Errorf("open storage root: %w", err)
	}

	return &filesystemRepository{
		root:         root,
		base:         base,
		grants:       grants,
		uploadTTL:    attachments.UploadTTL,
		maxFileBytes: attachments.MaxFileBytes,
	}, nil
}

func (r *filesystemRepository) resolve(key string) (string, error) {
	if !entity.ValidBlobKey(key) {
		return "", entity.ErrBlobNotFound
	}

	return filepath.FromSlash(key), nil
}

func translate(err error, key string, action string) error {
	if errors.Is(err, fs.ErrNotExist) {
		return entity.ErrBlobNotFound
	}

	return fmt.Errorf("%s %q: %w", action, key, err)
}

func (r *filesystemRepository) Put(
	ctx context.Context,
	key, contentType string,
	body io.Reader,
	size int64,
) error {
	path, err := r.resolve(key)
	if err != nil {
		return err
	}

	staged := filepath.Join(temporaryPrefix, uuid.NewString())

	file, err := r.root.OpenFile(staged, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fileMode)
	if err != nil {
		return fmt.Errorf("stage object %q: %w", key, err)
	}

	written, err := io.Copy(file, body)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}

	if err != nil {
		_ = r.root.Remove(staged)

		return fmt.Errorf("write object %q: %w", key, err)
	}

	if size >= 0 && written != size {
		_ = r.root.Remove(staged)

		return fmt.Errorf("write object %q: wrote %d of %d bytes", key, written, size)
	}

	if err := os.MkdirAll(filepath.Dir(filepath.Join(r.base, path)), directoryMode); err != nil {
		_ = r.root.Remove(staged)

		return fmt.Errorf("create object directory for %q: %w", key, err)
	}

	if err := os.Rename(filepath.Join(r.base, staged), filepath.Join(r.base, path)); err != nil {
		_ = r.root.Remove(staged)

		return fmt.Errorf("place object %q: %w", key, err)
	}

	return nil
}

func (r *filesystemRepository) Open(_ context.Context, key string) (io.ReadSeekCloser, error) {
	path, err := r.resolve(key)
	if err != nil {
		return nil, err
	}

	file, err := r.root.Open(path)
	if err != nil {
		return nil, translate(err, key, "open object")
	}

	return file, nil
}

func (r *filesystemRepository) Sniff(ctx context.Context, key string) (string, error) {
	file, err := r.Open(ctx, key)
	if err != nil {
		return "", err
	}

	defer func() { _ = file.Close() }()

	head := make([]byte, entity.AttachmentSniffLength)

	read, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", translate(err, key, "read object")
	}

	return http.DetectContentType(head[:read]), nil
}

func (r *filesystemRepository) Stat(_ context.Context, key string) (entity.BlobObject, error) {
	path, err := r.resolve(key)
	if err != nil {
		return entity.BlobObject{}, err
	}

	info, err := r.root.Stat(path)
	if err != nil {
		return entity.BlobObject{}, translate(err, key, "stat object")
	}

	return entity.BlobObject{Size: info.Size(), ModifiedAt: info.ModTime()}, nil
}

func (r *filesystemRepository) Delete(_ context.Context, key string) error {
	path, err := r.resolve(key)
	if err != nil {
		if errors.Is(err, entity.ErrBlobNotFound) {
			return nil
		}

		return err
	}

	if err := r.root.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("delete object %q: %w", key, err)
	}

	return nil
}

func (r *filesystemRepository) RemoveAll(_ context.Context, prefix string) error {
	path, err := r.resolve(prefix)
	if err != nil {
		if errors.Is(err, entity.ErrBlobNotFound) {
			return nil
		}

		return err
	}

	resolved := filepath.Join(r.base, path)
	if !strings.HasPrefix(resolved, r.base+string(os.PathSeparator)) {
		return entity.ErrBlobNotFound
	}

	if err := os.RemoveAll(resolved); err != nil {
		return fmt.Errorf("remove objects under %q: %w", prefix, err)
	}

	return nil
}

func (r *filesystemRepository) PresignPut(
	ctx context.Context,
	key string,
	ttl time.Duration,
) (entity.BlobTicket, error) {
	expires := time.Now().UTC().Add(ttl)

	token, err := r.grants.Issue(ctx, entity.BlobGrant{
		Purpose:   entity.BlobGrantUpload,
		Key:       key,
		MaxBytes:  r.maxFileBytes,
		ExpiresAt: expires,
	}, ttl)
	if err != nil {
		return entity.BlobTicket{}, err
	}

	return entity.BlobTicket{
		URL:       uploadPathPrefix + token,
		Method:    http.MethodPut,
		Headers:   map[string]string{"Content-Type": entity.AttachmentGenericType},
		ExpiresAt: expires,
	}, nil
}

func (r *filesystemRepository) PresignGet(
	ctx context.Context,
	key string,
	serve entity.ServeSpec,
	ttl time.Duration,
) (string, error) {
	token, err := r.grants.Issue(ctx, entity.BlobGrant{
		Purpose:   entity.BlobGrantDownload,
		Key:       key,
		Serve:     serve,
		ExpiresAt: time.Now().UTC().Add(ttl),
	}, ttl)
	if err != nil {
		return "", err
	}

	return downloadPathPrefix + token, nil
}
