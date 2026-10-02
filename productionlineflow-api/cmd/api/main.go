package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"productionlineflow-api/internal/config"
	"productionlineflow-api/internal/logging"
	database "productionlineflow-api/internal/platform/db"
)

func main() {
	if err := run(); err != nil {
		logging.Default().Error("service exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger, err := logging.New(cfg.Log.Level, cfg.Log.Format)
	if err != nil {
		return fmt.Errorf("configure logger: %w", err)
	}
	logging.SetDefault(logger)
	logger.Info("service starting", "environment", cfg.Env)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	logger.Info("connecting to database", "max_connections", cfg.Database.MaxConns)
	pool, err := database.Open(ctx, cfg.Database.DSN, int32(cfg.Database.MaxConns))
	cancel()
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		pool.Close()
		logger.Info("database connection pool closed")
	}()
	logger.Info("database connected", "max_connections", cfg.Database.MaxConns)

	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	httpServer, serverErrors, err := startHTTPServer(cfg, pool, logger)
	if err != nil {
		return err
	}

	select {
	case err := <-serverErrors:
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-signalContext.Done():
		logger.Info("shutdown signal received")
	}

	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := httpServer.Shutdown(shutdownContext); err != nil {
		_ = httpServer.Close()
		return fmt.Errorf("gracefully shut down HTTP server: %w", err)
	}
	if err := <-serverErrors; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP during shutdown: %w", err)
	}
	logger.Info("shutdown complete")
	return nil
}
