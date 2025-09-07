#!/bin/bash

# Exit on error, undefined vars
set -eu

# Default configuration
# Server configuration
SERVER_HOST="${1:-localhost}"
SERVER_PORT="${2:-8080}"
# Client configuration
CLIENT_HOST="localhost"
CLIENT_PORT="8081"
MAX_RETRIES=30
RETRY_INTERVAL=1

# Cleanup function to kill background processes
cleanup() {
    echo "Cleaning up processes..."
    # Kill server if running
    if [ -n "${SERVER_PID:-}" ]; then
        echo "Stopping server (PID: $SERVER_PID)..."
        kill $SERVER_PID 2>/dev/null || true
        wait $SERVER_PID 2>/dev/null || true
    fi
    # Kill client if running
    if [ -n "${CLIENT_PID:-}" ]; then
        echo "Stopping client (PID: $CLIENT_PID)..."
        kill $CLIENT_PID 2>/dev/null || true
        wait $CLIENT_PID 2>/dev/null || true
    fi
    exit 0
}

# Error handler
error() {
    echo "ERROR: $1" >&2
    cleanup
    exit 1
}

# Health check function
check_server_health() {
    curl -s "http://${SERVER_HOST}:${SERVER_PORT}/health" > /dev/null
    return $?
}

# Set trap for cleanup
trap cleanup EXIT INT TERM

echo "Starting server on ${SERVER_HOST}:${SERVER_PORT}..."
./server-start.sh --host="${SERVER_HOST}" --port="${SERVER_PORT}" &
SERVER_PID=$!

echo "Starting client on ${CLIENT_HOST}:${CLIENT_PORT}..."
./client-start.sh --host "${CLIENT_HOST}" --port "${CLIENT_PORT}" &
CLIENT_PID=$!

echo "Waiting for server to become healthy..."
retries=0
while ! check_server_health; do
    retries=$((retries + 1))
    if [ $retries -ge $MAX_RETRIES ]; then
        error "Server failed to become healthy after $MAX_RETRIES attempts"
    fi
    echo "Waiting for server to start (attempt $retries/$MAX_RETRIES)..."
    sleep $RETRY_INTERVAL
done

echo "Server is healthy!"
echo "Access the server at http://${SERVER_HOST}:${SERVER_PORT}"
echo "Access the client at http://${CLIENT_HOST}:${CLIENT_PORT}"
echo "Press Ctrl+C to stop both the server and client"

# Monitor both processes
while true; do
    # Check if server is still running
    if ! kill -0 $SERVER_PID 2>/dev/null; then
        error "Server process has terminated unexpectedly"
    fi
    
    # Check if client is still running
    if ! kill -0 $CLIENT_PID 2>/dev/null; then
        error "Client process has terminated unexpectedly"
    fi
    
    sleep 5
done
