package forge

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

const (
	AppJWTLifetime = 9 * time.Minute
	appJWTBackdate = 60 * time.Second
)

type appClaims struct {
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	Issuer    string `json:"iss"`
}

func ParseAppPrivateKey(pemKey string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(pemKey)))
	if block == nil {
		return nil, entity.ErrSCMPrivateKeyInvalid
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, entity.ErrSCMPrivateKeyInvalid
	}

	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, entity.ErrSCMPrivateKeyInvalid
	}

	return key, nil
}

func AppJWT(pemKey, externalAppID string, now time.Time) (string, error) {
	key, err := ParseAppPrivateKey(pemKey)
	if err != nil {
		return "", err
	}

	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	if err != nil {
		return "", fmt.Errorf("encode the application token header: %w", err)
	}

	claims, err := json.Marshal(appClaims{
		IssuedAt:  now.Add(-appJWTBackdate).Unix(),
		ExpiresAt: now.Add(AppJWTLifetime).Unix(),
		Issuer:    externalAppID,
	})
	if err != nil {
		return "", fmt.Errorf("encode the application token claims: %w", err)
	}

	signing := segment(header) + "." + segment(claims)

	digest := sha256.Sum256([]byte(signing))

	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign the application token: %w", err)
	}

	return signing + "." + segment(signature), nil
}

func segment(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}
