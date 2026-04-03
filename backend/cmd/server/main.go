package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"tanabata/backend/internal/config"
	"tanabata/backend/internal/db/postgres"
	"tanabata/backend/internal/handler"
	"tanabata/backend/internal/service"
	"tanabata/backend/migrations"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	pool, err := postgres.NewPool(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	slog.Info("database connected")

	migDB := stdlib.OpenDBFromPool(pool)
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		slog.Error("goose dialect error", "err", err)
		os.Exit(1)
	}
	if err := goose.Up(migDB, "."); err != nil {
		slog.Error("migrations failed", "err", err)
		os.Exit(1)
	}
	migDB.Close()
	slog.Info("migrations applied")

	// Repositories
	userRepo    := postgres.NewUserRepo(pool)
	sessionRepo := postgres.NewSessionRepo(pool)

	// Services
	authSvc := service.NewAuthService(
		userRepo,
		sessionRepo,
		cfg.JWTSecret,
		cfg.JWTAccessTTL,
		cfg.JWTRefreshTTL,
	)

	// Handlers
	authMiddleware := handler.NewAuthMiddleware(authSvc)
	authHandler    := handler.NewAuthHandler(authSvc)

	r := handler.NewRouter(authMiddleware, authHandler)

	slog.Info("starting server", "addr", cfg.ListenAddr)
	if err := r.Run(cfg.ListenAddr); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
