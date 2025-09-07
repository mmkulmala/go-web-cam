		package main

		import (
		        "context"
		        "log"
		        "net/http"
		        "os"
		        "os/signal"
		        "syscall"
		        "time"
		)
		func main() {
		        mux := http.NewServeMux()

		        // Serve static files
		        fs := http.FileServer(http.Dir("client/static"))
		        mux.Handle("/", fs)

		        // Create server
		        server := &http.Server{
		                Addr:         ":8081",
		                Handler:      mux,
		                ReadTimeout:  15 * time.Second,
		                WriteTimeout: 15 * time.Second,
		                IdleTimeout:  60 * time.Second,
		        }

		        // Channel to listen for errors coming from the listener.
		        serverErrors := make(chan error, 1)

		        // Start the server
		        go func() {
		                log.Printf("Client server is starting on :%s", "8081")
		                serverErrors <- server.ListenAndServe()
		        }()

		        // Channel to listen for an interrupt or terminate signal from the OS.
		        shutdown := make(chan os.Signal, 1)
		        signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

		        // Blocking main and waiting for shutdown.
		        select {
		        case err := <-serverErrors:
		                log.Fatalf("Error starting server: %v", err)

		        case <-shutdown:
		                log.Println("Client server is shutting down...")

		                // Give outstanding requests a deadline for completion.
		                ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		                defer cancel()

		                // Asking listener to shut down and shed load.
		                if err := server.Shutdown(ctx); err != nil {
		                        log.Printf("Could not stop server gracefully: %v", err)
		                        if err := server.Close(); err != nil {
		                                log.Printf("Could not force close server: %v", err)
		                        }
		                }
		        }
		}

