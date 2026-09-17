package workspace

import (
	"bytes"
	"context"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"

	"github.com/google/uuid"
	_ "golang.org/x/image/webp"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
)

var logoFormats = map[string]string{
	"image/jpeg": "jpeg",
	"image/png":  "png",
	"image/webp": "webp",
}

func (s *workspacesService) UploadLogo(ctx context.Context, workspaceID uuid.UUID, body io.Reader) (entity.Workspace, error) {
	if _, err := s.authorizer.Decide(ctx, entity.AccessRequest{Resource: entity.ResourceWorkspace, Action: entity.ActionUpdate, WorkspaceID: workspaceID}); err != nil {
		return entity.Workspace{}, err
	}

	content, err := io.ReadAll(io.LimitReader(body, entity.WorkspaceLogoMaxBytes+1))
	if err != nil {
		return entity.Workspace{}, err
	}

	if len(content) > entity.WorkspaceLogoMaxBytes {
		return entity.Workspace{}, entity.ErrWorkspaceLogoTooLarge
	}

	contentType, err := verifiedLogoType(content)
	if err != nil {
		return entity.Workspace{}, err
	}

	extension, _ := entity.AvatarExtension(contentType)
	key := entity.WorkspaceLogoKey(workspaceID, extension)

	if err := s.blobs.Put(ctx, key, contentType, bytes.NewReader(content), int64(len(content))); err != nil {
		return entity.Workspace{}, err
	}

	updated, previousKey, err := s.swapLogo(ctx, workspaceID, key)
	if err != nil {
		s.discardLogo(ctx, key)

		return entity.Workspace{}, err
	}

	s.discardLogo(ctx, previousKey)

	return updated, nil
}

func (s *workspacesService) RemoveLogo(ctx context.Context, workspaceID uuid.UUID) (entity.Workspace, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{Resource: entity.ResourceWorkspace, Action: entity.ActionUpdate, WorkspaceID: workspaceID})
	if err != nil {
		return entity.Workspace{}, err
	}

	if decision.Workspace.LogoObjectKey == "" {
		return entity.Workspace{}, entity.ErrWorkspaceLogoMissing
	}

	updated, previousKey, err := s.swapLogo(ctx, workspaceID, "")
	if err != nil {
		return entity.Workspace{}, err
	}

	s.discardLogo(ctx, previousKey)

	return updated, nil
}

func (s *workspacesService) LogoContent(ctx context.Context, workspaceID uuid.UUID) (string, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{Resource: entity.ResourceWorkspace, Action: entity.ActionRead, WorkspaceID: workspaceID})
	if err != nil {
		return "", err
	}

	key := decision.Workspace.LogoObjectKey
	if key == "" {
		return "", entity.ErrWorkspaceLogoMissing
	}

	return s.blobs.PresignGet(ctx, key, entity.ServeSpec{
		ContentType: entity.AttachmentServedType(entity.AvatarContentType(key)),
		Disposition: entity.AttachmentDispositionIn,
		FileName:    "logo",
	}, s.attachments.LinkTTL)
}

func (s *workspacesService) swapLogo(ctx context.Context, workspaceID uuid.UUID, key string) (entity.Workspace, string, error) {
	var (
		updated     entity.Workspace
		previousKey string
	)

	err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.workspaces.LockByIDs(ctx, []uuid.UUID{workspaceID}); err != nil {
			return err
		}

		locked, err := s.workspaces.GetByID(ctx, workspaceID)
		if err != nil {
			return err
		}

		saved, err := s.workspaces.SetLogo(ctx, workspaceID, key)
		if err != nil {
			return err
		}

		updated = saved
		previousKey = locked.LogoObjectKey

		return nil
	})
	if err != nil {
		return entity.Workspace{}, "", err
	}

	return updated, previousKey, nil
}

func (s *workspacesService) discardLogo(ctx context.Context, key string) {
	if key == "" {
		return
	}

	if err := s.blobs.Delete(ctx, key); err != nil {
		logging.From(ctx).WarnContext(ctx, "discarding workspace logo object failed", "object_key", key, "error", err.Error())
	}
}

func verifiedLogoType(content []byte) (string, error) {
	sniffed := content
	if len(sniffed) > entity.AvatarSniffLength {
		sniffed = sniffed[:entity.AvatarSniffLength]
	}

	contentType := http.DetectContentType(sniffed)

	format, supported := logoFormats[contentType]
	if !supported {
		return "", entity.ErrWorkspaceLogoUnsupportedType
	}

	config, declared, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil || declared != format {
		return "", entity.ErrWorkspaceLogoUnsupportedType
	}

	if entity.WorkspaceLogoTooLarge(config.Width, config.Height) {
		return "", entity.ErrWorkspaceLogoTooLarge
	}

	if entity.WorkspaceLogoTooSmall(config.Width, config.Height) {
		return "", entity.ErrWorkspaceLogoTooSmall
	}

	if _, decoded, err := image.Decode(bytes.NewReader(content)); err != nil || decoded != format {
		return "", entity.ErrWorkspaceLogoUnsupportedType
	}

	return contentType, nil
}
