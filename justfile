# USP — Universal Sessions Protocol

# Show available targets
default:
    @just --list

# ── Build ──────────────────────────────────────────────────

# Build binary to ./bin/usp
build:
    go build -o ./bin/usp ./cmd/usp/

# Install binary via go install
install:
    go install ./cmd/usp/

# ── Lint & Vet ─────────────────────────────────────────────

# Run golangci-lint
lint:
    golangci-lint run ./...

# Run go vet
vet:
    go vet ./...

# ── Test ───────────────────────────────────────────────────

# Run all tests
test:
    go test ./...

# ── Gate ───────────────────────────────────────────────────

# Full pre-merge gate (lint + vet + test)
check: lint vet test

# ── Clean ──────────────────────────────────────────────────

# Remove build artifacts
clean:
    rm -rf ./bin/
