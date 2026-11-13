package config

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/githost"
)

const (
	spKeyBits        = 2048
	spCertificateTtl = 10 * 365 * 24 * time.Hour
)

type KeyPair struct {
	Certificate *x509.Certificate
	Key         *rsa.PrivateKey
}

func (s *Service) KeyPair(ctx context.Context) (*KeyPair, error) {
	key, err := LoadKey()
	if err != nil {
		return nil, err
	}
	stored, err := s.repo.loadKeyPair(ctx)
	if err != nil {
		return nil, err
	}
	return openKeyPair(key, stored)
}

func (s *Service) ensureKeyPair(ctx context.Context, key []byte) error {
	stored, err := s.repo.loadKeyPair(ctx)
	if err != nil {
		return err
	}
	if _, err := openKeyPair(key, stored); !errors.Is(err, ErrSpKeyMissing) {
		return err
	}
	generated, err := sealedKeyPair(key)
	if err != nil {
		return err
	}
	return s.repo.saveMissingKeyPair(ctx, generated)
}

// RegenerateKeyPair only replaces a pair whose key no longer decrypts, because a new certificate must be uploaded to the IdP.
func (s *Service) RegenerateKeyPair(ctx context.Context) error {
	key, err := LoadKey()
	if err != nil {
		return err
	}
	stored, err := s.repo.loadKeyPair(ctx)
	if err != nil {
		return err
	}
	if _, err := openKeyPair(key, stored); !errors.Is(err, ErrSpKeyUndecryptable) {
		return ErrSpKeyReadable
	}
	generated, err := sealedKeyPair(key)
	if err != nil {
		return err
	}
	return s.repo.saveKeyPair(ctx, generated)
}

func sealedKeyPair(key []byte) (storedKeyPair, error) {
	certificate, privateKey, err := newKeyPair()
	if err != nil {
		return storedKeyPair{}, err
	}
	ciphertext, nonce, err := githost.Encrypt(key, x509.MarshalPKCS1PrivateKey(privateKey))
	if err != nil {
		return storedKeyPair{}, fmt.Errorf("encrypting sp key: %w", err)
	}
	return storedKeyPair{Certificate: certificate, KeyEnc: ciphertext, KeyNonce: nonce}, nil
}

func newKeyPair() (string, *rsa.PrivateKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, spKeyBits)
	if err != nil {
		return "", nil, fmt.Errorf("generating sp key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 62))
	if err != nil {
		return "", nil, fmt.Errorf("generating sp certificate serial: %w", err)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "rurdesk SAML service provider"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(spCertificateTtl),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", nil, fmt.Errorf("creating sp certificate: %w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), privateKey, nil
}

func openKeyPair(key []byte, stored storedKeyPair) (*KeyPair, error) {
	if stored.Certificate == "" || len(stored.KeyEnc) == 0 {
		return nil, ErrSpKeyMissing
	}
	block, _ := pem.Decode([]byte(stored.Certificate))
	if block == nil {
		return nil, ErrSpKeyUndecryptable
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, ErrSpKeyUndecryptable
	}
	plaintext, err := githost.Decrypt(key, stored.KeyNonce, stored.KeyEnc)
	if err != nil {
		return nil, ErrSpKeyUndecryptable
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(plaintext)
	if err != nil {
		return nil, ErrSpKeyUndecryptable
	}
	return &KeyPair{Certificate: certificate, Key: privateKey}, nil
}
