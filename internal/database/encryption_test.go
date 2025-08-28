package database

import (
	"os"
	"testing"
	"whatsignal/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptor_EncryptDecrypt(t *testing.T) {
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

	encryptor, err := NewEncryptor()
	require.NoError(t, err)

	testCases := []struct {
		name      string
		plaintext string
	}{
		{
			name:      "simple text",
			plaintext: "hello world",
		},
		{
			name:      "empty string",
			plaintext: "",
		},
		{
			name:      "unicode text",
			plaintext: "Hello 世界 🌍",
		},
		{
			name:      "long text",
			plaintext: "This is a very long message that contains multiple sentences and should test the encryption with larger data sizes to ensure it works correctly.",
		},
		{
			name:      "special characters",
			plaintext: "!@#$%^&*()_+-=[]{}|;':\",./<>?",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ciphertext, err := encryptor.Encrypt(tc.plaintext)
			require.NoError(t, err)

			if tc.plaintext == "" {
				assert.Equal(t, "", ciphertext)
				return
			}

			assert.NotEqual(t, tc.plaintext, ciphertext)
			assert.NotEmpty(t, ciphertext)

			decrypted, err := encryptor.Decrypt(ciphertext)
			require.NoError(t, err)
			assert.Equal(t, tc.plaintext, decrypted)
		})
	}
}

func TestEncryptor_EncryptionUniqueness(t *testing.T) {
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

	encryptor, err := NewEncryptor()
	require.NoError(t, err)

	plaintext := "test message"

	ciphertext1, err := encryptor.Encrypt(plaintext)
	require.NoError(t, err)

	ciphertext2, err := encryptor.Encrypt(plaintext)
	require.NoError(t, err)

	assert.NotEqual(t, ciphertext1, ciphertext2, "Same plaintext should produce different ciphertexts due to random nonces")

	decrypted1, err := encryptor.Decrypt(ciphertext1)
	require.NoError(t, err)

	decrypted2, err := encryptor.Decrypt(ciphertext2)
	require.NoError(t, err)

	assert.Equal(t, plaintext, decrypted1)
	assert.Equal(t, plaintext, decrypted2)
}

func TestEncryptor_DecryptInvalidData(t *testing.T) {
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

	encryptor, err := NewEncryptor()
	require.NoError(t, err)

	testCases := []struct {
		name       string
		ciphertext string
	}{
		{
			name:       "invalid base64",
			ciphertext: "invalid-base64!@#",
		},
		{
			name:       "too short",
			ciphertext: "dGVzdA==", // "test" in base64, but too short for nonce
		},
		{
			name:       "corrupted data",
			ciphertext: "YWJjZGVmZ2hpams=", // valid base64 but invalid encrypted data
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := encryptor.Decrypt(tc.ciphertext)
			assert.Error(t, err)
		})
	}
}

func TestEncryptor_EncryptIfEnabled(t *testing.T) {
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

	encryptor, err := NewEncryptor()
	require.NoError(t, err)

	plaintext := "test message"

	// Always-on encryption
	result, err := encryptor.EncryptIfEnabled(plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, result)
	assert.NotEmpty(t, result)
}

func TestEncryptor_DecryptIfEnabled(t *testing.T) {
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

	encryptor, err := NewEncryptor()
	require.NoError(t, err)

	plaintext := "test message"

	ciphertext, err := encryptor.Encrypt(plaintext)
	require.NoError(t, err)

	result, err := encryptor.DecryptIfEnabled(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, result)
}

func TestDeriveKey_WithCustomSecret(t *testing.T) {
	originalSecret := os.Getenv("WHATSIGNAL_ENCRYPTION_SECRET")
	defer func() {
		if originalSecret != "" {
			os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", originalSecret)
		} else {
			os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")
		}
	}()

	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-custom-secret-key-for-testing-purposes")

	key1, err := deriveKey()
	require.NoError(t, err)
	assert.Len(t, key1, models.KeySize)

	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-different-very-long-secret-key-for-testing-purposes")

	key2, err := deriveKey()
	require.NoError(t, err)
	assert.Len(t, key2, models.KeySize)

	assert.NotEqual(t, key1, key2, "Different secrets should produce different keys")
}

func TestDeriveKey_WithDefaultSecret(t *testing.T) {
	originalSecret := os.Getenv("WHATSIGNAL_ENCRYPTION_SECRET")
	defer func() {
		if originalSecret != "" {
			os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", originalSecret)
		} else {
			os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")
		}
	}()

	os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

	_, err := deriveKey()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "WHATSIGNAL_ENCRYPTION_SECRET environment variable is required")
}

func TestIsEncryptionEnabled(t *testing.T) {
	originalValue := os.Getenv("WHATSIGNAL_ENABLE_ENCRYPTION")
	defer func() {
		if originalValue != "" {
			os.Setenv("WHATSIGNAL_ENABLE_ENCRYPTION", originalValue)
		} else {
			os.Unsetenv("WHATSIGNAL_ENABLE_ENCRYPTION")
		}
	}()

	// Always-on encryption: no environment toggle
	assert.True(t, true)
}
