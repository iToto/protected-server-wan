# Makefile for protect-wan

# Binary name
BINARY_NAME=protect-wan

# Build directory
BUILD_DIR=.

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet
GOMOD=$(GOCMD) mod

# Build flags
LDFLAGS=-ldflags "-s -w"

.PHONY: all build run clean test fmt vet deps install uninstall help
.PHONY: build-linux build-darwin build-windows build-all
.PHONY: check list auto disable verbose

# Default target
all: build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) $(LDFLAGS)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Build with optimizations (smaller binary)
build-optimized:
	@echo "Building optimized $(BINARY_NAME)..."
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) $(LDFLAGS) -trimpath
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Run the binary (default behavior: check and auto-protect)
run: build
	@echo "Running $(BINARY_NAME)..."
	@./$(BINARY_NAME)

# Run with check flag
check: build
	@echo "Checking exit node status..."
	@./$(BINARY_NAME) --check

# Run with list flag
list: build
	@echo "Listing Mullvad exit nodes..."
	@./$(BINARY_NAME) --list

# Run with auto flag
auto: build
	@echo "Auto-selecting best Mullvad exit node..."
	@./$(BINARY_NAME) --auto

# Run with disable flag
disable: build
	@echo "Disabling exit node..."
	@./$(BINARY_NAME) --disable

# Run with verbose flag
verbose: build
	@echo "Running with verbose output..."
	@./$(BINARY_NAME) --verbose

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	@rm -f $(BUILD_DIR)/$(BINARY_NAME)
	@rm -f $(BUILD_DIR)/$(BINARY_NAME)-*
	@echo "Clean complete"

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

# Run go vet
vet:
	@echo "Running go vet..."
	$(GOVET) ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Install binary to /usr/local/bin (requires sudo)
install: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	@sudo install -m 755 $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@echo "Installation complete. Run with: $(BINARY_NAME)"

# Uninstall binary from /usr/local/bin (requires sudo)
uninstall:
	@echo "Uninstalling $(BINARY_NAME) from /usr/local/bin..."
	@sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "Uninstall complete"

# Cross-compilation targets
build-linux:
	@echo "Building for Linux (amd64)..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(LDFLAGS)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64"

build-darwin:
	@echo "Building for macOS (arm64 and amd64)..."
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(LDFLAGS)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(LDFLAGS)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)-darwin-*"

build-windows:
	@echo "Building for Windows (amd64)..."
	GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(LDFLAGS)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe"

# Build for all platforms
build-all: build-linux build-darwin build-windows
	@echo "All builds complete"

# Show help
help:
	@echo "Makefile for $(BINARY_NAME)"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build              Build the binary (default)"
	@echo "  build-optimized    Build with optimizations (smaller binary)"
	@echo "  run                Build and run with default behavior"
	@echo "  check              Build and check exit node status"
	@echo "  list               Build and list Mullvad exit nodes"
	@echo "  auto               Build and auto-select best exit node"
	@echo "  disable            Build and disable exit node"
	@echo "  verbose            Build and run with verbose output"
	@echo "  clean              Remove build artifacts"
	@echo "  test               Run tests"
	@echo "  fmt                Format code"
	@echo "  vet                Run go vet"
	@echo "  deps               Download dependencies"
	@echo "  install            Install binary to /usr/local/bin (requires sudo)"
	@echo "  uninstall          Remove binary from /usr/local/bin (requires sudo)"
	@echo "  build-linux        Build for Linux (amd64)"
	@echo "  build-darwin       Build for macOS (arm64 and amd64)"
	@echo "  build-windows      Build for Windows (amd64)"
	@echo "  build-all          Build for all platforms"
	@echo "  help               Show this help message"
