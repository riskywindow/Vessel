//go:build dev

// Package api provides a development-mode static handler that proxies to Vite.
package api

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// GetStaticHandler returns a reverse proxy to the Vite dev server during development.
func GetStaticHandler() http.Handler {
	target, _ := url.Parse("http://localhost:5173")
	return httputil.NewSingleHostReverseProxy(target)
}

// HasEmbeddedFrontend returns false in dev mode (frontend served by Vite).
func HasEmbeddedFrontend() bool {
	return false
}
