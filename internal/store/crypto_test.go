package store

import (
	"path/filepath"
	"testing"
)

func TestGenerateSalt(t *testing.T) {
	salt1, err := GenerateSalt()
	if err != nil {
		t.Fatalf("failed to generate salt: %v", err)
	}
	if len(salt1) != saltLen {
		t.Errorf("expected salt length %d, got %d", saltLen, len(salt1))
	}

	// Two salts should be different
	salt2, err := GenerateSalt()
	if err != nil {
		t.Fatalf("failed to generate second salt: %v", err)
	}

	if string(salt1) == string(salt2) {
		t.Error("two generated salts should be different")
	}
}

func TestDeriveKey(t *testing.T) {
	salt, _ := GenerateSalt()

	key1 := DeriveKey("password123", salt)
	if len(key1) != argon2KeyLen {
		t.Errorf("expected key length %d, got %d", argon2KeyLen, len(key1))
	}

	// Same password + salt = same key
	key2 := DeriveKey("password123", salt)
	if string(key1) != string(key2) {
		t.Error("same password and salt should produce same key")
	}

	// Different password = different key
	key3 := DeriveKey("different", salt)
	if string(key1) == string(key3) {
		t.Error("different passwords should produce different keys")
	}
}

func TestEncryptDecrypt_Roundtrip(t *testing.T) {
	salt, _ := GenerateSalt()
	key := DeriveKey("test-password", salt)

	tests := []struct {
		name      string
		plaintext string
	}{
		{"simple text", "hello world"},
		{"empty string", ""},
		{"special chars", "p@$$w0rd!#%^&*()"},
		{"long value", "this is a much longer value that contains lots of text to make sure encryption works for longer strings"},
		{"json", `{"key": "value", "nested": {"a": 1}}`},
		{"url", "postgres://user:pass@host:5432/db?sslmode=disable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext, err := Encrypt(key, []byte(tt.plaintext))
			if err != nil {
				t.Fatalf("Encrypt failed: %v", err)
			}

			// Ciphertext should be different from plaintext
			if tt.plaintext != "" && string(ciphertext) == tt.plaintext {
				t.Error("ciphertext should differ from plaintext")
			}

			plaintext, err := Decrypt(key, ciphertext)
			if err != nil {
				t.Fatalf("Decrypt failed: %v", err)
			}

			if string(plaintext) != tt.plaintext {
				t.Errorf("roundtrip failed: expected %q, got %q", tt.plaintext, plaintext)
			}
		})
	}
}

