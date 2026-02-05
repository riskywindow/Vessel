package store

import (
	"fmt"
	"sort"
	"time"

	vessel "github.com/vessel/vessel/internal"
	bolt "go.etcd.io/bbolt"
)

// CreateDeploy creates a new deploy record in the store.
func (s *BoltStore) CreateDeploy(d *Deploy) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDeploys)

		// Check if deploy already exists
		if b.Get([]byte(d.ID)) != nil {
			return fmt.Errorf("deploy %s already exists", d.ID)
		}

		// Set timestamp
		d.CreatedAt = time.Now().UTC()

		// Encode and store
		data, err := encode(d)
		if err != nil {
			return fmt.Errorf("failed to encode deploy: %w", err)
		}

		if err := b.Put([]byte(d.ID), data); err != nil {
			return fmt.Errorf("failed to store deploy: %w", err)
		}

		return nil
	})
}

// GetDeploy retrieves a deploy by ID.
func (s *BoltStore) GetDeploy(id string) (*Deploy, error) {
	var deploy Deploy

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDeploys)
		data := b.Get([]byte(id))
		if data == nil {
			return vessel.ErrVersionNotFound
		}

		if err := decode(data, &deploy); err != nil {
			return fmt.Errorf("failed to decode deploy: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &deploy, nil
}

// ListDeploysByApp returns all deploys for a given app, sorted by version descending.
func (s *BoltStore) ListDeploysByApp(appID string) ([]*Deploy, error) {
	var deploys []*Deploy

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDeploys)

		return b.ForEach(func(k, v []byte) error {
			var deploy Deploy
			if err := decode(v, &deploy); err != nil {
				return fmt.Errorf("failed to decode deploy %s: %w", k, err)
			}
			if deploy.AppID == appID {
				deploys = append(deploys, &deploy)
			}
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	// Sort by version descending (newest first)
	sort.Slice(deploys, func(i, j int) bool {
		return deploys[i].Version > deploys[j].Version
	})

	return deploys, nil
}

// GetLatestDeploy returns the most recent deploy for an app.
func (s *BoltStore) GetLatestDeploy(appID string) (*Deploy, error) {
	deploys, err := s.ListDeploysByApp(appID)
	if err != nil {
		return nil, err
	}

	if len(deploys) == 0 {
		return nil, vessel.ErrNoDeployHistory
	}

	return deploys[0], nil
}

// UpdateDeploy updates an existing deploy in the store.
func (s *BoltStore) UpdateDeploy(d *Deploy) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDeploys)

		// Check if deploy exists
		if b.Get([]byte(d.ID)) == nil {
			return vessel.ErrVersionNotFound
		}

		// Encode and store
		data, err := encode(d)
		if err != nil {
			return fmt.Errorf("failed to encode deploy: %w", err)
		}

		if err := b.Put([]byte(d.ID), data); err != nil {
			return fmt.Errorf("failed to store deploy: %w", err)
		}

		return nil
	})
}

// GetNextDeployVersion returns the next version number for an app.
func (s *BoltStore) GetNextDeployVersion(appID string) (int, error) {
	latest, err := s.GetLatestDeploy(appID)
	if err != nil {
		if err == vessel.ErrNoDeployHistory {
			return 1, nil
		}
		return 0, err
	}
	return latest.Version + 1, nil
}

// GetDeployByVersion returns a specific deploy by app ID and version number.
func (s *BoltStore) GetDeployByVersion(appID string, version int) (*Deploy, error) {
	deploys, err := s.ListDeploysByApp(appID)
	if err != nil {
		return nil, err
	}

	for _, d := range deploys {
		if d.Version == version {
			return d, nil
		}
	}

	return nil, vessel.ErrVersionNotFound
}

// DeleteDeploy removes a deploy record by ID.
func (s *BoltStore) DeleteDeploy(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDeploys)
		if b.Get([]byte(id)) == nil {
			return vessel.ErrVersionNotFound
		}
		return b.Delete([]byte(id))
	})
}

// PruneDeployHistory removes old deploys beyond the retention limit for an app.
func (s *BoltStore) PruneDeployHistory(appID string, keepN int) error {
	if keepN <= 0 {
		return nil
	}

	deploys, err := s.ListDeploysByApp(appID)
	if err != nil {
		return err
	}

	if len(deploys) <= keepN {
		return nil
	}

	// deploys is sorted newest-first; prune from the end
	for _, d := range deploys[keepN:] {
		if err := s.DeleteDeploy(d.ID); err != nil {
			return fmt.Errorf("failed to prune deploy %s: %w", d.ID, err)
		}
	}

	return nil
}
