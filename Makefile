# Vessel Makefile
# Build targets for the Vessel container orchestrator

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
GOFMT=$(GOCMD) fmt
GOMOD=$(GOCMD) mod

# Binary name
BINARY_NAME=vessel
BINARY_DIR=./bin

# Version info (embedded via ldflags)
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# Build flags
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)"

# Install path
INSTALL_PATH=/usr/local/bin

.PHONY: all build build-frontend build-backend build-dev test lint clean install fmt vet deps

# Default target — builds frontend + backend into single binary
all: build

# Build everything: frontend + backend
build: build-frontend build-backend

# Build frontend and copy to internal/api/dist for embedding
build-frontend:
	cd web && npm install && npm run build
	rm -rf internal/api/dist
	cp -r web/dist internal/api/dist

# Build the Go binary with embedded frontend
build-backend: $(BINARY_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME) ./cmd/vessel

# Build dev binary (proxies to Vite dev server instead of embedding frontend)
build-dev: $(BINARY_DIR)
	$(GOBUILD) -tags dev $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME) ./cmd/vessel

$(BINARY_DIR):
	mkdir -p $(BINARY_DIR)

# Run tests with race detector
test:
	$(GOTEST) -race -v ./...

# Run linter (requires golangci-lint)
lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed" && exit 1)
	golangci-lint run ./...

# Clean build artifacts
clean:
	rm -rf $(BINARY_DIR)
	rm -rf web/dist
	rm -rf internal/api/dist
	$(GOCMD) clean

# Install to system path
install: build
	sudo cp $(BINARY_DIR)/$(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "Installed $(BINARY_NAME) to $(INSTALL_PATH)"

# Format code
fmt:
	$(GOFMT) ./...

# Run go vet
vet:
	$(GOVET) ./...

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Development helpers
.PHONY: dev run

# Build and run for development
dev: build
	$(BINARY_DIR)/$(BINARY_NAME)

# Run without building
run:
	$(GOCMD) run ./cmd/vessel

# Show version info that will be embedded
.PHONY: version
version:
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
	@echo "Built:   $(BUILD_TIME)"
