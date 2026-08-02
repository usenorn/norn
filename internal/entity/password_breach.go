package entity

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"strings"
)

const PasswordBreachPrefixLen = 5

var ErrPasswordBreachCheckUnavailable = errors.New("password breach check is unavailable")

func PasswordBreachDigest(password string) (string, string) {
	sum := sha1.Sum([]byte(password))
	digest := strings.ToUpper(hex.EncodeToString(sum[:]))

	return digest[:PasswordBreachPrefixLen], digest[PasswordBreachPrefixLen:]
}
