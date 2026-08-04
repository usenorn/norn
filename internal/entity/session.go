package entity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	SessionTokenBytes = 32
	UserAgentMaxLen   = 512
)

var (
	ErrSessionNotFound          = errors.New("session not found")
	ErrSessionRevoked           = errors.New("session was revoked")
	ErrSessionExpired           = errors.New("session expired")
	ErrSessionAuthMethodUnknown = errors.New("session auth method is not one Norn issues")
)

type SessionAuthMethod string

const (
	SessionAuthMethodPassword SessionAuthMethod = "password"
	SessionAuthMethodSSO      SessionAuthMethod = "sso"
)

func (m SessionAuthMethod) Valid() bool {
	switch m {
	case SessionAuthMethodPassword, SessionAuthMethodSSO:
		return true
	default:
		return false
	}
}

type SessionClient struct {
	UserAgent string
	IP        netip.Addr
	Location  Location
}

type Session struct {
	ID                uuid.UUID
	TokenHash         string
	AccountID         uuid.UUID
	AuthMethod        SessionAuthMethod
	Client            SessionClient
	IssuedAt          time.Time
	LastUsedAt        time.Time
	IdleExpiresAt     time.Time
	AbsoluteExpiresAt time.Time
}

func NewSessionToken() (string, string, error) {
	raw := make([]byte, SessionTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate session token: %w", err)
	}

	token := base64.RawURLEncoding.EncodeToString(raw)

	return token, HashSessionToken(token), nil
}

func HashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))

	return hex.EncodeToString(sum[:])
}

func TruncateUserAgent(userAgent string) string {
	if utf8.RuneCountInString(userAgent) <= UserAgentMaxLen {
		return userAgent
	}

	runes := []rune(userAgent)

	return string(runes[:UserAgentMaxLen])
}

func (s Session) ExpiresAt() time.Time {
	if s.IdleExpiresAt.Before(s.AbsoluteExpiresAt) {
		return s.IdleExpiresAt
	}

	return s.AbsoluteExpiresAt
}

func (s Session) ExpiredAt(now time.Time) bool {
	return !now.Before(s.ExpiresAt())
}

func (s Session) RevokedAt(revokedAt time.Time) bool {
	return !revokedAt.IsZero() && !s.IssuedAt.After(revokedAt)
}

func (s Session) NeedsRefresh(now time.Time, interval time.Duration) bool {
	if !s.IdleExpiresAt.Before(s.AbsoluteExpiresAt) {
		return false
	}

	return now.Sub(s.LastUsedAt) >= interval
}

func (s Session) Refreshed(now time.Time, idleTimeout time.Duration) Session {
	s.LastUsedAt = now

	s.IdleExpiresAt = now.Add(idleTimeout)
	if s.IdleExpiresAt.After(s.AbsoluteExpiresAt) {
		s.IdleExpiresAt = s.AbsoluteExpiresAt
	}

	return s
}
