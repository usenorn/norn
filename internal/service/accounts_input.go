package service

import (
	"io"

	"github.com/usenorn/norn/internal/entity"
)

type RegisterAccountInput struct {
	Email       string
	DisplayName string
	Timezone    string
	Password    string
}

type UpdateProfileInput struct {
	DisplayName *string
	Timezone    *string
}

type RequestPasswordResetInput struct {
	Email  string
	Client entity.SessionClient
}

type AvatarUpload struct {
	DeclaredSize int64
	Body         io.Reader
}
