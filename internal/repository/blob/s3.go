package blob

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/objectstore"
	"github.com/usenorn/norn/internal/repository"
)

type objectStoreRepository struct {
	client    *objectstore.Client
	presigner *s3.PresignClient
}

func newObjectStore(client *objectstore.Client) repository.Blob {
	return &objectStoreRepository{client: client, presigner: s3.NewPresignClient(client.Client)}
}

func missing(err error) bool {
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}

	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return true
	}

	var responded interface{ HTTPStatusCode() int }

	return errors.As(err, &responded) && responded.HTTPStatusCode() == http.StatusNotFound
}

func (r *objectStoreRepository) Put(
	ctx context.Context,
	key, contentType string,
	body io.Reader,
	size int64,
) error {
	_, err := r.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(r.client.Bucket()),
		Key:           aws.String(key),
		Body:          body,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	})
	if err != nil {
		return fmt.Errorf("put object %q: %w", key, err)
	}

	return nil
}

func (r *objectStoreRepository) read(
	ctx context.Context,
	key, span string,
) (io.ReadCloser, error) {
	input := &s3.GetObjectInput{Bucket: aws.String(r.client.Bucket()), Key: aws.String(key)}
	if span != "" {
		input.Range = aws.String(span)
	}

	object, err := r.client.GetObject(ctx, input)
	if err != nil {
		if missing(err) {
			return nil, entity.ErrBlobNotFound
		}

		return nil, fmt.Errorf("get object %q: %w", key, err)
	}

	return object.Body, nil
}

func (r *objectStoreRepository) Open(ctx context.Context, key string) (io.ReadSeekCloser, error) {
	object, err := r.Stat(ctx, key)
	if err != nil {
		return nil, err
	}

	return &rangeReader{store: r, ctx: ctx, key: key, size: object.Size}, nil
}

func (r *objectStoreRepository) Sniff(ctx context.Context, key string) (string, error) {
	body, err := r.read(ctx, key, "bytes=0-"+strconv.Itoa(entity.AttachmentSniffLength-1))
	if err != nil {
		return "", err
	}

	defer func() { _ = body.Close() }()

	head := make([]byte, entity.AttachmentSniffLength)

	length, err := io.ReadFull(body, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read object %q: %w", key, err)
	}

	return http.DetectContentType(head[:length]), nil
}

func (r *objectStoreRepository) Stat(ctx context.Context, key string) (entity.BlobObject, error) {
	head, err := r.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(r.client.Bucket()),
		Key:    aws.String(key),
	})
	if err != nil {
		if missing(err) {
			return entity.BlobObject{}, entity.ErrBlobNotFound
		}

		return entity.BlobObject{}, fmt.Errorf("stat object %q: %w", key, err)
	}

	return entity.BlobObject{
		Size:        aws.ToInt64(head.ContentLength),
		ContentType: aws.ToString(head.ContentType),
		ModifiedAt:  aws.ToTime(head.LastModified),
	}, nil
}

func (r *objectStoreRepository) Delete(ctx context.Context, key string) error {
	_, err := r.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(r.client.Bucket()),
		Key:    aws.String(key),
	})
	if err != nil && !missing(err) {
		return fmt.Errorf("delete object %q: %w", key, err)
	}

	return nil
}

func (r *objectStoreRepository) RemoveAll(ctx context.Context, prefix string) error {
	pages := s3.NewListObjectsV2Paginator(r.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(r.client.Bucket()),
		Prefix: aws.String(prefix),
	})

	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("list objects under %q: %w", prefix, err)
		}

		keys := make([]types.ObjectIdentifier, 0, len(page.Contents))
		for _, object := range page.Contents {
			keys = append(keys, types.ObjectIdentifier{Key: object.Key})
		}

		if len(keys) == 0 {
			continue
		}

		if _, err := r.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(r.client.Bucket()),
			Delete: &types.Delete{Objects: keys, Quiet: aws.Bool(true)},
		}); err != nil {
			return fmt.Errorf("delete objects under %q: %w", prefix, err)
		}
	}

	return nil
}

func (r *objectStoreRepository) PresignPut(
	ctx context.Context,
	key string,
	ttl time.Duration,
) (entity.BlobTicket, error) {
	request, err := r.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r.client.Bucket()),
		Key:         aws.String(key),
		ContentType: aws.String(entity.AttachmentGenericType),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return entity.BlobTicket{}, fmt.Errorf("presign upload for %q: %w", key, err)
	}

	return entity.BlobTicket{
		URL:       request.URL,
		Method:    request.Method,
		Headers:   map[string]string{"Content-Type": entity.AttachmentGenericType},
		ExpiresAt: time.Now().UTC().Add(ttl),
	}, nil
}

func (r *objectStoreRepository) PresignGet(
	ctx context.Context,
	key string,
	serve entity.ServeSpec,
	ttl time.Duration,
) (string, error) {
	request, err := r.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     aws.String(r.client.Bucket()),
		Key:                        aws.String(key),
		ResponseContentType:        aws.String(serve.ContentType),
		ResponseContentDisposition: aws.String(entity.ContentDisposition(serve)),
		ResponseCacheControl:       aws.String("private, no-store"),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("presign download for %q: %w", key, err)
	}

	return request.URL, nil
}

type rangeReader struct {
	store  *objectStoreRepository
	ctx    context.Context
	key    string
	size   int64
	offset int64
}

func (s *rangeReader) Read(buffer []byte) (int, error) {
	if s.offset >= s.size {
		return 0, io.EOF
	}

	end := min(s.offset+int64(len(buffer))-1, s.size-1)

	body, err := s.store.read(
		s.ctx, s.key, "bytes="+strconv.FormatInt(s.offset, 10)+"-"+strconv.FormatInt(end, 10),
	)
	if err != nil {
		return 0, err
	}

	defer func() { _ = body.Close() }()

	read, err := io.ReadFull(body, buffer[:end-s.offset+1])
	s.offset += int64(read)

	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		err = nil
	}

	return read, err
}

func (s *rangeReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		s.offset = offset
	case io.SeekCurrent:
		s.offset += offset
	case io.SeekEnd:
		s.offset = s.size + offset
	default:
		return 0, fmt.Errorf("seek object %q: unknown whence %d", s.key, whence)
	}

	if s.offset < 0 {
		return 0, fmt.Errorf("seek object %q: negative position", s.key)
	}

	return s.offset, nil
}

func (s *rangeReader) Close() error { return nil }
