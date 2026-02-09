package store

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	bolt "go.etcd.io/bbolt"
)

// CreateHealthResult persists a health check result.
// Results are keyed by "containerID:RFC3339Nano" for range scanning.
func (s *BoltStore) CreateHealthResult(result *HealthResult) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucketHealthResults)
		if err != nil {
			return fmt.Errorf("failed to create health_results bucket: %w", err)
		}

		key := fmt.Sprintf("%s:%s", result.ContainerID, result.CheckedAt.UTC().Format("2006-01-02T15:04:05.000000000Z"))
		data, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("failed to marshal health result: %w", err)
		}

		return b.Put([]byte(key), data)
	})
}

// GetHealthResults returns the most recent health results for a container.
// Results are returned in reverse chronological order, limited by the limit parameter.
func (s *BoltStore) GetHealthResults(containerID string, limit int) ([]*HealthResult, error) {
	var results []*HealthResult

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketHealthResults)
		if b == nil {
			return nil
		}

		prefix := []byte(containerID + ":")
		c := b.Cursor()

		// Collect all matching keys
		var keys []string
		for k, _ := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, _ = c.Next() {
			keys = append(keys, string(k))
		}

		// Sort descending (most recent first)
		sort.Sort(sort.Reverse(sort.StringSlice(keys)))

		// Apply limit
		if limit > 0 && len(keys) > limit {
			keys = keys[:limit]
		}

		for _, key := range keys {
			data := b.Get([]byte(key))
			if data == nil {
				continue
			}
			var result HealthResult
			if err := json.Unmarshal(data, &result); err != nil {
				continue
			}
			results = append(results, &result)
		}

		return nil
	})

	return results, err
}

// GetLatestHealthResult returns the most recent health result for a container.
func (s *BoltStore) GetLatestHealthResult(containerID string) (*HealthResult, error) {
	results, err := s.GetHealthResults(containerID, 1)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no health results for container %s", containerID)
	}
	return results[0], nil
}

// PruneHealthResults removes old health results for a container, keeping the most recent N.
func (s *BoltStore) PruneHealthResults(containerID string, keep int) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketHealthResults)
		if b == nil {
			return nil
		}

		prefix := []byte(containerID + ":")
		c := b.Cursor()

		// Collect all matching keys
		var keys []string
		for k, _ := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, _ = c.Next() {
			keys = append(keys, string(k))
		}

		if len(keys) <= keep {
			return nil
		}

		// Sort ascending (oldest first)
		sort.Strings(keys)

		// Delete oldest entries
		toDelete := keys[:len(keys)-keep]
		for _, key := range toDelete {
			if err := b.Delete([]byte(key)); err != nil {
				return fmt.Errorf("failed to delete health result %s: %w", key, err)
			}
		}

		return nil
	})
}
