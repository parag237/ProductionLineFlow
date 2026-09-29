package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"productionlineflow-api/internal/auth"
	"productionlineflow-api/internal/config"
	database "productionlineflow-api/internal/platform/db"
	"productionlineflow-api/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, cfg.Database.DSN, int32(cfg.Database.MaxConns))
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	repository := auth.NewPostgresRepository(pool)
	jwtService := auth.NewJWTService(cfg.Auth.JWTSecret, time.Duration(cfg.Auth.AccessTTLMin)*time.Minute, cfg.Auth.JWTIssuer)
	authService := auth.NewService(repository, jwtService, time.Duration(cfg.Auth.RefreshTTLHours)*time.Hour, cfg.Auth.MinPasswordLength)
	r := server.NewRouter(cfg, &server.Dependencies{
		Auth:         authService,
		JWT:          jwtService,
		Repository:   repository,
		SecureCookie: cfg.Env == "prod",
	})
	if err := r.Run(fmt.Sprintf(":%d", cfg.Server.Port)); err != nil {
		log.Fatal(err)
	}
}
