package runtime

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/vessel/vessel/internal/paths"
)

// ImageMetadata contains metadata about a pulled image.
type ImageMetadata struct {
	Digest    string      `json:"digest"`
	Reference string      `json:"reference"` // Original pull reference
	Size      int64       `json:"size"`      // Total size in bytes
	Layers    int         `json:"layers"`    // Number of layers
	PulledAt  time.Time   `json:"pulled_at"`
	Config    ImageConfig `json:"config"`
}

// ImageConfig contains the runtime configuration from the image.
type ImageConfig struct {
	Entrypoint   []string            `json:"entrypoint"`
	Cmd          []string            `json:"cmd"`
	Env          []string            `json:"env"`
	WorkingDir   string              `json:"working_dir"`
	ExposedPorts map[string]struct{} `json:"exposed_ports"`
	User         string              `json:"user"`
}

// ImageStore manages the local image cache.
type ImageStore struct {
	rootDir string
}

// NewImageStore creates a new image store.
func NewImageStore() (*ImageStore, error) {
	if err := os.MkdirAll(paths.ImagesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create images directory: %w", err)
	}
	return &ImageStore{rootDir: paths.ImagesDir}, nil
}

// PullImage pulls an OCI image from a registry.
func (s *ImageStore) PullImage(ctx context.Context, ref string) (string, error) {
	// Parse the image reference
	parsedRef, err := parseImageRef(ref)
	if err != nil {
		return "", fmt.Errorf("invalid image reference %q: %w", ref, err)
	}

	fmt.Printf("Pulling image: %s\n", parsedRef.Name())

	// Set up authentication
	authOption := remote.WithAuthFromKeychain(authn.DefaultKeychain)

	// Set up platform matching (linux/amd64 or linux/arm64)
	arch := goruntime.GOARCH
	platform := v1.Platform{
		Architecture: arch,
		OS:           "linux",
	}
	platformOption := remote.WithPlatform(platform)

	// Create a context option
	ctxOption := remote.WithContext(ctx)

	// Fetch the image descriptor first to check if we have it
	desc, err := remote.Get(parsedRef, authOption, platformOption, ctxOption)
	if err != nil {
		return "", fmt.Errorf("failed to fetch image descriptor: %w", err)
	}

	// Use the digest as the local storage key
	digest := desc.Digest.String()
	// Clean up digest for use as directory name (remove sha256: prefix)
	digestDir := strings.ReplaceAll(digest, ":", "-")

	imageDir := filepath.Join(s.rootDir, digestDir)
	metadataPath := filepath.Join(imageDir, "metadata.json")

	// Check if image already exists locally
	if _, err := os.Stat(metadataPath); err == nil {
		fmt.Printf("Image already exists locally: %s\n", digest)
		return digest, nil
	}

	// Fetch the full image
	img, err := remote.Image(parsedRef, authOption, platformOption, ctxOption)
	if err != nil {
		return "", fmt.Errorf("failed to fetch image: %w", err)
	}

	// Create image directory structure
	layersDir := filepath.Join(imageDir, "layers")
	if err := os.MkdirAll(layersDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create layers directory: %w", err)
	}

	// Get image manifest
	manifest, err := img.Manifest()
	if err != nil {
		os.RemoveAll(imageDir)
		return "", fmt.Errorf("failed to get manifest: %w", err)
	}

	// Save manifest
	manifestPath := filepath.Join(imageDir, "manifest.json")
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		os.RemoveAll(imageDir)
		return "", fmt.Errorf("failed to marshal manifest: %w", err)
	}
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		os.RemoveAll(imageDir)
		return "", fmt.Errorf("failed to write manifest: %w", err)
	}

	// Get image config
	configFile, err := img.ConfigFile()
	if err != nil {
		os.RemoveAll(imageDir)
		return "", fmt.Errorf("failed to get config: %w", err)
	}

	// Save config
	configPath := filepath.Join(imageDir, "config.json")
	configData, err := json.MarshalIndent(configFile, "", "  ")
	if err != nil {
		os.RemoveAll(imageDir)
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		os.RemoveAll(imageDir)
		return "", fmt.Errorf("failed to write config: %w", err)
	}

	// Download and extract layers
	layers, err := img.Layers()
	if err != nil {
		os.RemoveAll(imageDir)
		return "", fmt.Errorf("failed to get layers: %w", err)
	}

	fmt.Printf("Downloading %d layers...\n", len(layers))
	var totalSize int64

	for i, layer := range layers {
		layerDigest, err := layer.Digest()
		if err != nil {
			os.RemoveAll(imageDir)
			return "", fmt.Errorf("failed to get layer %d digest: %w", i, err)
		}

		layerSize, err := layer.Size()
		if err != nil {
			os.RemoveAll(imageDir)
			return "", fmt.Errorf("failed to get layer %d size: %w", i, err)
		}
		totalSize += layerSize

		fmt.Printf("  Layer %d/%d: %s (%.2f MB)\n", i+1, len(layers), layerDigest.Hex[:12], float64(layerSize)/1024/1024)

		// Create layer directory
		layerDir := filepath.Join(layersDir, fmt.Sprintf("%d", i))
		if err := os.MkdirAll(layerDir, 0755); err != nil {
			os.RemoveAll(imageDir)
			return "", fmt.Errorf("failed to create layer %d directory: %w", i, err)
		}

		// Extract the layer
		if err := extractLayer(layer, layerDir); err != nil {
			os.RemoveAll(imageDir)
			return "", fmt.Errorf("failed to extract layer %d: %w", i, err)
		}
	}

	// Create metadata
	metadata := ImageMetadata{
		Digest:    digest,
		Reference: ref,
		Size:      totalSize,
		Layers:    len(layers),
		PulledAt:  time.Now(),
		Config: ImageConfig{
			Entrypoint:   configFile.Config.Entrypoint,
			Cmd:          configFile.Config.Cmd,
			Env:          configFile.Config.Env,
			WorkingDir:   configFile.Config.WorkingDir,
			ExposedPorts: make(map[string]struct{}),
			User:         configFile.Config.User,
		},
	}

	// Convert exposed ports
	for port := range configFile.Config.ExposedPorts {
		metadata.Config.ExposedPorts[port] = struct{}{}
	}

	// Save metadata
	metadataData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		os.RemoveAll(imageDir)
		return "", fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if err := os.WriteFile(metadataPath, metadataData, 0644); err != nil {
		os.RemoveAll(imageDir)
		return "", fmt.Errorf("failed to write metadata: %w", err)
	}

	fmt.Printf("Successfully pulled image: %s (%.2f MB)\n", digest[:19], float64(totalSize)/1024/1024)

	return digest, nil
}

