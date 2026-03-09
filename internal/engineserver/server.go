// Package engineserver provides the HTTP server for the policy evaluation engine API.
package engineserver

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	engineserverapi "github.com/dcm-project/policy-manager/api/v1alpha1/engine"
	engineserver "github.com/dcm-project/policy-manager/internal/api/engine"
	"github.com/dcm-project/policy-manager/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const gracefulShutdownTimeout = 5 * time.Second

// Server wraps the HTTP server for the engine API
type Server struct {
	config   *config.Config
	listener net.Listener
	handler  engineserver.StrictServerInterface
}

// New creates a new engine server instance
func New(cfg *config.Config, listener net.Listener, handler engineserver.StrictServerInterface) *Server {
	return &Server{
		config:   cfg,
		listener: listener,
		handler:  handler,
	}
}

// Run starts the HTTP server and blocks until shutdown
func (s *Server) Run(ctx context.Context) error {
	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	swagger, err := engineserverapi.GetSwagger()
	if err != nil {
		return fmt.Errorf("failed to load swagger spec: %w", err)
	}

	baseURL := ""
	if len(swagger.Servers) > 0 {
		baseURL = swagger.Servers[0].URL
	}

	engineserver.HandlerFromMuxWithBaseURL(
		engineserver.NewStrictHandler(s.handler, nil),
		router,
		baseURL,
	)

	srv := &http.Server{Handler: router}

	go func() {
		<-ctx.Done()
		ctxTimeout, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
		defer cancel()
		srv.SetKeepAlivesEnabled(false)
		log.Println("Shutting down engine server...")
		_ = srv.Shutdown(ctxTimeout)
	}()

	log.Printf("Starting engine server on %s", s.listener.Addr())
	if err := srv.Serve(s.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("failed to serve engine API server: %w", err)
	}

	log.Println("Engine server stopped successfully")
	return nil
}
