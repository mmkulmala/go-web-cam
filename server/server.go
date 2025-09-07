	package main

	import (
	    "context"
	    "fmt"
	    "log"
	    "net/http"
	    "os"
	    "os/signal"
	    "sync"
	    "syscall"
	    "time"

	    "gocv.io/x/gocv"
	)

	// StreamServer handles the MJPEG streaming
	type StreamServer struct {
	    sync.RWMutex
	    bound     chan struct{}
	    clients   map[chan []byte]bool
	    frame     []byte
	    webcam    *gocv.VideoCapture
	    webcamErr error
	    isRunning bool
	}

	// NewStreamServer creates a new StreamServer instance
	func NewStreamServer() *StreamServer {
	    return &StreamServer{
	        bound:   make(chan struct{}),
	        clients: make(map[chan []byte]bool),
	    }
	}

	// OpenCamera tries to open the specified camera device
	func (s *StreamServer) OpenCamera(deviceID int) error {
	    s.Lock()
	    defer s.Unlock()

	    if s.webcam != nil {
	        s.webcam.Close()
	        s.webcam = nil
	    }

	    webcam, err := gocv.OpenVideoCapture(deviceID)
	    if err != nil {
	        s.webcamErr = fmt.Errorf("error opening video capture device: %v", err)
	        return s.webcamErr
	    }

	    s.webcam = webcam
	    s.webcamErr = nil
	    log.Printf("Successfully opened webcam on device index %d", deviceID)
	    return nil
	}

	// Start begins capturing frames from the webcam
	func (s *StreamServer) Start(ctx context.Context) {
	    s.Lock()
	    if s.isRunning {
	        s.Unlock()
	        return
	    }
	    s.isRunning = true
	    s.Unlock()

	    close(s.bound)
	    go s.captureFrames(ctx)
	}

	// Stop stops the frame capture goroutine
	func (s *StreamServer) Stop() {
	    s.Lock()
	    defer s.Unlock()

	    if !s.isRunning {
	        return
	    }
	    s.isRunning = false

	    // Close the webcam
	    if s.webcam != nil {
	        s.webcam.Close()
	        s.webcam = nil
	    }

	    // Close all client channels
	    for clientChan := range s.clients {
	        close(clientChan)
	    }
	    s.clients = make(map[chan []byte]bool)
	    s.bound = make(chan struct{})
	}

	// captureFrames continuously captures frames from the webcam
	func (s *StreamServer) captureFrames(ctx context.Context) {
	    defer func() {
	        s.Lock()
	        s.isRunning = false
	        s.Unlock()
	    }()

	    // Wait for binding to complete
	    <-s.bound

	    img := gocv.NewMat()
	    defer img.Close()

	    // Use a ticker to control frame rate
	    ticker := time.NewTicker(33 * time.Millisecond) // ~30 FPS
	    defer ticker.Stop()

	    for {
	        select {
	        case <-ctx.Done():
	            return
	        case <-ticker.C:
	            s.Lock()
	            if s.webcam == nil {
	                s.Unlock()
	                continue
	            }

	            if ok := s.webcam.Read(&img); !ok || img.Empty() {
	                s.webcamErr = fmt.Errorf("failed to read frame from webcam")
	                s.Unlock()
	                time.Sleep(100 * time.Millisecond)
	                continue
	            }

	            // Convert the image to JPEG
	            buf, err := gocv.IMEncode(".jpg", img)
	            if err != nil {
	                s.webcamErr = fmt.Errorf("failed to encode frame: %v", err)
	                s.Unlock()
	                continue
	            }

	            // Update current frame and broadcast to clients
	            s.frame = buf.GetBytes()
	            s.broadcastFrame()
	            buf.Close()
	            s.Unlock()
	        }
	    }
	}

	// broadcastFrame sends the current frame to all connected clients
	// Assumes the caller holds the lock
	func (s *StreamServer) broadcastFrame() {
	    if s.frame == nil || len(s.clients) == 0 {
	        return
	    }

	    for client := range s.clients {
	        select {
	        case client <- s.frame:
	            // Frame sent successfully
	        default:
	            // Client buffer is full, remove it
	            delete(s.clients, client)
	            close(client)
	        }
	    }
	}

	// addClient adds a new client channel to the clients map
	func (s *StreamServer) addClient() chan []byte {
	    s.Lock()
	    defer s.Unlock()

	    // Create a new client channel with a buffer
	    clientChan := make(chan []byte, 5)
	    s.clients[clientChan] = true

	    // Send the current frame immediately if available
	    if s.frame != nil {
	        select {
	        case clientChan <- s.frame:
	            // Frame sent successfully
	        default:
	            // Buffer is full, which shouldn't happen for a new client
	        }
	    }

	    log.Printf("New client connected. Total clients: %d", len(s.clients))
	    return clientChan
	}

	// removeClient removes a client channel from the clients map
	func (s *StreamServer) removeClient(clientChan chan []byte) {
	    s.Lock()
	    defer s.Unlock()

	    if _, ok := s.clients[clientChan]; ok {
	        delete(s.clients, clientChan)
	        close(clientChan)
	        log.Printf("Client disconnected. Total clients: %d", len(s.clients))
	    }
	}

	// serveStream handles HTTP requests for the MJPEG stream
	func (s *StreamServer) serveStream(w http.ResponseWriter, r *http.Request) {
	    s.RLock()
	    if s.webcam == nil {
	        s.RUnlock()
	        http.Error(w, "Camera not available", http.StatusServiceUnavailable)
	        return
	    }
	    s.RUnlock()

	    // Set the content type for multipart JPEG
	    w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")
	    w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	    w.Header().Set("Connection", "close")

	    // Add this client to our map
	    clientChan := s.addClient()
	    defer s.removeClient(clientChan)

	    // Flush the initial headers immediately
	    if f, ok := w.(http.Flusher); ok {
	        f.Flush()
	    }

	    // Create a notification channel for detecting a closed connection
	    disconnected := r.Context().Done()

	    // Write frames to the response as they come
	    for {
	        select {
	        case <-disconnected:
	            return
	        case frame, ok := <-clientChan:
	            if !ok {
	                return
	            }

	            // Write the frame with MJPEG format
	            _, err := w.Write([]byte("--frame\r\nContent-Type: image/jpeg\r\n\r\n"))
	            if err != nil {
	                return
	            }

	            _, err = w.Write(frame)
	            if err != nil {
	                return
	            }

	            _, err = w.Write([]byte("\r\n"))
	            if err != nil {
	                return
	            }

	            // Flush the data to the client
	            if f, ok := w.(http.Flusher); ok {
	                f.Flush()
	            } else {
	                return
	            }
	        }
	    }
	}

	// getCORSMiddleware returns a middleware that adds CORS headers
	func getCORSMiddleware(next http.Handler) http.Handler {
	    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	        // Set CORS headers
	        w.Header().Set("Access-Control-Allow-Origin", "*")
	        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	        w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	        // Handle preflight requests
	        if r.Method == "OPTIONS" {
	            w.WriteHeader(http.StatusOK)
	            return
	        }

	        next.ServeHTTP(w, r)
	    })
	}

	func main() {
	    // Create a new stream server
	    server := NewStreamServer()

	    // Try to open the default camera
	    if err := server.OpenCamera(0); err != nil {
	        log.Printf("Warning: failed to open camera: %v", err)
	        log.Println("Server will start but streaming will be unavailable until a camera is connected")
	    }

	    // Create a context for controlling shutdown
	    ctx, cancel := context.WithCancel(context.Background())

	    // Start capturing frames
	    server.Start(ctx)

	    // Create a new HTTP server
	    mux := http.NewServeMux()

	    // CORS middleware for static files
	    staticHandler := getCORSMiddleware(http.FileServer(http.Dir("server/static")))
	    mux.Handle("/", staticHandler)

	    // Stream endpoint
	    mux.HandleFunc("/stream", server.serveStream)

	    // Health check endpoint
	    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
	        server.RLock()
	        err := server.webcamErr
	        isRunning := server.isRunning
	        server.RUnlock()

	        w.Header().Set("Content-Type", "application/json")
	        
	        if !isRunning {
	            w.WriteHeader(http.StatusServiceUnavailable)
	            fmt.Fprintf(w, `{"status":"error","message":"Stream server is not running"}`)
	            return
	        }

	        if err != nil {
	            w.WriteHeader(http.StatusServiceUnavailable)
	            fmt.Fprintf(w, `{"status":"error","message":%q}`, err.Error())
	            return
	        }
	        
	        w.WriteHeader(http.StatusOK)
	        fmt.Fprintf(w, `{"status":"ok","clients":%d}`, len(server.clients))
	    })

	    // Create HTTP server with configuration
	    httpServer := &http.Server{
	        Addr:    ":8080",
	        Handler: mux,
	    }

	    // Channel to listen for errors coming from the listener
	    serverErrors := make(chan error, 1)

	    // Start the server
	    go func() {
	        log.Printf("Starting video streaming server on http://localhost:8080")
	        serverErrors <- httpServer.ListenAndServe()
	    }()

	    // Channel for handling OS signals
	    osSignals := make(chan os.Signal, 1)
	    signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM)

	    // Block until we receive a signal or server error
	    select {
	    case err := <-serverErrors:
	        if err != nil && err != http.ErrServerClosed {
	            log.Fatalf("Error starting server: %v", err)
	        }
	    case <-osSignals:
	        log.Println("Received OS signal, shutting down server...")
	        
	        // Cancel context to stop frame capture
	        cancel()
	        
	        // Create a shutdown context with a timeout
	        shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	        defer shutdownCancel()
	        
	        // Stop the stream server
	        server.Stop()
	        
	        // Attempt to gracefully shutdown the server
	        if err := httpServer.Shutdown(shutdownCtx); err != nil {
	            log.Printf("Error during server shutdown: %v", err)
	            httpServer.Close()
	        }
	        
	        log.Println("Server shutdown complete")
	    }
	}
