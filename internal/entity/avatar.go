package entity

import (
	"errors"
	"strings"
)

const (
	AvatarMaxBytes    = 2 << 20
	AvatarKeyPrefix   = "avatars"
	AvatarSniffLength = 512
)

var (
	ErrAvatarTooLarge        = errors.New("avatar exceeds the maximum size")
	ErrAvatarUnsupportedType = errors.New("avatar content type is not supported")
	ErrAvatarMissing         = errors.New("account has no avatar")
)

var avatarExtensions = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

func AvatarExtension(contentType string) (string, bool) {
	extension, ok := avatarExtensions[contentType]

	return extension, ok
}

func AvatarContentType(key string) string {
	for contentType, extension := range avatarExtensions {
		if strings.HasSuffix(key, extension) {
			return contentType
		}
	}

	return AttachmentGenericType
}
