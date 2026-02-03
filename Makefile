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

.PHONY: all build test lint clean install fmt vet deps

# Default target
all: build

# Build the binary
build: $(BINARY_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME) ./cmd/vessel

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
