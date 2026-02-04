package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseImageRef(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
		wantTag  string
		wantErr  bool
	}{
		{
			name:     "simple image name",
			input:    "alpine",
			wantName: "index.docker.io/library/alpine",
			wantTag:  "latest",
		},
		{
			name:     "image with tag",
			input:    "alpine:3.19",
			wantName: "index.docker.io/library/alpine",
			wantTag:  "3.19",
		},
		{
			name:     "nginx with alpine tag",
			input:    "nginx:alpine",
			wantName: "index.docker.io/library/nginx",
			wantTag:  "alpine",
		},
		{
			name:     "user repo",
			input:    "myuser/myrepo",
			wantName: "index.docker.io/myuser/myrepo",
			wantTag:  "latest",
		},
		{
			name:     "user repo with tag",
			input:    "myuser/myrepo:v1.0",
			wantName: "index.docker.io/myuser/myrepo",
			wantTag:  "v1.0",
		},
		{
			name:     "full reference with registry",
			input:    "ghcr.io/org/repo:tag",
			wantName: "ghcr.io/org/repo",
			wantTag:  "tag",
		},
		{
			name:     "ubuntu with latest",
			input:    "ubuntu:24.04",
			wantName: "index.docker.io/library/ubuntu",
			wantTag:  "24.04",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := parseImageRef(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseImageRef() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			// Check the name (context + repository)
			gotName := ref.Context().String()
			if gotName != tt.wantName {
				t.Errorf("parseImageRef() name = %v, want %v", gotName, tt.wantName)
			}

			// Check the tag (if it's a tag reference)
			gotTag := ref.Identifier()
			if gotTag != tt.wantTag {
				t.Errorf("parseImageRef() tag = %v, want %v", gotTag, tt.wantTag)
			}
		})
	}
}

func TestImageMetadataSerialization(t *testing.T) {
	original := ImageMetadata{
		Digest:    "sha256:abc123",
		Reference: "alpine:latest",
		Size:      1024 * 1024 * 10, // 10 MB
		Layers:    3,
		PulledAt:  time.Now().Truncate(time.Second),
		Config: ImageConfig{
			Entrypoint:   []string{"/bin/sh"},
			Cmd:          []string{"-c", "echo hello"},
			Env:          []string{"PATH=/usr/bin", "HOME=/root"},
			WorkingDir:   "/app",
			ExposedPorts: map[string]struct{}{"80/tcp": {}},
			User:         "nobody",
		},
	}

	// Serialize
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Deserialize
	var decoded ImageMetadata
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify fields
	if decoded.Digest != original.Digest {
		t.Errorf("Digest mismatch: got %s, want %s", decoded.Digest, original.Digest)
	}
	if decoded.Reference != original.Reference {
		t.Errorf("Reference mismatch: got %s, want %s", decoded.Reference, original.Reference)
	}
	if decoded.Size != original.Size {
		t.Errorf("Size mismatch: got %d, want %d", decoded.Size, original.Size)
	}
	if decoded.Layers != original.Layers {
		t.Errorf("Layers mismatch: got %d, want %d", decoded.Layers, original.Layers)
	}
	if len(decoded.Config.Entrypoint) != len(original.Config.Entrypoint) {
		t.Errorf("Entrypoint length mismatch")
	}
	if len(decoded.Config.Cmd) != len(original.Config.Cmd) {
		t.Errorf("Cmd length mismatch")
	}
	if decoded.Config.WorkingDir != original.Config.WorkingDir {
		t.Errorf("WorkingDir mismatch")
	}
}

func TestImageStoreListEmpty(t *testing.T) {
	// Create a temporary directory for the test
	tempDir := t.TempDir()

	store := &ImageStore{rootDir: tempDir}

	images, err := store.ListImages()
	if err != nil {
		t.Fatalf("ListImages() error = %v", err)
	}

	if len(images) != 0 {
		t.Errorf("ListImages() returned %d images, want 0", len(images))
	}
}

func TestImageStoreGetImageNotFound(t *testing.T) {
	tempDir := t.TempDir()
	store := &ImageStore{rootDir: tempDir}

	_, err := store.GetImage("sha256:nonexistent")
	if err == nil {
		t.Error("GetImage() expected error for nonexistent image")
	}
}

func TestImageStoreRemoveImageNotFound(t *testing.T) {
	tempDir := t.TempDir()
	store := &ImageStore{rootDir: tempDir}

	err := store.RemoveImage("sha256:nonexistent")
	if err == nil {
		t.Error("RemoveImage() expected error for nonexistent image")
	}
}

func TestImageStoreWithMockedImage(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()
	store := &ImageStore{rootDir: tempDir}

	// Create a mock image directory
	digest := "sha256-abc123def456"
	imageDir := filepath.Join(tempDir, digest)
	layersDir := filepath.Join(imageDir, "layers")

	if err := os.MkdirAll(filepath.Join(layersDir, "0"), 0755); err != nil {
		t.Fatalf("failed to create layer dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(layersDir, "1"), 0755); err != nil {
		t.Fatalf("failed to create layer dir: %v", err)
	}

	// Create metadata file
	metadata := ImageMetadata{
		Digest:    "sha256:abc123def456",
		Reference: "test:latest",
		Size:      1024,
		Layers:    2,
		PulledAt:  time.Now(),
		Config: ImageConfig{
			Cmd: []string{"/bin/echo", "hello"},
		},
	}
	metadataData, _ := json.Marshal(metadata)
	if err := os.WriteFile(filepath.Join(imageDir, "metadata.json"), metadataData, 0644); err != nil {
		t.Fatalf("failed to write metadata: %v", err)
	}

	// Test ListImages
	images, err := store.ListImages()
	if err != nil {
		t.Fatalf("ListImages() error = %v", err)
	}
	if len(images) != 1 {
		t.Errorf("ListImages() returned %d images, want 1", len(images))
	}

	// Test GetImage
	got, err := store.GetImage("sha256:abc123def456")
	if err != nil {
		t.Fatalf("GetImage() error = %v", err)
	}
	if got.Reference != "test:latest" {
		t.Errorf("GetImage() reference = %s, want test:latest", got.Reference)
	}

	// Test GetLayerPaths
	paths, err := store.GetLayerPaths("sha256:abc123def456")
	if err != nil {
		t.Fatalf("GetLayerPaths() error = %v", err)
	}
	if len(paths) != 2 {
		t.Errorf("GetLayerPaths() returned %d paths, want 2", len(paths))
	}

	// Test RemoveImage
	if err := store.RemoveImage("sha256:abc123def456"); err != nil {
		t.Fatalf("RemoveImage() error = %v", err)
	}

	// Verify removal
	if _, err := os.Stat(imageDir); !os.IsNotExist(err) {
		t.Error("Image directory should have been removed")
	}
}
