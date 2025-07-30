package database

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"whatsignal/internal/constants"
	"whatsignal/internal/models"

	"golang.org/x/crypto/pbkdf2"
)

type encryptor struct {
	gcm cipher.AEAD
}

func NewEncryptor() (*encryptor, error) {
	// If encryption is disabled, return a nil encryptor
	if !isEncryptionEnabled() {
		return &encryptor{gcm: nil}, nil
	}

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
	if plaintext == "" || e.gcm == nil {
		return plaintext, nil
	}

	nonce := make([]byte, models.NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := e.gcm.Seal(nil, nonce, []byte(plaintext), nil) // #nosec G407 - Deterministic nonce required for searchable encryption
	// Prepend nonce to ciphertext for storage
	result := append(nonce, ciphertext...)
	return base64.StdEncoding.EncodeToString(result), nil
}

func (e *encryptor) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" || e.gcm == nil {
		return ciphertext, nil
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
		return nil, fmt.Errorf("WHATSIGNAL_ENCRYPTION_SECRET environment variable is required when encryption is enabled")
	}

	// Validate secret strength
	if len(secret) < 32 {
		return nil, fmt.Errorf("encryption secret must be at least 32 characters long")
	}

	salt := []byte(constants.EncryptionSalt)

	key := pbkdf2.Key([]byte(secret), salt, models.Iterations, models.KeySize, sha256.New)
	return key, nil
}

// EncryptForLookup creates deterministic encryption for database lookups
// Uses a deterministic nonce derived from the plaintext for consistent results
// This is intentionally deterministic to enable encrypted database searches
// #nosec G407 - Deterministic encryption is required for searchable encryption
func (e *encryptor) EncryptForLookup(plaintext string) (string, error) {
	if plaintext == "" || e.gcm == nil {
		return plaintext, nil
	}

	// Create deterministic nonce from plaintext hash with application-specific salt
	// This ensures the same plaintext always produces the same ciphertext for database lookups
	lookupSalt := constants.EncryptionLookupSalt
	hash := sha256.Sum256([]byte(plaintext + lookupSalt))
	nonce := hash[:models.NonceSize]

	// #nosec G407 - Using deterministic nonce for searchable encryption by design
	ciphertext := e.gcm.Seal(nil, nonce, []byte(plaintext), nil)
	// Prepend nonce to ciphertext for consistency
	result := append(nonce, ciphertext...)
	return base64.StdEncoding.EncodeToString(result), nil
}

func (e *encryptor) EncryptIfEnabled(plaintext string) (string, error) {
	if !isEncryptionEnabled() {
		return plaintext, nil
	}
	return e.Encrypt(plaintext)
}

// EncryptForLookupIfEnabled encrypts with deterministic method for database lookups
func (e *encryptor) EncryptForLookupIfEnabled(plaintext string) (string, error) {
	if !isEncryptionEnabled() {
		return plaintext, nil
	}
	return e.EncryptForLookup(plaintext)
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
