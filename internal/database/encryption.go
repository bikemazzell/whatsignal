package database

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"whatsignal/internal/models"

	"golang.org/x/crypto/pbkdf2"
)

type encryptor struct {
	gcm cipher.AEAD
}

func NewEncryptor() (*encryptor, error) {
	key, err := deriveKey()
	if err != nil {
		return nil, fmt.Errorf("failed to derive encryption key: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	return &encryptor{gcm: gcm}, nil
}

func (e *encryptor) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	nonce := make([]byte, models.NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := e.gcm.Seal(nil, nonce, []byte(plaintext), nil)
	// Prepend nonce to ciphertext for storage
	result := append(nonce, ciphertext...)
	return base64.StdEncoding.EncodeToString(result), nil
}

func (e *encryptor) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	if len(data) < models.NonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext_bytes := data[:models.NonceSize], data[models.NonceSize:]
	plaintext, err := e.gcm.Open(nil, nonce, ciphertext_bytes, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

func deriveKey() ([]byte, error) {
	secret := os.Getenv("WHATSIGNAL_ENCRYPTION_SECRET")
	if secret == "" {
		secret = "whatsignal-default-secret-change-in-production"
	}

	salt := []byte("whatsignal-salt-v1")

	key := pbkdf2.Key([]byte(secret), salt, models.Iterations, models.KeySize, sha256.New)
	return key, nil
}

func (e *encryptor) EncryptIfEnabled(plaintext string) (string, error) {
	if !isEncryptionEnabled() {
		return plaintext, nil
	}
	return e.Encrypt(plaintext)
}

func (e *encryptor) DecryptIfEnabled(ciphertext string) (string, error) {
	if !isEncryptionEnabled() {
		return ciphertext, nil
	}
	return e.Decrypt(ciphertext)
}

func isEncryptionEnabled() bool {
	return os.Getenv("WHATSIGNAL_ENABLE_ENCRYPTION") == "true"
}
