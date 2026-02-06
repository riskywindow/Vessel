// Package vessel contains project-level sentinel errors.
package vessel

import "errors"

// Sentinel errors used across the project.
var (
	// Runtime errors
	ErrContainerNotFound       = errors.New("container not found")
	ErrContainerNotRunning     = errors.New("container is not running")
	ErrContainerAlreadyRunning = errors.New("container is already running")
	ErrImagePullFailed         = errors.New("failed to pull image")
	ErrNamespaceSetupFailed    = errors.New("failed to set up namespaces")
	ErrCgroupSetupFailed       = errors.New("failed to set up cgroups")
	ErrFilesystemSetupFailed   = errors.New("failed to set up filesystem")

	// App errors
	ErrAppNotFound      = errors.New("app not found")
	ErrAppAlreadyExists = errors.New("app already exists")
	ErrDeployInProgress = errors.New("deploy already in progress")
	ErrNoDeployHistory  = errors.New("no deploy history found")
	ErrVersionNotFound  = errors.New("deploy version not found")

	// Config errors
	ErrInvalidConfig  = errors.New("invalid configuration")
	ErrConfigNotFound = errors.New("vessel.toml not found")

	// Network errors
	ErrPortConflict         = errors.New("port already in use")
	ErrTLSProvisionFailed   = errors.New("failed to provision TLS certificate")
	ErrIPExhausted          = errors.New("no available IPs in subnet")
	ErrBridgeSetupFailed    = errors.New("failed to set up network bridge")
	ErrNetworkSetupFailed   = errors.New("failed to set up container network")
	ErrContainerIPNotFound  = errors.New("no IP allocated for container")

	// Store errors
	ErrStoreCorrupted = errors.New("data store corrupted")
	ErrSecretNotFound = errors.New("secret not found")
)
