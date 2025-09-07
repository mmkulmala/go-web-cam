# WARP.md

This file provides guidance to WARP (warp.dev) when working with code in this repository.

Repository overview
- Language/tooling: Go (module: webcam-app, go 1.24.0)
- Purpose: Simple webcam streaming app. A server captures frames from a local camera and serves an MJPEG stream; a client serves a small UI that displays the stream.
- Layout:
  - server: HTTP server and webcam capture/streaming logic
  - client: Static HTTP server hosting a page that pulls the stream from the server
  - start.sh: Orchestrates starting both server and client and performs a health check

Common commands
- Build
  - Build server: go build -o bin/server ./server
  - Build client: go build -o bin/client ./client
- Run
  - Start both (recommended): ./start.sh
  - Start server only: ./server-start.sh [--host=HOST] [--port=PORT]
  - Start client only: ./client-start.sh [-h HOST] [-p PORT]
  - UI endpoints:
    - Server UI: http://localhost:8080 (serves static page and stream)
    - Client UI: http://localhost:8081 (web page that points at the server stream)
  - Note: As of the current code, the Go programs listen on fixed ports :8080 (server) and :8081 (client). The scripts accept host/port flags, but the programs do not parse them yet.
- Test
  - All packages: go test ./...
  - Single test in a package: go test ./server -run '^TestName$'
  - Currently there are no *_test.go files, so these commands will no-op until tests are added.
- Lint/format (built-in tooling)
  - Static analysis: go vet ./...
  - Formatting: go fmt ./...

High-level architecture
- Server (server/server.go)
  - StreamServer owns webcam capture state and fan-out of frames to clients:
    - clients: map[chan []byte]bool of subscribers; frames are pushed via buffered channels
    - frame: latest JPEG-encoded frame (updated ~30 FPS via time.Ticker)
    - webcam: *gocv.VideoCapture; opened with OpenCamera(0) on startup
    - isRunning/bound: guard start/shutdown of the capture loop
  - Capture loop (captureFrames):
    - Reads frames from the camera, encodes to JPEG (gocv.IMEncode), updates current frame, and broadcasts to all client channels without blocking; slow clients are dropped
  - HTTP endpoints (net/http on :8080):
    - /: Serves static files from server/static (simple viewer pointing to /stream)
    - /stream: Multipart MJPEG stream; writes frame boundaries with flushes for each frame
    - /health: JSON health with running status and last webcam error (if any)
  - Graceful shutdown: Context cancellation stops capture; server.Shutdown with timeout; cleans up webcam and client channels
- Client (client/client.go)
  - Simple static file server on :8081 serving client/static
  - The page (client/static/index.html) tries to load http://localhost:8080/stream, shows connection status, and retries a few times on errors
- Orchestration (start.sh)
  - Starts server and client in the background, polls /health until healthy, prints access URLs, and monitors both PIDs; ensures cleanup on exit

Operational notes
- Dependencies: The server uses gocv (gocv.io/x/gocv). On some systems, building/running may require OpenCV native libraries to be installed. If you see gocv build/link errors, consult the gocv installation instructions for your OS.
- Camera availability: If the webcam cannot be opened, the server still starts and /health reports an error until a camera is connected. Streaming remains unavailable until then.
- Ports/endpoints (defaults):
  - Server: :8080 → / (static), /stream (MJPEG), /health (JSON)
  - Client: :8081 → / (static page referencing the server stream)

Important from README.md
- Quick start: ./start.sh starts both server and client and provides a basic webcam app usable for video communication
- You can also run the two services independently via ./server-start.sh and ./client-start.sh

