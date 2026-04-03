package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"tanabata/backend/internal/config"
	"tanabata/backend/migrations"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		slog.Error("database ping failed", "err", err)
		os.Exit(1)
	}
	slog.Info("database connected")

	// Run migrations using the embedded FS.
	// stdlib.OpenDBFromPool wraps the pool in a *sql.DB without closing
	// the pool when the sql.DB is closed.
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

	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	slog.Info("starting server", "addr", cfg.ListenAddr)
	if err := r.Run(cfg.ListenAddr); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
