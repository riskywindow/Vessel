// Package testutil provides shared test utilities.
package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vessel/vessel/internal/store"
)

// TempStore creates a temporary BBolt store for testing.
// The store and its directory are cleaned up when the test finishes.
func TempStore(t *testing.T) store.Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to create temp store: %v", err)
	}
	t.Cleanup(func() {
		st.Close()
	})

	return st
}

// CreateTestDir creates a temporary directory for testing.
func CreateTestDir(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}
	return dir
}