func TestEncryptDecrypt_DifferentCiphertext(t *testing.T) {
	salt, _ := GenerateSalt()
	key := DeriveKey("test-password", salt)
	plaintext := []byte("same data")

	ct1, _ := Encrypt(key, plaintext)
	ct2, _ := Encrypt(key, plaintext)

	// Two encryptions of the same data should produce different ciphertext (random nonce)
	if string(ct1) == string(ct2) {
		t.Error("two encryptions of the same data should produce different ciphertext")
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	salt, _ := GenerateSalt()
	key1 := DeriveKey("correct-password", salt)
	key2 := DeriveKey("wrong-password", salt)

	ciphertext, err := Encrypt(key1, []byte("secret data"))
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = Decrypt(key2, ciphertext)
	if err == nil {
		t.Error("expected error when decrypting with wrong key")
	}
	if err != ErrDecryptionFailed {
		t.Errorf("expected ErrDecryptionFailed, got %v", err)
	}
}

func TestDecrypt_InvalidCiphertext(t *testing.T) {
	salt, _ := GenerateSalt()
	key := DeriveKey("password", salt)

	// Too short
	_, err := Decrypt(key, []byte("short"))
	if err != ErrDecryptionFailed {
		t.Errorf("expected ErrDecryptionFailed for short ciphertext, got %v", err)
	}

	// Corrupted
	ciphertext, _ := Encrypt(key, []byte("data"))
	ciphertext[len(ciphertext)-1] ^= 0xFF // flip last byte
	_, err = Decrypt(key, ciphertext)
	if err != ErrDecryptionFailed {
		t.Errorf("expected ErrDecryptionFailed for corrupted ciphertext, got %v", err)
	}
}

func TestSecretManager_SetGetDelete(t *testing.T) {
	st := setupTestStore(t)

	sm, err := NewSecretManager(st, "master-password", nil)
	if err != nil {
		t.Fatalf("failed to create SecretManager: %v", err)
	}

	// Set a secret
	if err := sm.SetSecret("api-key", "sk-test-12345"); err != nil {
		t.Fatalf("SetSecret failed: %v", err)
	}

	// Get the secret
	value, err := sm.GetSecret("api-key")
	if err != nil {
		t.Fatalf("GetSecret failed: %v", err)
	}
	if value != "sk-test-12345" {
		t.Errorf("expected 'sk-test-12345', got %q", value)
	}

	// List keys
	keys, err := sm.ListSecretKeys()
	if err != nil {
		t.Fatalf("ListSecretKeys failed: %v", err)
	}
	if len(keys) != 1 || keys[0] != "api-key" {
		t.Errorf("expected [api-key], got %v", keys)
	}

	// Delete
	if err := sm.DeleteSecret("api-key"); err != nil {
		t.Fatalf("DeleteSecret failed: %v", err)
	}

	// Verify deleted
	_, err = sm.GetSecret("api-key")
	if err == nil {
		t.Error("expected error after deletion")
	}
}

func TestSecretManager_WrongMasterPassword(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	st, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer st.Close()

	salt, _ := GenerateSalt()

	// Set with correct password
	sm1, _ := NewSecretManager(st, "correct-password", salt)
	if err := sm1.SetSecret("key", "secret-value"); err != nil {
		t.Fatalf("SetSecret failed: %v", err)
	}

	// Try to get with wrong password
	sm2, _ := NewSecretManager(st, "wrong-password", salt)
	_, err = sm2.GetSecret("key")
	if err == nil {
		t.Error("expected error when getting with wrong password")
	}
}

func TestSecretManager_UpdateSecret(t *testing.T) {
	st := setupTestStore(t)

	sm, err := NewSecretManager(st, "password", nil)
	if err != nil {
		t.Fatalf("failed to create SecretManager: %v", err)
	}

	// Set initial value
	sm.SetSecret("key", "value1")

	// Update
	sm.SetSecret("key", "value2")

	// Get should return updated value
	value, err := sm.GetSecret("key")
	if err != nil {
		t.Fatalf("GetSecret failed: %v", err)
	}
	if value != "value2" {
		t.Errorf("expected 'value2', got %q", value)
	}
}

func TestSecretManager_MultipleSecrets(t *testing.T) {
	st := setupTestStore(t)

	sm, err := NewSecretManager(st, "password", nil)
	if err != nil {
		t.Fatalf("failed to create SecretManager: %v", err)
	}

	secrets := map[string]string{
		"db-url":  "postgres://localhost/db",
		"api-key": "sk-12345",
		"jwt":     "eyJhbGciOiJIUzI1NiJ9...",
	}

	for k, v := range secrets {
		if err := sm.SetSecret(k, v); err != nil {
			t.Fatalf("SetSecret(%s) failed: %v", k, err)
		}
	}

	// Verify all can be retrieved
	for k, expected := range secrets {
		value, err := sm.GetSecret(k)
		if err != nil {
			t.Fatalf("GetSecret(%s) failed: %v", k, err)
		}
		if value != expected {
			t.Errorf("GetSecret(%s): expected %q, got %q", k, expected, value)
		}
	}

	// List should return all keys
	keys, err := sm.ListSecretKeys()
	if err != nil {
		t.Fatalf("ListSecretKeys failed: %v", err)
	}
	if len(keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(keys))
	}
}

func TestSecretManager_GetMetadata(t *testing.T) {
	st := setupTestStore(t)

	sm, err := NewSecretManager(st, "password", nil)
	if err != nil {
		t.Fatalf("failed to create SecretManager: %v", err)
	}

	sm.SetSecret("key", "value")

	meta, err := sm.GetSecretMetadata("key")
	if err != nil {
		t.Fatalf("GetSecretMetadata failed: %v", err)
	}

	if meta.Key != "key" {
		t.Errorf("expected key 'key', got %q", meta.Key)
	}
	if meta.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if meta.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestSecretManager_SaltPersistence(t *testing.T) {
	st := setupTestStore(t)

	// Create with auto-generated salt
	sm1, err := NewSecretManager(st, "password", nil)
	if err != nil {
		t.Fatalf("failed to create SecretManager: %v", err)
	}

	sm1.SetSecret("key", "value")
	salt := sm1.Salt()

	// Create new manager with same salt and password
	sm2, err := NewSecretManager(st, "password", salt)
	if err != nil {
		t.Fatalf("failed to create SecretManager: %v", err)
	}

	value, err := sm2.GetSecret("key")
	if err != nil {
		t.Fatalf("GetSecret failed: %v", err)
	}
	if value != "value" {
		t.Errorf("expected 'value', got %q", value)
	}
}
