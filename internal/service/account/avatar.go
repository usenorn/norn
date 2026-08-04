package account

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/service"
)

func (s *accountsService) UploadAvatar(ctx context.Context, accountID uuid.UUID, upload service.AvatarUpload) (entity.Account, error) {
	if err := s.authorizeSelf(ctx, entity.ActionUpdate, accountID); err != nil {
		return entity.Account{}, err
	}

	if upload.DeclaredSize > entity.AvatarMaxBytes {
		return entity.Account{}, entity.ErrAvatarTooLarge
	}

	account, err := s.activeAccount(ctx, accountID)
	if err != nil {
		return entity.Account{}, err
	}

	content, err := io.ReadAll(io.LimitReader(upload.Body, entity.AvatarMaxBytes+1))
	if err != nil {
		return entity.Account{}, err
	}

	if len(content) > entity.AvatarMaxBytes {
		return entity.Account{}, entity.ErrAvatarTooLarge
	}

	contentType := detectContentType(content)

	extension, supported := entity.AvatarExtension(contentType)
	if !supported {
		return entity.Account{}, entity.ErrAvatarUnsupportedType
	}

	key := entity.AvatarKeyPrefix + "/" + accountID.String() + "/" + uuid.NewString() + extension

	if err := s.blobs.Put(ctx, key, contentType, bytes.NewReader(content), int64(len(content))); err != nil {
		return entity.Account{}, err
	}

	previousKey := account.AvatarObjectKey
	account.AvatarObjectKey = key

	updated, err := s.accounts.Update(ctx, account)
	if err != nil {
		s.discardAvatar(ctx, key)

		return entity.Account{}, err
	}

	s.discardAvatar(ctx, previousKey)

	return updated, nil
}

func (s *accountsService) RemoveAvatar(ctx context.Context, accountID uuid.UUID) (entity.Account, error) {
	if err := s.authorizeSelf(ctx, entity.ActionUpdate, accountID); err != nil {
		return entity.Account{}, err
	}

	account, err := s.activeAccount(ctx, accountID)
	if err != nil {
		return entity.Account{}, err
	}

	if account.AvatarObjectKey == "" {
		return entity.Account{}, entity.ErrAvatarMissing
	}

	previousKey := account.AvatarObjectKey
	account.AvatarObjectKey = ""

	updated, err := s.accounts.Update(ctx, account)
	if err != nil {
		return entity.Account{}, err
	}

	s.discardAvatar(ctx, previousKey)

	return updated, nil
}

func (s *accountsService) discardAvatar(ctx context.Context, key string) {
	if key == "" {
		return
	}

	if err := s.blobs.Delete(ctx, key); err != nil {
		logging.From(ctx).WarnContext(ctx, "discarding avatar object failed", "object_key", key, "error", err.Error())
	}
}

func detectContentType(content []byte) string {
	if len(content) > entity.AvatarSniffLength {
		content = content[:entity.AvatarSniffLength]
	}

	return http.DetectContentType(content)
}

func (s *accountsService) AvatarContent(ctx context.Context, accountID uuid.UUID) (string, error) {
	account, err := s.accounts.GetByID(ctx, accountID)
	if err != nil {
		return "", err
	}

	if account.AvatarObjectKey == "" {
		return "", entity.ErrAvatarMissing
	}

	return s.blobs.PresignGet(ctx, account.AvatarObjectKey, entity.ServeSpec{
		ContentType: entity.AttachmentServedType(entity.AvatarContentType(account.AvatarObjectKey)),
		Disposition: entity.AttachmentDispositionIn,
		FileName:    "avatar",
	}, s.attachments.LinkTTL)
}
