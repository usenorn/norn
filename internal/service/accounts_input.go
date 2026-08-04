package service

import (
	"io"
	"time"

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

type RequestSignUpInput struct {
	Email       string
	DisplayName string
	Timezone    string
	Password    string
	Client      entity.SessionClient
}

type RequestedSignUp struct {
	Email     string
	ExpiresAt time.Time
	Delivery  entity.SignUpDelivery
	URL       string
}

type ConfirmSignUpInput struct {
	Token  string
	Client entity.SessionClient
}

type ConfirmedSignUp struct {
	Account entity.Account
	Session IssuedSession
}
