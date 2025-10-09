#!/bin/bash

# BuildPrize Quiz Go Test Runner
# Runs the Go test suite for the backend

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Check if Go is installed
if ! command -v go &> /dev/null; then
    print_error "Go is not installed. Please install Go first."
    exit 1
fi

# Check if we're in the right directory
if [ ! -f "go.mod" ]; then
    print_error "go.mod not found. Please run this script from the project root."
    exit 1
fi

print_status "🚀 Starting BuildPrize Quiz Go Tests"
echo "=================================================="

# Clean up any existing processes on port 8080
print_status "Cleaning up port 8080..."
lsof -ti:8080 | xargs kill -9 2>/dev/null || true

# Run the tests
print_status "Running Go test suite..."
echo ""

if go test ./internal/testing -v; then
    print_success "🎉 All Go tests passed!"
    echo "=================================================="
    print_status "Backend is working correctly!"
    print_status "You can now:"
    print_status "  - Create lobbies via API"
    print_status "  - Join lobbies with players"
    print_status "  - Start games and submit answers"
    print_status "  - Use WebSocket for real-time updates"
else
    print_error "❌ Some tests failed!"
    exit 1
fi
