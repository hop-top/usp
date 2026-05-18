SHELL := /bin/sh

GO ?= go
GOFLAGS ?=
PKG ?= ./...
BIN_DIR ?= bin
GOCACHE ?= $(CURDIR)/.cache/go-build
INSTALL_DIR ?= $${XDG_BIN_HOME:-$$HOME/.local/bin}

USP_BIN := $(BIN_DIR)/usp
USP_CTXT_BIN := $(BIN_DIR)/usp-ctxt
GOENV := env -u GOROOT GOCACHE=$(GOCACHE) GOFLAGS="$(GOFLAGS)"

.PHONY: help build build-usp build-usp-ctxt install run test fmt vet lint tidy check clean

help: ## Show available targets
	@echo "Targets:"
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	  | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

build: build-usp build-usp-ctxt ## Build all binaries into $(BIN_DIR)/

build-usp: ## Build the usp CLI into $(BIN_DIR)/
	@mkdir -p $(BIN_DIR) $(GOCACHE)
	$(GOENV) $(GO) build -buildvcs=false -o $(USP_BIN) ./cmd/usp
	@echo "binary: $(USP_BIN)"

build-usp-ctxt: ## Build the usp-ctxt bridge into $(BIN_DIR)/
	@mkdir -p $(BIN_DIR) $(GOCACHE)
	$(GOENV) $(GO) build -buildvcs=false -o $(USP_CTXT_BIN) ./cmd/usp-ctxt
	@echo "binary: $(USP_CTXT_BIN)"

install: build ## Install binaries into $$XDG_BIN_HOME or ~/.local/bin
	@install_dir="$(INSTALL_DIR)"; \
	  mkdir -p "$$install_dir"; \
	  cp "$(USP_BIN)" "$$install_dir/usp"; \
	  cp "$(USP_CTXT_BIN)" "$$install_dir/usp-ctxt"; \
	  echo "installed: $$install_dir/usp"; \
	  echo "installed: $$install_dir/usp-ctxt"

run: build-usp ## Run usp locally with ARGS='...'
	$(USP_BIN) $(ARGS)

test: ## Run all Go tests
	@mkdir -p $(GOCACHE)
	$(GOENV) $(GO) test $(PKG)

fmt: ## Format Go source
	@$(GO) fmt $(PKG)

vet: ## Run go vet
	@mkdir -p $(GOCACHE)
	$(GOENV) $(GO) vet $(PKG)

lint: vet ## Run lint checks (vet + golangci-lint if installed)
	@mkdir -p $(GOCACHE)
	@if command -v golangci-lint >/dev/null 2>&1; then \
	  $(GOENV) GOFLAGS="-buildvcs=false $(GOFLAGS)" golangci-lint run $(PKG); \
	else \
	  echo "golangci-lint not installed — skipping (install: https://golangci-lint.run/welcome/install/)"; \
	fi

tidy: ## Tidy Go modules
	@mkdir -p $(GOCACHE)
	$(GOENV) $(GO) mod tidy

check: build test lint ## Build, test, and lint

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) .cache coverage.out
