package crypter

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"golang.org/x/crypto/chacha20poly1305"

	"github.com/usenorn/norn/internal/config"
)

const KeyBytes = chacha20poly1305.KeySize

var (
	ErrKeyMissing   = errors.New("no encryption key is configured on this instance")
	ErrKeyMalformed = errors.New("the configured encryption key is not 32 base64-encoded bytes")
	ErrCiphertext   = errors.New("stored ciphertext could not be opened")
)

type Crypter struct {
	key []byte
}

func New(cfg config.Security) (*Crypter, error) {
	if cfg.EncryptionKey == "" {
		return &Crypter{}, nil
	}

	key, err := base64.StdEncoding.DecodeString(cfg.EncryptionKey)
	if err != nil || len(key) != KeyBytes {
		return nil, ErrKeyMalformed
	}

	return &Crypter{key: key}, nil
}

func (c *Crypter) Ready() bool {
	return len(c.key) == KeyBytes
}

func (c *Crypter) Seal(plaintext []byte) ([]byte, error) {
	if !c.Ready() {
		return nil, ErrKeyMissing
	}

	aead, err := chacha20poly1305.NewX(c.key)
	if err != nil {
		return nil, fmt.Errorf("build cipher: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	return aead.Seal(nonce, nonce, plaintext, nil), nil
}

func (c *Crypter) Open(sealed []byte) ([]byte, error) {
	if !c.Ready() {
		return nil, ErrKeyMissing
	}

	aead, err := chacha20poly1305.NewX(c.key)
	if err != nil {
		return nil, fmt.Errorf("build cipher: %w", err)
	}

	if len(sealed) < aead.NonceSize() {
		return nil, ErrCiphertext
	}

	nonce, body := sealed[:aead.NonceSize()], sealed[aead.NonceSize():]

	plaintext, err := aead.Open(nil, nonce, body, nil)
	if err != nil {
		return nil, ErrCiphertext
	}

	return plaintext, nil
}
