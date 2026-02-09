//go:build !dev

// Package api embeds the frontend dashboard in the production binary.
package api

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

//go:embed dist/*
var staticFiles embed.FS

// GetStaticHandler returns an http.Handler that serves the embedded frontend.
// It implements SPA routing by falling back to index.html for non-file requests.
func GetStaticHandler() http.Handler {
	fsys, _ := fs.Sub(staticFiles, "dist")
	fileServer := http.FileServer(http.FS(fsys))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the exact file
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Check if file exists in the embedded FS
		f, err := fs.Stat(fsys, path)
		if err == nil && !f.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}

		// For SPA routing: serve index.html for non-file paths
		// (files with extensions that don't exist should 404)
		if filepath.Ext(path) != "" {
			http.NotFound(w, r)
			return
		}

		// Serve index.html for SPA routes
		indexFile, err := fs.ReadFile(fsys, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexFile)
	})
}

// HasEmbeddedFrontend returns true if the frontend is embedded.
func HasEmbeddedFrontend() bool {
	_, err := fs.Stat(staticFiles, "dist/index.html")
	return err == nil || os.IsExist(err)
}
