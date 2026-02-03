package store

import (
	"fmt"
	"time"

	vessel "github.com/vessel/vessel/internal"
	bolt "go.etcd.io/bbolt"
)

// CreateApp creates a new app in the store.
func (s *BoltStore) CreateApp(app *App) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketApps)

		// Check if app already exists
		if b.Get([]byte(app.Name)) != nil {
			return vessel.ErrAppAlreadyExists
		}

		// Set timestamps
		now := time.Now().UTC()
		app.CreatedAt = now
		app.UpdatedAt = now

		// Encode and store
		data, err := encode(app)
		if err != nil {
			return fmt.Errorf("failed to encode app: %w", err)
		}

		if err := b.Put([]byte(app.Name), data); err != nil {
			return fmt.Errorf("failed to store app: %w", err)
		}

		return nil
	})
}

// GetApp retrieves an app by name.
func (s *BoltStore) GetApp(name string) (*App, error) {
	var app App

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketApps)
		data := b.Get([]byte(name))
		if data == nil {
			return vessel.ErrAppNotFound
		}

		if err := decode(data, &app); err != nil {
			return fmt.Errorf("failed to decode app: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &app, nil
}

// ListApps returns all apps in the store.
func (s *BoltStore) ListApps() ([]*App, error) {
	var apps []*App

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketApps)

		return b.ForEach(func(k, v []byte) error {
			var app App
			if err := decode(v, &app); err != nil {
				return fmt.Errorf("failed to decode app %s: %w", k, err)
			}
			apps = append(apps, &app)
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return apps, nil
}

// UpdateApp updates an existing app in the store.
func (s *BoltStore) UpdateApp(app *App) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketApps)

		// Check if app exists
		if b.Get([]byte(app.Name)) == nil {
			return vessel.ErrAppNotFound
		}

		// Update timestamp
		app.UpdatedAt = time.Now().UTC()

		// Encode and store
		data, err := encode(app)
		if err != nil {
			return fmt.Errorf("failed to encode app: %w", err)
		}

		if err := b.Put([]byte(app.Name), data); err != nil {
			return fmt.Errorf("failed to store app: %w", err)
		}

		return nil
	})
}

// DeleteApp removes an app from the store.
func (s *BoltStore) DeleteApp(name string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketApps)

		// Check if app exists
		if b.Get([]byte(name)) == nil {
			return vessel.ErrAppNotFound
		}

		if err := b.Delete([]byte(name)); err != nil {
			return fmt.Errorf("failed to delete app: %w", err)
		}

		return nil
	})
}
