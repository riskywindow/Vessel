package store

import (
	"fmt"
	"time"

	vessel "github.com/vessel/vessel/internal"
	bolt "go.etcd.io/bbolt"
)

// CreateContainer creates a new container in the store.
func (s *BoltStore) CreateContainer(c *Container) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketContainers)

		// Check if container already exists
		if b.Get([]byte(c.ID)) != nil {
			return fmt.Errorf("container %s already exists", c.ID)
		}

		// Set timestamp
		c.CreatedAt = time.Now().UTC()

		// Encode and store
		data, err := encode(c)
		if err != nil {
			return fmt.Errorf("failed to encode container: %w", err)
		}

		if err := b.Put([]byte(c.ID), data); err != nil {
			return fmt.Errorf("failed to store container: %w", err)
		}

		return nil
	})
}

// GetContainer retrieves a container by ID.
func (s *BoltStore) GetContainer(id string) (*Container, error) {
	var container Container

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketContainers)
		data := b.Get([]byte(id))
		if data == nil {
			return vessel.ErrContainerNotFound
		}

		if err := decode(data, &container); err != nil {
			return fmt.Errorf("failed to decode container: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &container, nil
}

// ListContainersByApp returns all containers for a given app.
func (s *BoltStore) ListContainersByApp(appID string) ([]*Container, error) {
	var containers []*Container

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketContainers)

		return b.ForEach(func(k, v []byte) error {
			var container Container
			if err := decode(v, &container); err != nil {
				return fmt.Errorf("failed to decode container %s: %w", k, err)
			}
			if container.AppID == appID {
				containers = append(containers, &container)
			}
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return containers, nil
}

// UpdateContainer updates an existing container in the store.
func (s *BoltStore) UpdateContainer(c *Container) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketContainers)

		// Check if container exists
		if b.Get([]byte(c.ID)) == nil {
			return vessel.ErrContainerNotFound
		}

		// Encode and store
		data, err := encode(c)
		if err != nil {
			return fmt.Errorf("failed to encode container: %w", err)
		}

		if err := b.Put([]byte(c.ID), data); err != nil {
			return fmt.Errorf("failed to store container: %w", err)
		}

		return nil
	})
}

// DeleteContainer removes a container from the store.
func (s *BoltStore) DeleteContainer(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketContainers)

		// Check if container exists
		if b.Get([]byte(id)) == nil {
			return vessel.ErrContainerNotFound
		}

		if err := b.Delete([]byte(id)); err != nil {
			return fmt.Errorf("failed to delete container: %w", err)
		}

		return nil
	})
}
