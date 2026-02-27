package cryptox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

const (
	encryptedValuePrefix = "enc:v1:"
	gcmNonceSize         = 12
)

func EncryptWithSecret(secret, plaintext string) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", fmt.Errorf("secret is required")
	}
	if plaintext == "" {
		return "", nil
	}

	key := deriveKey(secret)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}

	nonce := make([]byte, gcmNonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	raw := append(nonce, ciphertext...)
	return encryptedValuePrefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

func DecryptWithSecret(secret, value string) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", fmt.Errorf("secret is required")
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if !strings.HasPrefix(trimmed, encryptedValuePrefix) {
		// Backward-compatible path for values that predate encryption.
		return trimmed, nil
	}

	encoded := strings.TrimPrefix(trimmed, encryptedValuePrefix)
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode encrypted payload: %w", err)
	}
	if len(raw) <= gcmNonceSize {
		return "", fmt.Errorf("encrypted payload is invalid")
	}

	key := deriveKey(secret)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}

	nonce := raw[:gcmNonceSize]
	ciphertext := raw[gcmNonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt payload: %w", err)
	}
	return string(plaintext), nil
}

func deriveKey(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}
