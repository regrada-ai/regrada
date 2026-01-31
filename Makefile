.PHONY: build build-static build-obfuscated clean install test run help

BINARY_NAME=regrada
BUILD_DIR=./bin
MAIN_PATH=.

# Build flags for smaller binaries
LDFLAGS=-s -w
BUILDFLAGS=-trimpath

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "✓ Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

build-static: ## Build static binary (no CGO)
	@echo "Building static $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" $(BUILDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "✓ Static build complete: $(BUILD_DIR)/$(BINARY_NAME)"

build-obfuscated: ## Build obfuscated binary using garble (requires: go install mvdan.cc/garble@latest)
	@echo "Building obfuscated $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@which garble > /dev/null || (echo "Error: garble not found. Install with: go install mvdan.cc/garble@latest" && exit 1)
	CGO_ENABLED=0 garble -literals -tiny -seed=random build -ldflags="$(LDFLAGS)" $(BUILDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "✓ Obfuscated build complete: $(BUILD_DIR)/$(BINARY_NAME)"

clean: ## Remove built binaries and artifacts
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f $(BINARY_NAME)
	@echo "✓ Clean complete"

install: build ## Install the binary to GOPATH
	@echo "Installing $(BINARY_NAME)..."
	go install $(MAIN_PATH)
	@echo "✓ Install complete"

test: ## Run tests
	@echo "Running tests..."
	go test -v ./...
	@echo "✓ Tests complete"

run: build ## Build and run the binary
	@$(BUILD_DIR)/$(BINARY_NAME)

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy
	@echo "✓ Dependencies downloaded"

fmt: ## Format the code
	@echo "Formatting code..."
	go fmt ./...
	@echo "✓ Format complete"

vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...
	@echo "✓ Vet complete"

lint: fmt vet ## Run linters

all: clean deps lint test build ## Run all tasks
