package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"productionlineflow-api/internal/auth"
	"productionlineflow-api/internal/company"
	"productionlineflow-api/internal/config"
	"productionlineflow-api/internal/people"
	database "productionlineflow-api/internal/platform/db"
	"productionlineflow-api/internal/server"
	warehouseModule "productionlineflow-api/internal/warehouse"
)

func main() {
	if err := run(); err != nil {
		slog.Error("service exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger, err := newLogger(cfg.Log)
	if err != nil {
		return fmt.Errorf("configure logger: %w", err)
	}
	slog.SetDefault(logger)
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

	repository := auth.NewPostgresRepository(pool)
	jwtService := auth.NewJWTService(cfg.Auth.JWTSecret, time.Duration(cfg.Auth.AccessTTLMin)*time.Minute, cfg.Auth.JWTIssuer)
	refreshTTL := time.Duration(cfg.Auth.RefreshTTLHours) * time.Hour
	minPasswordLength := cfg.Auth.MinPasswordLength
	authService := auth.NewService(repository, jwtService, refreshTTL, minPasswordLength)
	platformAuthService := auth.NewPlatformService(repository, jwtService, refreshTTL)
	companyService := company.NewService(company.NewPostgresRepository(pool), minPasswordLength)
	warehouseService := warehouseModule.NewService(warehouseModule.NewPostgresRepository(pool))
	peopleService := people.NewService(people.NewPostgresRepository(pool), minPasswordLength)
	r := server.NewRouter(cfg, &server.Dependencies{
		Auth:               authService,
		PlatformAuth:       platformAuthService,
		JWT:                jwtService,
		Repository:         repository,
		PlatformRepository: repository,
		Company:            companyService,
		Warehouse:          warehouseService,
		People:             peopleService,
		SecureCookie:       cfg.Env == "prod",
	})
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSec) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeoutSec) * time.Second,
	}
	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	listener, err := net.Listen("tcp", httpServer.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", httpServer.Addr, err)
	}
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- httpServer.Serve(listener)
	}()
	logger.Info("HTTP server listening",
		"address", httpServer.Addr,
		"port", cfg.Server.Port,
		"read_timeout", httpServer.ReadTimeout,
		"write_timeout", httpServer.WriteTimeout,
	)

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

func newLogger(cfg config.LogConfig) (*slog.Logger, error) {
	var level slog.Level
	switch strings.ToLower(strings.TrimSpace(cfg.Level)) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return nil, fmt.Errorf("unsupported log level %q", cfg.Level)
	}

	options := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	switch strings.ToLower(strings.TrimSpace(cfg.Format)) {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, options)
	case "text":
		handler = slog.NewTextHandler(os.Stdout, options)
	default:
		return nil, fmt.Errorf("unsupported log format %q", cfg.Format)
	}
	return slog.New(handler), nil
}
