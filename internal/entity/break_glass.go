package entity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	BreakGlassCodeCount = 8
	BreakGlassCodeBytes = 10
)

var (
	ErrBreakGlassCodeInvalid  = errors.New("that recovery code is not one this workspace issued")
	ErrBreakGlassNotEnforcing = errors.New("this workspace does not require single sign-on")
)

var breakGlassAlphabet = base32.NewEncoding("ABCDEFGHJKLMNPQRSTUVWXYZ23456789").WithPadding(base32.NoPadding)

type BreakGlassCode struct {
	ID           uuid.UUID
	WorkspaceID  uuid.UUID
	CodeHash     []byte
	IssuedAt     time.Time
	IssuedBy     *uuid.UUID
	RedeemedAt   *time.Time
	RedeemedFrom string
}

func (c BreakGlassCode) Redeemed() bool { return c.RedeemedAt != nil }

func NewBreakGlassCode() (string, []byte, error) {
	raw := make([]byte, BreakGlassCodeBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate recovery code: %w", err)
	}

	encoded := breakGlassAlphabet.EncodeToString(raw)
	code := encoded[:4] + "-" + encoded[4:8] + "-" + encoded[8:]

	return code, HashBreakGlassCode(code), nil
}

func HashBreakGlassCode(code string) []byte {
	sum := sha256.Sum256([]byte(NormalizeBreakGlassCode(code)))

	return sum[:]
}

func NormalizeBreakGlassCode(code string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(code), "-", ""))
}
