#!/bin/bash

# Default values
HOST="localhost"
PORT="8080"

# Process arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        --host=*)
            HOST="${1#*=}"
            shift
            ;;
        --port=*)
            PORT="${1#*=}"
            shift
            ;;
        *)
            echo "Unknown parameter: $1"
            exit 1
            ;;
    esac
done

echo "Starting video stream server on $HOST:$PORT..."
go run server/server.go -host "$HOST" -port "$PORT"
