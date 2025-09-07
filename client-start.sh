#!/bin/bash

# Default values
HOST="localhost"
PORT="8081"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--host)
      HOST="$2"
      shift 2
      ;;
    -p|--port)
      PORT="$2"
      shift 2
      ;;
    *)
      echo "Unknown parameter: $1"
      exit 1
      ;;
  esac
done

echo "Starting client server on ${HOST}:${PORT}..."
go run client/client.go -host "${HOST}" -port "${PORT}"
