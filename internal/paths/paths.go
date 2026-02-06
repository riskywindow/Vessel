// Package paths defines filesystem path constants for Vessel.
package paths

import "path/filepath"

const (
	// RootDir is the root data directory for Vessel.
	RootDir = "/var/lib/vessel"
)

var (
	// ImagesDir stores OCI image layers.
	ImagesDir = filepath.Join(RootDir, "images")

	// ContainersDir stores container filesystems.
	ContainersDir = filepath.Join(RootDir, "containers")

	// StateDir stores persistent state.
	StateDir = filepath.Join(RootDir, "state")

	// DatabasePath is the path to the BBolt database.
	DatabasePath = filepath.Join(StateDir, "vessel.db")

	// SecretsDir stores encrypted secrets.
	SecretsDir = filepath.Join(RootDir, "secrets")

	// VaultPath is the path to the encrypted secrets file.
	VaultPath = filepath.Join(SecretsDir, "vault.enc")

	// SaltPath is the path to the Argon2id salt for secret encryption.
	SaltPath = filepath.Join(SecretsDir, "salt")

	// TLSDir stores TLS certificates.
	TLSDir = filepath.Join(RootDir, "tls")

	// RunDir stores runtime files.
	RunDir = filepath.Join(RootDir, "run")

	// SocketPath is the path to the Unix socket for CLI-daemon communication.
	SocketPath = filepath.Join(RunDir, "vessel.sock")
)

// ContainerDir returns the path to a specific container's directory.
func ContainerDir(containerID string) string {
	return filepath.Join(ContainersDir, containerID)
}

// ContainerLowerDir returns the path to a container's lower (image layers) directory.
func ContainerLowerDir(containerID string) string {
	return filepath.Join(ContainerDir(containerID), "lower")
}

// ContainerUpperDir returns the path to a container's upper (writable) layer.
func ContainerUpperDir(containerID string) string {
	return filepath.Join(ContainerDir(containerID), "upper")
}

// ContainerWorkDir returns the path to a container's OverlayFS work directory.
func ContainerWorkDir(containerID string) string {
	return filepath.Join(ContainerDir(containerID), "work")
}

// ContainerMergedDir returns the path to a container's merged rootfs.
func ContainerMergedDir(containerID string) string {
	return filepath.Join(ContainerDir(containerID), "merged")
}

// ContainerConfigPath returns the path to a container's configuration file.
func ContainerConfigPath(containerID string) string {
	return filepath.Join(ContainerDir(containerID), "config.json")
}

// ContainerLogsDir returns the path to a container's logs directory.
func ContainerLogsDir(containerID string) string {
	return filepath.Join(ContainerDir(containerID), "logs")
}

// ContainerStdoutLog returns the path to a container's stdout log.
func ContainerStdoutLog(containerID string) string {
	return filepath.Join(ContainerLogsDir(containerID), "stdout.log")
}

// ContainerStderrLog returns the path to a container's stderr log.
func ContainerStderrLog(containerID string) string {
	return filepath.Join(ContainerLogsDir(containerID), "stderr.log")
}

// ImageDir returns the path to a specific image's directory.
func ImageDir(digest string) string {
	return filepath.Join(ImagesDir, digest)
}

// ImageManifestPath returns the path to an image's manifest file.
func ImageManifestPath(digest string) string {
	return filepath.Join(ImageDir(digest), "manifest.json")
}

// TLSDomainDir returns the path to a specific domain's TLS directory.
func TLSDomainDir(domain string) string {
	return filepath.Join(TLSDir, domain)
}

// TLSCertPath returns the path to a domain's certificate.
func TLSCertPath(domain string) string {
	return filepath.Join(TLSDomainDir(domain), "cert.pem")
}

// TLSKeyPath returns the path to a domain's private key.
func TLSKeyPath(domain string) string {
	return filepath.Join(TLSDomainDir(domain), "key.pem")
}
