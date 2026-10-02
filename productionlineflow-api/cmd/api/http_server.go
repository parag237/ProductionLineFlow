package main

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"productionlineflow-api/internal/auth"
	"productionlineflow-api/internal/company"
	"productionlineflow-api/internal/config"
	"productionlineflow-api/internal/people"
	"productionlineflow-api/internal/server"
	warehouseModule "productionlineflow-api/internal/warehouse"

	"github.com/jackc/pgx/v5/pgxpool"
)

func startHTTPServer(cfg *config.Config, pool *pgxpool.Pool, logger *slog.Logger) (*http.Server, <-chan error, error) {
	repository := auth.NewPostgresRepository(pool)
	jwtService := auth.NewJWTService(cfg.Auth.JWTSecret, time.Duration(cfg.Auth.AccessTTLMin)*time.Minute, cfg.Auth.JWTIssuer)
	refreshTTL := time.Duration(cfg.Auth.RefreshTTLHours) * time.Hour
	minPasswordLength := cfg.Auth.MinPasswordLength
	authService := auth.NewService(repository, jwtService, refreshTTL, minPasswordLength)
	platformAuthService := auth.NewPlatformService(repository, jwtService, refreshTTL)
	companyService := company.NewService(company.NewPostgresRepository(pool), minPasswordLength)
	warehouseService := warehouseModule.NewService(warehouseModule.NewPostgresRepository(pool))
	peopleService := people.NewService(people.NewPostgresRepository(pool), minPasswordLength)
	router := server.NewRouter(cfg, &server.Dependencies{
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
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSec) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeoutSec) * time.Second,
	}
	listener, err := net.Listen("tcp", httpServer.Addr)
	if err != nil {
		return nil, nil, fmt.Errorf("listen on %s: %w", httpServer.Addr, err)
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
	return httpServer, serverErrors, nil
}
