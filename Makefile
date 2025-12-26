.PHONY: all build build-frontend build-dev install test clean lint fmt deps dev run help vet doctor version completions frontend-dev

# Ensure Go is in PATH
export PATH := /usr/local/go/bin:$(PATH)

# Build variables
BINARY_NAME=tunnel
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GO_VERSION=$(shell /usr/local/go/bin/go version 2>/dev/null | awk '{print $$3}' || go version | awk '{print $$3}')
LDFLAGS=-ldflags "\
	-X 'main.Version=$(VERSION)' \
	-X 'main.BuildDate=$(BUILD_DATE)' \
	-X 'main.GitCommit=$(GIT_COMMIT)' \
	-X 'main.GoVersion=$(GO_VERSION)'"

# Go commands
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOVET=$(GOCMD) vet

# Directories
CMD_DIR=./cmd/tunnel
BIN_DIR=./bin
INSTALL_DIR=$(HOME)/.local/bin
WEB_DIR=./web
EMBED_DIR=./internal/web/embed/dist

# Default target
all: deps fmt vet build

## help: Show this help message
help:
	@echo "TUNNEL - Terminal Unified Network Node Encrypted Link"
	@echo ""
	@echo "Usage:"
	@echo "  make <target>"
	@echo ""
	@echo "Targets:"
	@awk '/^## / {sub("## ", ""); print}' $(MAKEFILE_LIST) | column -t -s ':'

## build-frontend: Build the React frontend
build-frontend:
	@echo "Building React frontend..."
	@if [ -d "$(WEB_DIR)" ]; then \
		cd $(WEB_DIR) && npm ci && npm run build && cd ..; \
		echo "Copying dist to embed directory..."; \
		rm -rf $(EMBED_DIR); \
		mkdir -p $(EMBED_DIR); \
		cp -r web/dist/. $(EMBED_DIR)/; \
		echo "Frontend built successfully"; \
	else \
		echo "Warning: $(WEB_DIR) directory not found, skipping frontend build"; \
		mkdir -p $(EMBED_DIR); \
		echo '<!DOCTYPE html><html><body><h1>Frontend not built</h1><p>Run make build-frontend first.</p></body></html>' > $(EMBED_DIR)/index.html; \
	fi

## build: Build with embedded frontend (production)
build: deps build-frontend
	@echo "Building $(BINARY_NAME) with embedded frontend..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "Built $(BIN_DIR)/$(BINARY_NAME)"

## build-dev: Build without frontend (faster development builds)
build-dev: deps
	@echo "Building $(BINARY_NAME) (dev mode - no embedded frontend)..."
	@mkdir -p $(EMBED_DIR)
	@echo '<!DOCTYPE html><html><body><h1>Development Mode</h1><p>Frontend not embedded. Run make build-frontend first.</p></body></html>' > $(EMBED_DIR)/index.html
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "Built $(BIN_DIR)/$(BINARY_NAME) (dev mode)"

## install: Install the binary to local bin
install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_DIR)..."
	@mkdir -p $(INSTALL_DIR)
	@cp $(BIN_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@chmod +x $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Installed to $(INSTALL_DIR)/$(BINARY_NAME)"

## uninstall: Remove the installed binary
uninstall:
	@echo "Removing $(BINARY_NAME) from $(INSTALL_DIR)..."
	@rm -f $(INSTALL_DIR)/$(BINARY_NAME)

## test: Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -cover ./...

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## lint: Run linter
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed, skipping..."; \
	fi

## fmt: Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GOVET) ./...

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

## dev: Run in development mode (without frontend)
dev: build-dev
	@echo "Running $(BINARY_NAME) in dev mode..."
	$(BIN_DIR)/$(BINARY_NAME)

## frontend-dev: Run frontend dev server
frontend-dev:
	@if [ -d "$(WEB_DIR)" ]; then \
		cd $(WEB_DIR) && npm run dev; \
	else \
		echo "$(WEB_DIR) directory not found"; \
		exit 1; \
	fi

## run: Build and run
run: build
	$(BIN_DIR)/$(BINARY_NAME)

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BIN_DIR)
	@rm -rf $(EMBED_DIR)
	@rm -rf $(WEB_DIR)/node_modules
	@rm -rf $(WEB_DIR)/dist
	@rm -f coverage.out coverage.html
	@echo "Cleaned"

## release: Build release binaries for all platforms
release: build-frontend
	@echo "Building release binaries..."
	@mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_DIR)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_DIR)
	@echo "Release binaries built in $(BIN_DIR)/"

## docker: Build Docker image
docker:
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):$(VERSION) .

## doctor: Run diagnostic checks
doctor: build-dev
	@echo "Running diagnostics..."
	$(BIN_DIR)/$(BINARY_NAME) doctor

## version: Show version information
version: build-dev
	$(BIN_DIR)/$(BINARY_NAME) version

## completions: Generate shell completions
completions: build-dev
	@echo "Generating shell completions..."
	@mkdir -p completions
	$(BIN_DIR)/$(BINARY_NAME) completions bash > completions/$(BINARY_NAME).bash
	$(BIN_DIR)/$(BINARY_NAME) completions zsh > completions/$(BINARY_NAME).zsh
	$(BIN_DIR)/$(BINARY_NAME) completions fish > completions/$(BINARY_NAME).fish
	@echo "Completions generated in completions/"
