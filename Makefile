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

.PHONY: build
build:
	go build -ldflags "$(LDFLAGS)" -o build/$(BINARY) ./cli

.PHONY: install
install:
	go install -ldflags "$(LDFLAGS)" ./cli

.PHONY: clean
clean:
	rm -rf build/

.PHONY: version
version:
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Date: $(DATE)"
	@echo "Built by: $(BUILT_BY)"
