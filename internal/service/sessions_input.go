package service

import "github.com/usenorn/norn/internal/entity"

type SignInInput struct {
	Email    string
	Password string
	Client   entity.SessionClient
}

type IssuedSession struct {
	Session entity.Session
	Token   string
}
