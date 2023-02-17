// server starts an HTTP Server to run the application
package server

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/go-playground/errors/v5"
	"github.com/jtwatson/shutdown"
)

const shutdownHammer = time.Second * 5

// Server contains an HTTP Server for running the AppServer
type Server struct {
	srv *http.Server
}

// New configures and returns a Server
func New(addr string) *Server {
	return &Server{
		srv: &http.Server{
			Addr:              addr,
			ReadHeaderTimeout: 60 * time.Second,
		},
	}
}

// Start starts up an HTTP Server with appServer as its handler.
func (s *Server) Start(ctx context.Context, handler http.Handler) error {
	// Capture interrupts so we can handle them gracefully.
	ctx, cancel := shutdown.CaptureInterrupts(ctx)

	log.Printf("Starting Server at %s", s.srv.Addr)
	defer log.Print("Server Exited")

	errChan := make(chan error, 1)
	go func() {
		defer cancel()
		defer close(errChan)

		s.srv.Handler = handler
		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- errors.Wrap(err, "http.Server shutdown abnormally")
		}
	}()

	<-ctx.Done()
	select {
	case err := <-errChan:
		return err
	default:
	}

	ctxShutDown, cancelShutDown := context.WithTimeout(context.Background(), shutdownHammer)
	defer cancelShutDown()

	if err := s.srv.Shutdown(ctxShutDown); err != nil {
		return errors.Wrap(err, "http.Server.Shutdown(): didn't shutdown gracefully")
	}

	return nil
}
