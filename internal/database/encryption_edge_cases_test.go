package database

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"strings"
	"sync"
	"testing"
	"whatsignal/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEncryptor_VeryLargeData tests encryption with very large data
func TestEncryptor_VeryLargeData(t *testing.T) {
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

	encryptor, err := NewEncryptor()
	require.NoError(t, err)

	// Test with 10MB of data
	largeData := strings.Repeat("A", 10*1024*1024)
	
	ciphertext, err := encryptor.Encrypt(largeData)
	require.NoError(t, err)
	
	decrypted, err := encryptor.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, largeData, decrypted)
}

// TestEncryptor_BinaryData tests encryption with binary data
func TestEncryptor_BinaryData(t *testing.T) {
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

	encryptor, err := NewEncryptor()
	require.NoError(t, err)

	// Create random binary data
	binaryData := make([]byte, 1024)
	_, err = rand.Read(binaryData)
	require.NoError(t, err)

	// Convert to string for encryption
	plaintext := string(binaryData)
	
	ciphertext, err := encryptor.Encrypt(plaintext)
	require.NoError(t, err)
	
	decrypted, err := encryptor.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

// TestEncryptor_ConcurrentAccess tests thread safety
func TestEncryptor_ConcurrentAccess(t *testing.T) {
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

	encryptor, err := NewEncryptor()
	require.NoError(t, err)

	const numGoroutines = 100
	const numOperations = 10

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numOperations*2)

	// Concurrent encryption and decryption
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				plaintext := strings.Repeat("test", id+j+1)
				
				// Encrypt
				ciphertext, err := encryptor.Encrypt(plaintext)
				if err != nil {
					errors <- err
					continue
				}
				
				// Decrypt
				decrypted, err := encryptor.Decrypt(ciphertext)
				if err != nil {
					errors <- err
					continue
				}
				
				if decrypted != plaintext {
					errors <- assert.AnError
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		require.NoError(t, err)
	}
}

// TestEncryptor_InvalidKeySize tests with invalid key sizes
func TestEncryptor_InvalidKeySize(t *testing.T) {
	// Test with key too short
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "short")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

	_, err := NewEncryptor()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least 32 characters")
}

// TestEncryptor_EncryptForLookupConsistency tests deterministic encryption
func TestEncryptor_EncryptForLookupConsistency(t *testing.T) {
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

	encryptor, err := NewEncryptor()
	require.NoError(t, err)

	plaintext := "test@example.com"
	
	// Encrypt multiple times
	results := make([]string, 10)
	for i := 0; i < 10; i++ {
		encrypted, err := encryptor.EncryptForLookup(plaintext)
		require.NoError(t, err)
		results[i] = encrypted
	}

	// All results should be identical
	for i := 1; i < len(results); i++ {
		assert.Equal(t, results[0], results[i], "EncryptForLookup should produce consistent results")
	}
}

// TestEncryptor_MalformedBase64 tests decryption with malformed base64
func TestEncryptor_MalformedBase64(t *testing.T) {
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

	encryptor, err := NewEncryptor()
	require.NoError(t, err)

	testCases := []struct {
		name       string
		ciphertext string
	}{
		{
			name:       "invalid characters",
			ciphertext: "!@#$%^&*()",
		},
		{
			name:       "incomplete base64",
			ciphertext: "dGVzdA",
		},
		{
			name:       "valid base64 but too short for nonce",
			ciphertext: base64.StdEncoding.EncodeToString([]byte("short")),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := encryptor.Decrypt(tc.ciphertext)
			require.Error(t, err)
		})
	}
}

// TestEncryptor_ModifiedCiphertext tests tamper detection
func TestEncryptor_ModifiedCiphertext(t *testing.T) {
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

	encryptor, err := NewEncryptor()
	require.NoError(t, err)

	plaintext := "sensitive data"
	ciphertext, err := encryptor.Encrypt(plaintext)
	require.NoError(t, err)

	// Decode, modify, and re-encode
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	require.NoError(t, err)
	
	// Flip a bit in the ciphertext (after the nonce)
	if len(data) > models.NonceSize {
		data[models.NonceSize] ^= 0x01
	}
	
	modifiedCiphertext := base64.StdEncoding.EncodeToString(data)
	
	// Decryption should fail due to authentication
	_, err = encryptor.Decrypt(modifiedCiphertext)
	require.Error(t, err)
}

// TestEncryptor_NullBytes tests handling of null bytes
func TestEncryptor_NullBytes(t *testing.T) {
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

	encryptor, err := NewEncryptor()
	require.NoError(t, err)

	// Test with null bytes
	plaintext := "before\x00middle\x00after"
	
	ciphertext, err := encryptor.Encrypt(plaintext)
	require.NoError(t, err)
	
	decrypted, err := encryptor.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

// TestEncryptor_KeyDerivationDeterminism tests key derivation consistency
func TestEncryptor_KeyDerivationDeterminism(t *testing.T) {
	secret := "this-is-a-very-long-test-secret-key-for-encryption-testing"
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", secret)
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

	// Derive key multiple times
	keys := make([][]byte, 5)
	for i := 0; i < 5; i++ {
		key, err := deriveKey()
		require.NoError(t, err)
		keys[i] = key
	}

	// All keys should be identical
	for i := 1; i < len(keys); i++ {
		assert.Equal(t, keys[0], keys[i], "Key derivation should be deterministic")
	}
}