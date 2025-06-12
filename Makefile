# claude-squad Makefile
# Build commands for cross-platform Go CLI

# Variables
BINARY_NAME=cs
APP_NAME=claude-squad
VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT ?= $(shell git rev-parse --short HEAD)
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Build flags
LDFLAGS = -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)"

# Default target
.PHONY: all
all: build

# Build for current platform
.PHONY: build
build:
	go build $(LDFLAGS) -o $(BINARY_NAME)

# Build app binary (alternative name)
.PHONY: build-app
build-app:
	go build $(LDFLAGS) -o $(APP_NAME)

# Build for all platforms
.PHONY: build-all
build-all: clean-dist
	mkdir -p dist
	# Linux AMD64
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-amd64
	# Linux ARM64
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-arm64
	# macOS AMD64 (Intel)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-amd64
	# macOS ARM64 (Apple Silicon)
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-arm64
	# Windows AMD64
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-windows-amd64.exe

# Install to local bin directory
.PHONY: install
install:
	go build $(LDFLAGS) -o $(HOME)/bin/$(BINARY_NAME)

# Install to system bin directory (requires sudo)
.PHONY: install-system
install-system:
	go build $(LDFLAGS) -o /usr/local/bin/$(BINARY_NAME)

# Run tests
.PHONY: test
test:
	go test ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Run tests with verbose output
.PHONY: test-verbose
test-verbose:
	go test -v ./...

# Format code
.PHONY: fmt
fmt:
	go fmt ./...

# Run linter
.PHONY: lint
lint:
	golangci-lint run

# Clean build artifacts
.PHONY: clean
clean:
	rm -f $(BINARY_NAME) $(APP_NAME)
	rm -f coverage.out coverage.html

# Clean distribution directory
.PHONY: clean-dist
clean-dist:
	rm -rf dist

# Clean all artifacts
.PHONY: clean-all
clean-all: clean clean-dist

# Development build and run
.PHONY: dev
dev: build
	./$(BINARY_NAME)

# Build and test
.PHONY: ci
ci: fmt test lint build

# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build         Build binary for current platform"
	@echo "  build-all     Build binaries for all platforms"
	@echo "  install       Install to ~/bin/$(BINARY_NAME)"
	@echo "  install-system Install to /usr/local/bin/$(BINARY_NAME) (requires sudo)"
	@echo "  test          Run tests"
	@echo "  test-coverage Run tests with coverage report"
	@echo "  test-verbose  Run tests with verbose output"
	@echo "  fmt           Format code"
	@echo "  lint          Run linter"
	@echo "  clean         Clean build artifacts"
	@echo "  clean-all     Clean all artifacts including dist/"
	@echo "  dev           Build and run for development"
	@echo "  ci            Run CI pipeline (fmt, test, lint, build)"
	@echo "  help          Show this help"

# Release checklist (for manual releases)
.PHONY: release-check
release-check:
	@echo "Release checklist:"
	@echo "1. Update version tag: git tag v1.0.x"
	@echo "2. Push tag: git push origin v1.0.x"
	@echo "3. GitHub Actions will build and release automatically"
	@echo "4. Current version would be: $(VERSION)"

# Show build info
.PHONY: info
info:
	@echo "Build Information:"
	@echo "  Binary Name: $(BINARY_NAME)"
	@echo "  App Name:    $(APP_NAME)"
	@echo "  Version:     $(VERSION)"
	@echo "  Commit:      $(COMMIT)"
	@echo "  Build Time:  $(BUILD_TIME)"