package entity

import (
	"errors"

	"github.com/google/uuid"
)

const (
	WorkspaceLogoKeyPrefix    = "workspace-logos"
	WorkspaceLogoMaxBytes     = 2 << 20
	WorkspaceLogoMinDimension = 128
	WorkspaceLogoMaxDimension = 4096
)

var (
	ErrWorkspaceLogoTooLarge        = errors.New("workspace logo exceeds the maximum size")
	ErrWorkspaceLogoTooSmall        = errors.New("workspace logo is smaller than the minimum dimensions")
	ErrWorkspaceLogoUnsupportedType = errors.New("workspace logo content type is not supported")
	ErrWorkspaceLogoMissing         = errors.New("workspace has no logo")
)

func WorkspaceLogoKey(workspaceID uuid.UUID, extension string) string {
	return WorkspaceLogoPrefix(workspaceID) + "/" + uuid.NewString() + extension
}

func WorkspaceLogoPrefix(workspaceID uuid.UUID) string {
	return WorkspaceLogoKeyPrefix + "/" + workspaceID.String()
}

func WorkspaceLogoServeSpec(key string) ServeSpec {
	return ServeSpec{
		ContentType: AttachmentServedType(AvatarContentType(key)),
		Disposition: AttachmentDispositionIn,
		FileName:    "logo",
	}
}

func WorkspaceLogoTooSmall(width, height int) bool {
	return width < WorkspaceLogoMinDimension || height < WorkspaceLogoMinDimension
}

func WorkspaceLogoTooLarge(width, height int) bool {
	return width > WorkspaceLogoMaxDimension || height > WorkspaceLogoMaxDimension
}
