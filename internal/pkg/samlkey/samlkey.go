package samlkey

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"time"
)

const (
	keyBits  = 2048
	lifetime = 10 * 365 * 24 * time.Hour
)

var (
	ErrPrivateKeyMalformed  = errors.New("the stored service provider key is not a usable RSA key")
	ErrCertificateMalformed = errors.New("the certificate is not readable as PEM or base64 X.509")
)

type Keypair struct {
	PrivateKey  *rsa.PrivateKey
	Certificate *x509.Certificate
}

func Generate(subject string, now time.Time) (Keypair, error) {
	key, err := rsa.GenerateKey(rand.Reader, keyBits)
	if err != nil {
		return Keypair{}, fmt.Errorf("generate service provider key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return Keypair{}, fmt.Errorf("generate certificate serial: %w", err)
	}

	template := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: subject},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(lifetime),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return Keypair{}, fmt.Errorf("create service provider certificate: %w", err)
	}

	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		return Keypair{}, fmt.Errorf("parse service provider certificate: %w", err)
	}

	return Keypair{PrivateKey: key, Certificate: certificate}, nil
}

func MarshalPrivateKey(key *rsa.PrivateKey) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
}

func ParsePrivateKey(encoded []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(encoded)
	if block == nil {
		return nil, ErrPrivateKeyMalformed
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, ErrPrivateKeyMalformed
	}

	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, ErrPrivateKeyMalformed
	}

	return key, nil
}

func MarshalCertificate(certificate *x509.Certificate) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Raw}))
}

func ParseCertificate(encoded string) (*x509.Certificate, error) {
	if block, _ := pem.Decode([]byte(encoded)); block != nil {
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, ErrCertificateMalformed
		}

		return certificate, nil
	}

	der, err := base64.StdEncoding.DecodeString(compact(encoded))
	if err != nil {
		return nil, ErrCertificateMalformed
	}

	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, ErrCertificateMalformed
	}

	return certificate, nil
}

func EarliestExpiry(certificates []string) (time.Time, error) {
	earliest := time.Time{}

	for _, encoded := range certificates {
		certificate, err := ParseCertificate(encoded)
		if err != nil {
			return time.Time{}, err
		}

		if earliest.IsZero() || certificate.NotAfter.Before(earliest) {
			earliest = certificate.NotAfter
		}
	}

	if earliest.IsZero() {
		return time.Time{}, ErrCertificateMalformed
	}

	return earliest, nil
}

func compact(value string) string {
	out := make([]rune, 0, len(value))

	for _, r := range value {
		switch r {
		case ' ', '\t', '\n', '\r':
		default:
			out = append(out, r)
		}
	}

	return string(out)
}
