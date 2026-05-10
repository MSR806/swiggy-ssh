package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"swiggy-ssh/internal/application/auth"
)

// Server is the HTTP delivery adapter for browser-facing login flows.
type Server struct {
	addr   string
	logger *slog.Logger
	svc    auth.LoginCodeService
}

// New constructs an HTTP server bound to addr, backed by svc.
func New(addr string, logger *slog.Logger, svc auth.LoginCodeService) *Server {
	return &Server{addr: addr, logger: logger, svc: svc}
}

// Start listens on s.addr and blocks until ctx is cancelled or a listen error occurs.
// On context cancellation the server shuts down gracefully (up to 10 s).
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /login", s.handleLoginGet)
	mux.HandleFunc("POST /login", s.handleLoginPost)

	srv := &http.Server{
		Addr:         s.addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.InfoContext(ctx, "http server listening", "addr", s.addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
		return nil
	case err := <-errCh:
		return err
	}
}
