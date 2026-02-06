package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters for key derivation.
const (
	argon2Time    = 1
	argon2Memory  = 64 * 1024 // 64 MB
	argon2Threads = 4
	argon2KeyLen  = 32 // AES-256
	saltLen       = 16
)

// ErrDecryptionFailed is returned when decryption fails (wrong password or corrupted data).
var ErrDecryptionFailed = errors.New("decryption failed: wrong master password or corrupted data")

// GenerateSalt creates a cryptographically random salt for Argon2id.
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	return salt, nil
}

// DeriveKey derives an AES-256 key from a master password using Argon2id.
func DeriveKey(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
}

// Encrypt encrypts plaintext using AES-256-GCM with the provided key.
func Encrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Nonce is prepended to the ciphertext
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt decrypts ciphertext using AES-256-GCM with the provided key.
func Decrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrDecryptionFailed
	}

	nonce, data := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// SecretManager wraps the Store's secret operations with encryption.
// It holds the derived encryption key in memory.
type SecretManager struct {
	store Store
	key   []byte
	salt  []byte
}

// NewSecretManager creates a SecretManager with the given master password and salt.
// If salt is nil, a new salt is generated (first-time setup).
func NewSecretManager(st Store, masterPassword string, salt []byte) (*SecretManager, error) {
	if salt == nil {
		var err error
		salt, err = GenerateSalt()
		if err != nil {
			return nil, err
		}
	}

	key := DeriveKey(masterPassword, salt)

	return &SecretManager{
		store: st,
		key:   key,
		salt:  salt,
	}, nil
}

// Salt returns the salt used for key derivation.
func (sm *SecretManager) Salt() []byte {
	return sm.salt
}

// SetSecret encrypts and stores a secret.
func (sm *SecretManager) SetSecret(secretKey string, value string) error {
	encrypted, err := Encrypt(sm.key, []byte(value))
	if err != nil {
		return fmt.Errorf("failed to encrypt secret: %w", err)
	}
	return sm.store.SetSecret(secretKey, encrypted)
}

// GetSecret retrieves and decrypts a secret.
func (sm *SecretManager) GetSecret(secretKey string) (string, error) {
	secret, err := sm.store.GetSecret(secretKey)
	if err != nil {
		return "", err
	}

	plaintext, err := Decrypt(sm.key, secret.Value)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// ListSecretKeys returns all secret key names (not values).
func (sm *SecretManager) ListSecretKeys() ([]string, error) {
	return sm.store.ListSecretKeys()
}

// DeleteSecret removes a secret from the store.
func (sm *SecretManager) DeleteSecret(secretKey string) error {
	return sm.store.DeleteSecret(secretKey)
}

// GetSecretMetadata returns a secret's metadata (timestamps) without decrypting.
func (sm *SecretManager) GetSecretMetadata(secretKey string) (*Secret, error) {
	return sm.store.GetSecret(secretKey)
}
