package crypto

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// AESGCMEncryptor implements auth.TokenEncryptor using AES-256-GCM.
// The stored format is base64url(nonce || ciphertext || tag).
// Key must be exactly 32 bytes.
type AESGCMEncryptor struct {
	key []byte // 32-byte AES-256 key
}

// NewAESGCMEncryptor constructs an encryptor from a 32-byte key.
// Returns an error if the key is not exactly 32 bytes.
func NewAESGCMEncryptor(key []byte) (*AESGCMEncryptor, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("aes-gcm encryptor: key must be 32 bytes, got %d", len(key))
	}
	keyCopy := make([]byte, 32)
	copy(keyCopy, key)
	return &AESGCMEncryptor{key: keyCopy}, nil
}

func (e *AESGCMEncryptor) Encrypt(_ context.Context, plaintext string) (string, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("aes-gcm encrypt: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("aes-gcm encrypt: new gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("aes-gcm encrypt: read nonce: %w", err)
	}

	// Seal appends ciphertext+tag to nonce.
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (e *AESGCMEncryptor) Decrypt(_ context.Context, ciphertext string) (string, error) {
	data, err := base64.RawURLEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("aes-gcm decrypt: base64 decode: %w", err)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("aes-gcm decrypt: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("aes-gcm decrypt: new gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("aes-gcm decrypt: ciphertext too short")
	}
	nonce, sealed := data[:nonceSize], data[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("aes-gcm decrypt: open: %w", err)
	}
	return string(plaintext), nil
}
