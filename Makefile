# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
BUILT_BY ?= $(shell whoami)

# Go build flags
LDFLAGS = -X github.com/mnemolet/liberida/internal/version.Version=$(VERSION)
LDFLAGS += -X github.com/mnemolet/liberida/internal/version.Commit=$(COMMIT)
LDFLAGS += -X github.com/mnemolet/liberida/internal/version.Date=$(DATE)
LDFLAGS += -X github.com/mnemolet/liberida/internal/version.BuiltBy=$(BUILT_BY)

# Binary name
BINARY = liberida

# Build directory
BUILD_DIR = bin

# Default target
.PHONY: all
all: test build

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	go test ./... -v

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -cover ./...
	@echo "Generating coverage report..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report saved to coverage.html"

.PHONY: build
build:
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) ./cmd

.PHONY: install
install:
	go install -ldflags "$(LDFLAGS)" ./cmd

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)/

.PHONY: version
version:
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Date: $(DATE)"
	@echo "Built by: $(BUILT_BY)"
