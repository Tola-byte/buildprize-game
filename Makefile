# BuildPrize Quiz Makefile

.PHONY: test test-verbose build run clean help

# Default target
all: test

# Run tests
test:
	@echo "🚀 Running BuildPrize Quiz Tests"
	@echo "=================================="
	@go test ./internal/testing -v

# Run tests with verbose output
test-verbose:
	@echo "🚀 Running BuildPrize Quiz Tests (Verbose)"
	@echo "=========================================="
	@go test ./internal/testing -v -count=1

# Build the application
build:
	@echo "🔨 Building BuildPrize Quiz..."
	@go build -o buildprize-game .

# Run the application
run:
	@echo "🚀 Starting BuildPrize Quiz Server..."
	@go run main.go

# Run with auto-reload (requires air or watch.sh)
dev:
	@if command -v air &> /dev/null; then \
		echo "🔄 Starting with Air (auto-reload)..."; \
		air; \
	elif [ -f "./watch.sh" ]; then \
		echo "🔄 Starting with watch.sh (auto-reload)..."; \
		./watch.sh; \
	else \
		echo "⚠️  Auto-reload not available. Install 'air' or use 'make run'"; \
		echo "   Install air: go install github.com/cosmtrek/air@latest"; \
		echo "   Or use: make run"; \
	fi

# Clean build artifacts
clean:
	@echo "🧹 Cleaning up..."
	@rm -f buildprize-game
	@go clean

# Run tests and show help
help:
	@echo "BuildPrize Quiz - Available Commands:"
	@echo ""
	@echo "  make test         - Run all tests"
	@echo "  make test-verbose - Run tests with verbose output"
	@echo "  make build        - Build the application"
	@echo "  make run          - Run the application (no auto-reload)"
	@echo "  make dev          - Run with auto-reload (recommended for development)"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make help         - Show this help"
	@echo ""
	@echo "Quick Start:"
	@echo "  make test    # Test the backend"
	@echo "  make run     # Start the server"
