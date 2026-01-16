#!/bin/bash

# Simple file watcher for Go development
# Watches for .go file changes and automatically restarts the server

echo "Watching for Go file changes..."
echo "Press Ctrl+C to stop"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to start server
start_server() {
    echo -e "${GREEN}Starting server...${NC}"
    go run main.go &
    SERVER_PID=$!
}

# Function to stop server
stop_server() {
    if [ ! -z "$SERVER_PID" ]; then
        echo -e "${YELLOW}Stopping server (PID: $SERVER_PID)...${NC}"
        kill $SERVER_PID 2>/dev/null
        wait $SERVER_PID 2>/dev/null
    fi
}

# Trap Ctrl+C
trap 'stop_server; exit' INT TERM

# Start server initially
start_server

# Watch for changes
if command -v fswatch &> /dev/null; then
    # macOS - use fswatch
    fswatch -o --exclude='.*' --include='\.go$' . | while read f; do
        stop_server
        sleep 1
        start_server
    done
elif command -v inotifywait &> /dev/null; then
    # Linux - use inotifywait
    while inotifywait -e modify -r --exclude='\.git|node_modules|tmp|frontend' --include='\.go$' .; do
        stop_server
        sleep 1
        start_server
    done
else
    echo "No file watcher found. Install 'fswatch' (macOS) or 'inotifywait' (Linux)"
    echo "   macOS: brew install fswatch"
    echo "   Linux: sudo apt-get install inotify-tools"
    echo ""
    echo "Running server without auto-reload. Press Ctrl+C to stop."
    wait $SERVER_PID
fi

