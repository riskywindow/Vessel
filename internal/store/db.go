package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	bolt "go.etcd.io/bbolt"
)

// Bucket names for the BBolt database.
var (
	bucketApps       = []byte("apps")
	bucketContainers = []byte("containers")
	bucketDeploys    = []byte("deploys")
	bucketSecrets    = []byte("secrets")
)

// Store provides persistent state storage using BBolt.
type Store interface {
	// App operations
	CreateApp(app *App) error
	GetApp(name string) (*App, error)
	ListApps() ([]*App, error)
	UpdateApp(app *App) error
	DeleteApp(name string) error

	// Container operations
	CreateContainer(c *Container) error
	GetContainer(id string) (*Container, error)
	ListContainersByApp(appID string) ([]*Container, error)
	UpdateContainer(c *Container) error
	DeleteContainer(id string) error

	// Deploy operations
	CreateDeploy(d *Deploy) error
	GetDeploy(id string) (*Deploy, error)
	ListDeploysByApp(appID string) ([]*Deploy, error)
	GetLatestDeploy(appID string) (*Deploy, error)
	UpdateDeploy(d *Deploy) error
	GetNextDeployVersion(appID string) (int, error)
	GetDeployByVersion(appID string, version int) (*Deploy, error)
	DeleteDeploy(id string) error

	// Secret operations
	SetSecret(key string, encryptedValue []byte) error
	GetSecret(key string) (*Secret, error)
	ListSecretKeys() ([]string, error)
	DeleteSecret(key string) error

	// Lifecycle
	Close() error
}

// BoltStore implements Store using BBolt.
type BoltStore struct {
	db *bolt.DB
}

// Open creates or opens a BBolt database at the specified path.
func Open(path string) (*BoltStore, error) {
	// Ensure the directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &BoltStore{db: db}

	// Initialize buckets
	if err := store.initBuckets(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize buckets: %w", err)
	}

	return store, nil
}

// initBuckets creates the required buckets if they don't exist.
func (s *BoltStore) initBuckets() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		buckets := [][]byte{
			bucketApps,
			bucketContainers,
			bucketDeploys,
			bucketSecrets,
		}

		for _, name := range buckets {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", name, err)
			}
		}

		return nil
	})
}

// Close closes the database.
func (s *BoltStore) Close() error {
	return s.db.Close()
}

// encode serializes a value to JSON.
func encode(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// decode deserializes JSON data into a value.
func decode(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
