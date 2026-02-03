package store

import (
	"fmt"
	"time"

	vessel "github.com/vessel/vessel/internal"
	bolt "go.etcd.io/bbolt"
)

// secretStorage is an internal type for storing secrets in the database.
// This is separate from Secret because Secret.Value has json:"-" to prevent
// accidental exposure in API responses, but we need to persist it in the store.
type secretStorage struct {
	Key       string    `json:"key"`
	Value     []byte    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SetSecret stores or updates an encrypted secret.
func (s *BoltStore) SetSecret(key string, encryptedValue []byte) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketSecrets)

		now := time.Now().UTC()
		secret := &secretStorage{
			Key:       key,
			Value:     encryptedValue,
			UpdatedAt: now,
		}

		// Check if this is an update or create
		existing := b.Get([]byte(key))
		if existing == nil {
			secret.CreatedAt = now
		} else {
			var old secretStorage
			if err := decode(existing, &old); err == nil {
				secret.CreatedAt = old.CreatedAt
			} else {
				secret.CreatedAt = now
			}
		}

		// Encode and store
		data, err := encode(secret)
		if err != nil {
			return fmt.Errorf("failed to encode secret: %w", err)
		}

		if err := b.Put([]byte(key), data); err != nil {
			return fmt.Errorf("failed to store secret: %w", err)
		}

		return nil
	})
}

// GetSecret retrieves a secret by key.
func (s *BoltStore) GetSecret(key string) (*Secret, error) {
	var stored secretStorage

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketSecrets)
		data := b.Get([]byte(key))
		if data == nil {
			return vessel.ErrSecretNotFound
		}

		if err := decode(data, &stored); err != nil {
			return fmt.Errorf("failed to decode secret: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Convert from storage type to public type
	return &Secret{
		Key:       stored.Key,
		Value:     stored.Value,
		CreatedAt: stored.CreatedAt,
		UpdatedAt: stored.UpdatedAt,
	}, nil
}

// ListSecretKeys returns all secret keys (not values).
func (s *BoltStore) ListSecretKeys() ([]string, error) {
	var keys []string

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketSecrets)

		return b.ForEach(func(k, v []byte) error {
			keys = append(keys, string(k))
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return keys, nil
}

// DeleteSecret removes a secret from the store.
func (s *BoltStore) DeleteSecret(key string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketSecrets)

		// Check if secret exists
		if b.Get([]byte(key)) == nil {
			return vessel.ErrSecretNotFound
		}

		if err := b.Delete([]byte(key)); err != nil {
			return fmt.Errorf("failed to delete secret: %w", err)
		}

		return nil
	})
}
