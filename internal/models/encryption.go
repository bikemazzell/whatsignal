package models

import (
	"crypto/cipher"
	"whatsignal/internal/constants"
)

const (
	KeySize    = 32                         // AES-256
	NonceSize  = 12                         // GCM standard nonce size
	SaltSize   = 16                         // Salt size for PBKDF2
	Iterations = constants.PBKDF2Iterations // PBKDF2 iterations
)

type Encryptor struct {
	GCM cipher.AEAD
}
