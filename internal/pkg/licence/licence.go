package licence

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
)

const issuerPublicKey = "F/brktkihM/8aVWvO8jfoDqRWfwjea7qGyaHm2mGjRI="

type claims struct {
	Holder    string    `json:"holder"`
	IssuedAt  time.Time `json:"issuedAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	Features  features  `json:"features"`
}

type features struct {
	Audit     bool `json:"audit"`
	Directory bool `json:"directory"`
}

func Resolve(cfg config.Licence) (entity.Licence, error) {
	if strings.TrimSpace(cfg.Key) == "" {
		return entity.Licence{}, nil
	}

	return Verify(cfg.Key)
}

func Verify(key string) (entity.Licence, error) {
	issuer, err := base64.StdEncoding.DecodeString(issuerPublicKey)
	if err != nil || len(issuer) != ed25519.PublicKeySize {
		return entity.Licence{}, fmt.Errorf("the compiled-in issuer key is unusable")
	}

	body, signature, found := strings.Cut(strings.TrimSpace(key), ".")
	if !found {
		return entity.Licence{}, entity.ErrLicenceMalformed
	}

	stated, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return entity.Licence{}, entity.ErrLicenceMalformed
	}

	sealed, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return entity.Licence{}, entity.ErrLicenceMalformed
	}

	if !ed25519.Verify(ed25519.PublicKey(issuer), stated, sealed) {
		return entity.Licence{}, entity.ErrLicenceForged
	}

	var decoded claims

	if err := json.Unmarshal(stated, &decoded); err != nil {
		return entity.Licence{}, entity.ErrLicenceMalformed
	}

	return entity.Licence{
		Holder:    decoded.Holder,
		IssuedAt:  decoded.IssuedAt,
		ExpiresAt: decoded.ExpiresAt,
		Features: entity.LicenceFeatures{
			Audit:     decoded.Features.Audit,
			Directory: decoded.Features.Directory,
		},
	}, nil
}
