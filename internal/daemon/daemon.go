// Package daemon manages the Vessel daemon process.
package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/vessel/vessel/internal/manager"
	"github.com/vessel/vessel/internal/network"
	"github.com/vessel/vessel/internal/paths"
	"github.com/vessel/vessel/internal/runtime"
	"github.com/vessel/vessel/internal/store"
)

const (
	// defaultMasterPassword is used when no master password is configured.
	// In production, this should be prompted or set via env var.
	defaultMasterPassword = "vessel-default-master-key"
)

// Daemon is the long-running Vessel process that manages containers.
type Daemon struct {
	runtime       runtime.Runtime
	store         store.Store
	network       network.NetworkManager
	manager       *manager.AppManager
	secretManager *store.SecretManager
	logger        *slog.Logger
	socket        net.Listener
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

// NewDaemon creates a new Daemon instance.
func NewDaemon(logger *slog.Logger) (*Daemon, error) {
	// Ensure directories exist
	for _, dir := range []string{paths.StateDir, paths.RunDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Initialize the store
	st, err := store.Open(paths.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open store: %w", err)
	}

	// Initialize the runtime
	rt, err := runtime.NewLinuxRuntime()
	if err != nil {
		st.Close()
		return nil, fmt.Errorf("failed to create runtime: %w", err)
	}

	// Initialize the network manager
	netMgr, err := network.NewLinuxNetworkManager(logger)
	if err != nil {
		logger.Warn("failed to create network manager, container networking will be unavailable", "error", err)
	}

	// Initialize the manager (netMgr may be nil if network init failed)
	var netIface network.NetworkManager
	if netMgr != nil {
		netIface = netMgr
	}
	mgr := manager.NewAppManager(rt, st, netIface, logger)

	// Initialize the secret manager
	sm, err := initSecretManager(st, logger)
	if err != nil {
		logger.Warn("failed to initialize secret manager, secrets will be unavailable", "error", err)
	}

	// Wire the secret manager into the manager for deploy-time secret resolution
	if sm != nil {
		mgr.SetSecretManager(sm)
	}

	return &Daemon{
		runtime:       rt,
		store:         st,
		network:       netIface,
		manager:       mgr,
		secretManager: sm,
		logger:        logger,
		stopCh:        make(chan struct{}),
	}, nil
}

// initSecretManager creates or loads the SecretManager with persisted salt.
func initSecretManager(st store.Store, logger *slog.Logger) (*store.SecretManager, error) {
	// Ensure secrets directory exists
	if err := os.MkdirAll(paths.SecretsDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create secrets directory: %w", err)
	}

	// Get master password from env var, or use default
	masterPassword := os.Getenv("VESSEL_MASTER_PASSWORD")
	if masterPassword == "" {
		masterPassword = defaultMasterPassword
		logger.Debug("using default master password (set VESSEL_MASTER_PASSWORD to override)")
	}

	// Load or generate salt
	var salt []byte
	saltData, err := os.ReadFile(paths.SaltPath)
	if err == nil && len(saltData) > 0 {
		salt = saltData
	} else {
		// First-time: generate and persist salt
		salt, err = store.GenerateSalt()
		if err != nil {
			return nil, fmt.Errorf("failed to generate salt: %w", err)
		}
		if err := os.WriteFile(paths.SaltPath, salt, 0600); err != nil {
			return nil, fmt.Errorf("failed to save salt: %w", err)
		}
		logger.Info("generated new encryption salt")
	}

	sm, err := store.NewSecretManager(st, masterPassword, salt)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret manager: %w", err)
	}

	return sm, nil
}

// GetManager returns the daemon's app manager.
func (d *Daemon) GetManager() *manager.AppManager {
	return d.manager
}

// GetRuntime returns the daemon's runtime.
func (d *Daemon) GetRuntime() runtime.Runtime {
	return d.runtime
}

// GetSecretManager returns the daemon's secret manager.
func (d *Daemon) GetSecretManager() *store.SecretManager {
	return d.secretManager
}

// Start starts the daemon. It blocks until the context is cancelled or Stop is called.
func (d *Daemon) Start(ctx context.Context) error {
	d.logger.Info("starting vessel daemon")

	// Start network bridge (sets up vessel0 bridge, NAT rules, IP forwarding)
	if d.network != nil {
		if err := d.network.Start(ctx); err != nil {
			d.logger.Warn("failed to start network bridge, container networking may not work", "error", err)
		} else {
			d.logger.Info("network bridge initialized")
		}
	}

	// Remove stale socket file
	os.Remove(paths.SocketPath)

	// Create Unix socket listener
	listener, err := net.Listen("unix", paths.SocketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on socket %s: %w", paths.SocketPath, err)
	}
	d.socket = listener

	// Set socket permissions
	if err := os.Chmod(paths.SocketPath, 0660); err != nil {
		d.logger.Warn("failed to set socket permissions", "error", err)
	}

	d.logger.Info("listening on socket", "path", paths.SocketPath)

	// Start the reconciliation loop
	reconcileCtx, reconcileCancel := context.WithCancel(ctx)
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.manager.GetReconciler().Run(reconcileCtx)
	}()

	// Start accepting connections on the socket
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.acceptConnections(ctx)
	}()

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	d.logger.Info("vessel daemon started")

	// Block until shutdown signal
	select {
	case sig := <-sigCh:
		d.logger.Info("received signal", "signal", sig)
	case <-ctx.Done():
		d.logger.Info("context cancelled")
	case <-d.stopCh:
		d.logger.Info("stop requested")
	}

	// Initiate graceful shutdown
	signal.Stop(sigCh)
	reconcileCancel()

	return d.shutdown()
}

// Stop signals the daemon to shut down.
func (d *Daemon) Stop() error {
	select {
	case <-d.stopCh:
		// Already stopping
	default:
		close(d.stopCh)
	}
	return nil
}

// shutdown performs graceful shutdown of all daemon components.
func (d *Daemon) shutdown() error {
	d.logger.Info("shutting down vessel daemon")

	// Stop accepting new connections
	if d.socket != nil {
		d.socket.Close()
	}

	// Wait for in-flight operations to complete (with timeout)
	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		d.logger.Info("all goroutines stopped")
	case <-time.After(30 * time.Second):
		d.logger.Warn("shutdown timed out, forcing exit")
	}

	// Stop network manager
	if d.network != nil {
		if err := d.network.Stop(); err != nil {
			d.logger.Error("failed to stop network manager", "error", err)
		}
	}

	// Close the store
	if d.store != nil {
		if err := d.store.Close(); err != nil {
			d.logger.Error("failed to close store", "error", err)
		}
	}

	// Remove socket file
	os.Remove(paths.SocketPath)

	d.logger.Info("vessel daemon stopped")
	return nil
}

// acceptConnections handles incoming connections on the Unix socket.
func (d *Daemon) acceptConnections(ctx context.Context) {
	for {
		conn, err := d.socket.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-d.stopCh:
				return
			default:
				d.logger.Error("failed to accept connection", "error", err)
				continue
			}
		}

		d.wg.Add(1)
		go func() {
			defer d.wg.Done()
			d.handleConnection(ctx, conn)
		}()
	}
}

// handleConnection processes a single client connection.
func (d *Daemon) handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	handler := NewHandler(d.manager, d.runtime, d.secretManager, d.logger)
	handler.Handle(ctx, conn)
}

// IsRunning checks if a daemon is currently running by attempting to connect to the socket.
func IsRunning() bool {
	conn, err := net.DialTimeout("unix", paths.SocketPath, 1*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
