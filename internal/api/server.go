// Package api provides the REST API server for the Vessel web dashboard.
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// Server is the HTTP API server for the Vessel dashboard.
type Server struct {
	router  chi.Router
	handler *Handler
	server  *http.Server
	logger  *slog.Logger
}

// Config holds API server configuration.
type Config struct {
	Addr      string   // Listen address (default ":8080")
	APIKeys   []string // Valid API keys (empty = no auth required)
	CORSHosts []string // Allowed CORS origins
}

// NewServer creates a new API server.
func NewServer(h *Handler, cfg Config, logger *slog.Logger) *Server {
	if cfg.Addr == "" {
		cfg.Addr = ":8080"
	}

	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS
	allowedOrigins := cfg.CORSHosts
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"*"}
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-API-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// API key authentication (if configured)
	if len(cfg.APIKeys) > 0 {
		r.Use(apiKeyAuth(cfg.APIKeys))
	}

	s := &Server{
		router:  r,
		handler: h,
		logger:  logger,
	}

	s.setupRoutes()

	s.server = &http.Server{
		Addr:         cfg.Addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// setupRoutes registers all API routes.
func (s *Server) setupRoutes() {
	r := s.router

	// Health check (no auth required — auth middleware skips /api/ping)
	r.Get("/api/ping", s.handler.Ping)

	// Apps
	r.Route("/api/apps", func(r chi.Router) {
		r.Get("/", s.handler.ListApps)
		r.Route("/{name}", func(r chi.Router) {
			r.Get("/", s.handler.GetApp)
			r.Delete("/", s.handler.RemoveApp)
			r.Post("/stop", s.handler.StopApp)
			r.Post("/restart", s.handler.RestartApp)
			r.Post("/deploy", s.handler.DeployApp)
			r.Post("/rollback", s.handler.RollbackApp)
			r.Get("/history", s.handler.GetDeployHistory)
		})
	})

	// Containers
	r.Route("/api/containers", func(r chi.Router) {
		r.Get("/", s.handler.ListContainers)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", s.handler.GetContainer)
			r.Get("/logs", s.handler.GetContainerLogs)
			r.Get("/stats", s.handler.GetContainerStats)
		})
	})

	// Health monitoring
	r.Route("/api/health", func(r chi.Router) {
		r.Get("/", s.handler.GetAllHealth)
		r.Route("/{containerId}", func(r chi.Router) {
			r.Get("/", s.handler.GetHealthStatus)
			r.Post("/check", s.handler.CheckHealthNow)
			r.Get("/history", s.handler.GetHealthHistory)
		})
	})

	// Metrics
	r.Route("/api/metrics", func(r chi.Router) {
		r.Get("/{containerId}", s.handler.GetMetrics)
		r.Get("/{containerId}/latest", s.handler.GetLatestMetrics)
	})

	// Secrets
	r.Route("/api/secrets", func(r chi.Router) {
		r.Get("/", s.handler.ListSecrets)
		r.Post("/", s.handler.SetSecret)
		r.Delete("/{key}", s.handler.DeleteSecret)
	})

	// Network
	r.Get("/api/network/routes", s.handler.GetNetworkRoutes)

	// Certificates
	r.Route("/api/certs", func(r chi.Router) {
		r.Get("/", s.handler.ListCerts)
		r.Post("/", s.handler.ImportCert)
	})

	// System info
	r.Get("/api/system", s.handler.GetSystemInfo)

	// WebSocket endpoints
	r.Route("/api/ws", func(r chi.Router) {
		r.Get("/containers/{id}/logs", s.handler.StreamContainerLogs)
		r.Get("/containers/{id}/metrics", s.handler.StreamContainerMetrics)
		r.Get("/metrics", s.handler.StreamAllMetrics)
		r.Get("/health", s.handler.StreamHealth)
		r.Get("/events", s.handler.StreamEvents)
	})

	// Serve embedded frontend for all non-API routes
	staticHandler := GetStaticHandler()
	r.NotFound(staticHandler.ServeHTTP)
}

// Start starts the API server. It blocks until the server is shut down.
func (s *Server) Start() error {
	s.logger.Info("starting API server", "addr", s.server.Addr)
	if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Stop gracefully shuts down the API server.
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("stopping API server")
	return s.server.Shutdown(ctx)
}

// Router returns the chi router for testing.
func (s *Server) Router() chi.Router {
	return s.router
}

// apiKeyAuth is middleware that validates API keys.
func apiKeyAuth(validKeys []string) func(http.Handler) http.Handler {
	keySet := make(map[string]bool)
	for _, k := range validKeys {
		keySet[k] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for ping endpoint
			if r.URL.Path == "/api/ping" {
				next.ServeHTTP(w, r)
				return
			}

			// Check X-API-Key header
			key := r.Header.Get("X-API-Key")
			if key == "" {
				// Also check Authorization: Bearer <key>
				auth := r.Header.Get("Authorization")
				if len(auth) > 7 && auth[:7] == "Bearer " {
					key = auth[7:]
				}
			}
			if key == "" {
				// Also check api_key query param (for WebSocket clients)
				key = r.URL.Query().Get("api_key")
			}

			if key == "" || !keySet[key] {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or missing API key")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Response helpers

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