// GetImage returns metadata for a cached image.
func (s *ImageStore) GetImage(digest string) (*ImageMetadata, error) {
	digestDir := strings.ReplaceAll(digest, ":", "-")
	metadataPath := filepath.Join(s.rootDir, digestDir, "metadata.json")

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("image not found: %s", digest)
	}

	var metadata ImageMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return &metadata, nil
}

// GetImageByRef returns metadata for an image by its reference.
// This searches through all cached images to find a match.
func (s *ImageStore) GetImageByRef(ref string) (*ImageMetadata, error) {
	images, err := s.ListImages()
	if err != nil {
		return nil, err
	}

	// Normalize the reference for comparison
	parsedRef, err := parseImageRef(ref)
	if err != nil {
		return nil, err
	}
	normalizedRef := parsedRef.Name()

	for _, img := range images {
		// Try to parse the stored reference and compare
		storedRef, err := parseImageRef(img.Reference)
		if err == nil && storedRef.Name() == normalizedRef {
			return img, nil
		}
		// Also check exact match
		if img.Reference == ref {
			return img, nil
		}
	}

	return nil, fmt.Errorf("image not found: %s", ref)
}

// ListImages returns all locally cached images.
func (s *ImageStore) ListImages() ([]*ImageMetadata, error) {
	entries, err := os.ReadDir(s.rootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to list images directory: %w", err)
	}

	var images []*ImageMetadata
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		metadataPath := filepath.Join(s.rootDir, entry.Name(), "metadata.json")
		data, err := os.ReadFile(metadataPath)
		if err != nil {
			continue // Skip directories without metadata
		}

		var metadata ImageMetadata
		if err := json.Unmarshal(data, &metadata); err != nil {
			continue
		}

		images = append(images, &metadata)
	}

	return images, nil
}

// RemoveImage removes a cached image.
func (s *ImageStore) RemoveImage(digest string) error {
	digestDir := strings.ReplaceAll(digest, ":", "-")
	imageDir := filepath.Join(s.rootDir, digestDir)

	if _, err := os.Stat(imageDir); os.IsNotExist(err) {
		return fmt.Errorf("image not found: %s", digest)
	}

	if err := os.RemoveAll(imageDir); err != nil {
		return fmt.Errorf("failed to remove image: %w", err)
	}

	return nil
}

