# govpn Makefile
# Usage: make [target]

BINARY     := govpn
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS    := -X main.Version=$(VERSION)
BUILD_DIR  := dist

GO         := go
GOFLAGS    := -mod=vendor

.PHONY: all build test test-race bench lint clean install help

all: test build ## Run tests then build (default)

build: ## Build the govpn binary
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) ./cmd/govpn
	@echo "Built $(BUILD_DIR)/$(BINARY)"

test: ## Run the full test suite
	$(GO) test $(GOFLAGS) ./... -timeout 30s -count=1

test-race: ## Run tests with the data race detector
	$(GO) test $(GOFLAGS) -race ./... -timeout 60s -count=1

bench: ## Run benchmarks (cipher package)
	$(GO) test $(GOFLAGS) ./internal/cipher/... -bench=. -benchmem -benchtime=5s

lint: ## Run go vet
	$(GO) vet $(GOFLAGS) ./...

clean: ## Remove build artefacts
	rm -rf $(BUILD_DIR)

install: build ## Install govpn to $(GOPATH)/bin
	cp $(BUILD_DIR)/$(BINARY) $(GOPATH)/bin/$(BINARY)
	@echo "Installed to $(GOPATH)/bin/$(BINARY)"

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
