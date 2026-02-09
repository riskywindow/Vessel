package store

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

// CreateMetricPoint persists a metric data point for a container.
// Keys are formatted as "containerID:timestamp_unix_nano" for efficient range queries.
func (s *BoltStore) CreateMetricPoint(containerID string, metric *MetricPoint) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucketMetrics)
		if err != nil {
			return fmt.Errorf("failed to create metrics bucket: %w", err)
		}

		key := fmt.Sprintf("%s:%020d", containerID, metric.Timestamp.UnixNano())
		data, err := json.Marshal(metric)
		if err != nil {
			return fmt.Errorf("failed to marshal metric point: %w", err)
		}

		return b.Put([]byte(key), data)
	})
}

// GetMetrics returns metric points for a container within a time range.
// If start is zero, retrieves from the beginning. If end is zero, retrieves up to now.
// Results are returned in chronological order. If limit is 0, returns all matching points.
// If limit is negative, returns the last N points (most recent).
func (s *BoltStore) GetMetrics(containerID string, start, end time.Time, limit int) ([]*MetricPoint, error) {
	var metrics []*MetricPoint

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketMetrics)
		if b == nil {
			return nil
		}

		prefix := containerID + ":"
		c := b.Cursor()

		var startKey, endKey string
		if start.IsZero() {
			startKey = prefix
		} else {
			startKey = fmt.Sprintf("%s:%020d", containerID, start.UnixNano())
		}
		if end.IsZero() {
			endKey = fmt.Sprintf("%s:%020d", containerID, time.Now().UnixNano())
		} else {
			endKey = fmt.Sprintf("%s:%020d", containerID, end.UnixNano())
		}

		for k, v := c.Seek([]byte(startKey)); k != nil; k, v = c.Next() {
			keyStr := string(k)
			if !strings.HasPrefix(keyStr, prefix) {
				break
			}
			if keyStr > endKey {
				break
			}

			var m MetricPoint
			if err := json.Unmarshal(v, &m); err != nil {
				continue
			}
			metrics = append(metrics, &m)

			if limit > 0 && len(metrics) >= limit {
				break
			}
		}

		return nil
	})

	// If limit is negative, return the last N points
	if limit < 0 && len(metrics) > -limit {
		metrics = metrics[len(metrics)+limit:]
	}

	return metrics, err
}

// PruneMetrics removes all metric points older than the given cutoff time.
func (s *BoltStore) PruneMetrics(before time.Time) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketMetrics)
		if b == nil {
			return nil
		}

		cutoffNano := fmt.Sprintf("%020d", before.UnixNano())
		c := b.Cursor()

		var toDelete [][]byte
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			keyStr := string(k)
			// Key format: "containerID:timestamp_nano"
			idx := strings.LastIndex(keyStr, ":")
			if idx < 0 {
				continue
			}
			tsStr := keyStr[idx+1:]
			if tsStr < cutoffNano {
				toDelete = append(toDelete, append([]byte{}, k...))
			}
		}

		for _, k := range toDelete {
			if err := b.Delete(k); err != nil {
				return fmt.Errorf("failed to delete metric: %w", err)
			}
		}

		return nil
	})
}