// GetLayerPaths returns the paths to all layer directories for an image, ordered bottom to top.
func (s *ImageStore) GetLayerPaths(digest string) ([]string, error) {
	metadata, err := s.GetImage(digest)
	if err != nil {
		return nil, err
	}

	digestDir := strings.ReplaceAll(digest, ":", "-")
	layersDir := filepath.Join(s.rootDir, digestDir, "layers")

	var layerPaths []string
	for i := 0; i < metadata.Layers; i++ {
		layerPath := filepath.Join(layersDir, fmt.Sprintf("%d", i))
		if _, err := os.Stat(layerPath); err != nil {
			return nil, fmt.Errorf("layer %d not found: %w", i, err)
		}
		layerPaths = append(layerPaths, layerPath)
	}

	return layerPaths, nil
}

// parseImageRef parses an image reference string.
func parseImageRef(ref string) (name.Reference, error) {
	// Handle different reference formats
	// - alpine -> docker.io/library/alpine:latest
	// - alpine:3.19 -> docker.io/library/alpine:3.19
	// - nginx:alpine -> docker.io/library/nginx:alpine
	// - ghcr.io/org/repo:tag -> ghcr.io/org/repo:tag
	// - ubuntu@sha256:... -> docker.io/library/ubuntu@sha256:...

	// Try to parse as-is first
	parsed, err := name.ParseReference(ref)
	if err == nil {
		return parsed, nil
	}

	// If parsing fails, try with docker.io/library prefix
	if !strings.Contains(ref, "/") {
		// Simple name like "alpine" or "alpine:latest"
		ref = "docker.io/library/" + ref
	} else if strings.Count(ref, "/") == 1 && !strings.Contains(strings.Split(ref, "/")[0], ".") {
		// User/repo format like "myuser/myrepo" without a registry
		ref = "docker.io/" + ref
	}

	return name.ParseReference(ref)
}

// extractLayer extracts a layer tarball to a directory.
func extractLayer(layer v1.Layer, destDir string) error {
	// Get the uncompressed layer content
	reader, err := layer.Uncompressed()
	if err != nil {
		return fmt.Errorf("failed to get uncompressed layer: %w", err)
	}
	defer reader.Close()

	// Extract the tarball
	return extractTar(destDir, reader)
}

// extractTar extracts a tar archive to the specified directory.
func extractTar(destDir string, r io.Reader) error {
	tr := tar.NewReader(r)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Clean the path to prevent path traversal
		target := filepath.Join(destDir, header.Name)
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) {
			// Allow the destDir itself
			if target != filepath.Clean(destDir) {
				return fmt.Errorf("invalid tar path: %s", header.Name)
			}
		}

		// Handle whiteout files (used by OverlayFS to mark deletions)
		baseName := filepath.Base(header.Name)
		if strings.HasPrefix(baseName, ".wh.") {
			// This is a whiteout marker - create a character device with 0,0
			// to signal deletion in the overlay
			whiteoutTarget := filepath.Join(filepath.Dir(target), strings.TrimPrefix(baseName, ".wh."))
			// For now, we just skip whiteouts during extraction
			// The OverlayFS upper layer handles them properly
			_ = whiteoutTarget
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", target, err)
			}
		case tar.TypeReg:
			// Create parent directories
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for %s: %w", target, err)
			}

			// Create file
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", target, err)
			}

			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("failed to write file %s: %w", target, err)
			}
			f.Close()

		case tar.TypeSymlink:
			// Create parent directories
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for symlink %s: %w", target, err)
			}
			// Remove existing file if present
			os.Remove(target)
			if err := os.Symlink(header.Linkname, target); err != nil {
				return fmt.Errorf("failed to create symlink %s: %w", target, err)
			}

		case tar.TypeLink:
			// Create parent directories
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for hardlink %s: %w", target, err)
			}
			// Hard link to another file in the archive
			linkTarget := filepath.Join(destDir, header.Linkname)
			os.Remove(target)
			if err := os.Link(linkTarget, target); err != nil {
				return fmt.Errorf("failed to create hardlink %s -> %s: %w", target, linkTarget, err)
			}

		case tar.TypeChar, tar.TypeBlock:
			// Skip device nodes for now - they require root and mknod
			continue

		case tar.TypeFifo:
			// Skip FIFOs
			continue

		default:
			// Skip unknown types
			continue
		}
	}

	return nil
}
