package entity

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	PasswordMinLen = 12
	PasswordMaxLen = 256

	argon2Time      = 3
	argon2Memory    = 64 * 1024
	argon2Threads   = 2
	argon2KeyLength = 32
	argon2SaltBytes = 16
	argon2Scheme    = "argon2id"
)

var ErrPasswordHashMalformed = errors.New("password hash is malformed")

func ValidatePassword(field, password string) FieldError {
	length := utf8.RuneCountInString(password)

	switch {
	case length == 0:
		return FieldError{Field: field, Code: ValidationCodeRequired}
	case length < PasswordMinLen:
		return FieldError{Field: field, Code: ValidationCodeTooShort}
	case length > PasswordMaxLen:
		return FieldError{Field: field, Code: ValidationCodeTooLong}
	default:
		return FieldError{}
	}
}

func HashPassword(password string) (string, error) {
	salt := make([]byte, argon2SaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	key := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLength)

	return fmt.Sprintf("$%s$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2Scheme,
		argon2.Version,
		argon2Memory,
		argon2Time,
		argon2Threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

func VerifyPassword(hash, password string) (bool, error) {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != argon2Scheme {
		return false, ErrPasswordHashMalformed
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, ErrPasswordHashMalformed
	}

	var memory uint32

	var time, threads uint8

	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, ErrPasswordHashMalformed
	}

	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil {
		return false, ErrPasswordHashMalformed
	}

	key, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil {
		return false, ErrPasswordHashMalformed
	}

	candidate := argon2.IDKey([]byte(password), salt, uint32(time), memory, threads, uint32(len(key)))

	return subtle.ConstantTimeCompare(key, candidate) == 1, nil
}
