package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var version = "dev" // overridden at build time via -ldflags "-X main.version=x.y.z"

type server struct {
	hostname string
}

type rootResponse struct {
	Message  string `json:"message"`
	Version  string `json:"version"`
	Hostname string `json:"hostname"`
}

func newServer() (*server, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("get hostname: %w", err)
	}
	return &server{hostname: hostname}, nil
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleRoot)
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /readyz", handleReadyz)
	return mux
}

func (s *server) handleRoot(w http.ResponseWriter, r *http.Request) {
	resp := rootResponse{
		Message:  "hello",
		Version:  version,
		Hostname: s.hostname,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// Header already sent; log and move on — nothing else we can do.
		slog.ErrorContext(r.Context(), "encode response", "err", err)
	}
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

func handleReadyz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ready")
}

func port() string {
	if p := os.Getenv("PORT"); p != "" {
		return p
	}
	return "8080"
}

func run() error {
	// Structured JSON logging to stdout — expected by log aggregators in Kubernetes.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	srv, err := newServer()
	if err != nil {
		return fmt.Errorf("init server: %w", err)
	}

	addr := net.JoinHostPort("", port())
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      srv.routes(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Channel closed by the signal handler to trigger graceful shutdown.
	quit := make(chan struct{})
	// errCh carries the ListenAndServe result back to the main goroutine.
	errCh := make(chan error, 1)

	go func() {
		slog.Info("listening", "addr", addr, "version", version)
		if err := httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("listen: %w", err)
		}
		close(errCh)
	}()

	// Block until SIGINT or SIGTERM is received.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-sigCh:
		slog.Info("shutting down", "signal", sig)
	case err := <-errCh:
		// Server failed before any signal — surface the error immediately.
		return err
	}
	close(quit)

	// Give in-flight requests up to 10 s to finish before forcibly closing.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	// Drain errCh so the goroutine does not leak.
	if err := <-errCh; err != nil {
		return err
	}

	slog.Info("shutdown complete")
	return nil
}

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}
