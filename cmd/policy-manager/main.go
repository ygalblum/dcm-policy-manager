package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/dcm-project/policy-manager/internal/apiserver"
	"github.com/dcm-project/policy-manager/internal/config"
	"github.com/dcm-project/policy-manager/internal/engineserver"
	"github.com/dcm-project/policy-manager/internal/handlers/engine"
	"github.com/dcm-project/policy-manager/internal/handlers/v1alpha1"
	"github.com/dcm-project/policy-manager/internal/logging"
	"github.com/dcm-project/policy-manager/internal/opa"
	"github.com/dcm-project/policy-manager/internal/service"
	"github.com/dcm-project/policy-manager/internal/store"
)

type Server interface {
	Run(ctx context.Context) error
}

func main() {
	os.Exit(run())
}

func run() int {
	// Load configuration from environment
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		return 1
	}

	// Initialize structured logging
	logging.Init(cfg.Service.LogLevel)

	slog.Info("Configuration loaded",
		"bind_address", cfg.Service.BindAddress,
		"engine_bind_address", cfg.Service.EngineBindAddress,
		"log_level", cfg.Service.LogLevel,
		"db_type", cfg.Database.Type,
		"db_host", cfg.Database.Hostname,
		"opa_url", cfg.OPA.URL,
	)

	// Initialize database
	db, err := store.InitDB(cfg)
	if err != nil {
		slog.Error("Failed to initialize database", "error", err)
		return 1
	}
	slog.Info("Database initialized", "type", cfg.Database.Type)

	// Create store
	dataStore := store.NewStore(db)
	defer func() {
		if err := dataStore.Close(); err != nil {
			slog.Error("Error closing database", "error", err)
		}
	}()

	// Parse OPA timeout
	opaTimeout, err := time.ParseDuration(cfg.OPA.Timeout)
	if err != nil {
		slog.Error("Failed to parse OPA timeout", "error", err, "timeout", cfg.OPA.Timeout)
		return 1
	}

	// Initialize OPA client
	opaClient := opa.NewClient(cfg.OPA.URL, opaTimeout)
	slog.Info("OPA client initialized", "url", cfg.OPA.URL, "timeout", opaTimeout)

	// Create services
	policyService := service.NewPolicyService(dataStore, opaClient)
	evaluationService := service.NewEvaluationService(dataStore.Policy(), opaClient)

	// Create public API handler
	policyHandler := v1alpha1.NewPolicyHandler(policyService)

	// Create public API TCP listener
	publicListener, err := net.Listen("tcp", cfg.Service.BindAddress)
	if err != nil {
		slog.Error("Failed to create public API listener", "error", err, "address", cfg.Service.BindAddress)
		return 1
	}
	defer func() { _ = publicListener.Close() }()

	// Create public API server
	publicSrv := apiserver.New(cfg, publicListener, policyHandler)

	// Create engine API handler
	engineHandler := engine.NewHandler(evaluationService)

	// Create private engine API TCP listener
	engineListener, err := net.Listen("tcp", cfg.Service.EngineBindAddress)
	if err != nil {
		slog.Error("Failed to create engine API listener", "error", err, "address", cfg.Service.EngineBindAddress)
		return 1
	}
	defer func() { _ = engineListener.Close() }()

	// Create private engine API server
	engineSrv := engineserver.New(cfg, engineListener, engineHandler)

	slog.Info("Starting servers")
	if err := runServers([]Server{publicSrv, engineSrv}); err != nil {
		return 1
	}

	return 0
}

func runServers(servers []Server) error {
	// Setup signal handling for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var wg sync.WaitGroup
	errChan := make(chan error, len(servers))
	for _, server := range servers {
		wg.Add(1)
		go func(server Server) {
			defer wg.Done()
			if err := server.Run(ctx); err != nil {
				errChan <- err
			}
		}(server)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	var firstErr error
	for err := range errChan {
		if err != nil {
			if firstErr == nil {
				firstErr = err
				cancel()
			}
			slog.Error("Server error", "error", err)
		}
	}

	return firstErr
}
