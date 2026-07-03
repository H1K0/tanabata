package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"tanabata/backend/internal/config"
	"tanabata/backend/internal/db/postgres"
	"tanabata/backend/internal/handler"
	"tanabata/backend/internal/service"
	"tanabata/backend/internal/storage"
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

	// Storage
	diskStorage, err := storage.NewDiskStorage(
		cfg.FilesPath,
		cfg.ThumbsCachePath,
		cfg.ThumbWidth, cfg.ThumbHeight,
		cfg.PreviewWidth, cfg.PreviewHeight,
		cfg.ThumbMaxPixels, cfg.ThumbConcurrency,
	)
	if err != nil {
		slog.Error("failed to initialise storage", "err", err)
		os.Exit(1)
	}

	// Repositories
	userRepo := postgres.NewUserRepo(pool)
	sessionRepo := postgres.NewSessionRepo(pool)
	fileRepo := postgres.NewFileRepo(pool)
	mimeRepo := postgres.NewMimeRepo(pool)
	aclRepo := postgres.NewACLRepo(pool)
	auditRepo := postgres.NewAuditRepo(pool)
	tagRepo := postgres.NewTagRepo(pool)
	tagRuleRepo := postgres.NewTagRuleRepo(pool)
	categoryRepo := postgres.NewCategoryRepo(pool)
	poolRepo := postgres.NewPoolRepo(pool)
	duplicatePairRepo := postgres.NewDuplicatePairRepo(pool)
	dismissalRepo := postgres.NewDismissalRepo(pool)
	transactor := postgres.NewTransactor(pool)

	// Services
	authSvc := service.NewAuthService(
		userRepo,
		sessionRepo,
		cfg.JWTSecret,
		cfg.JWTAccessTTL,
		cfg.JWTRefreshTTL,
		cfg.ContentTokenTTL,
	)
	aclSvc := service.NewACLService(aclRepo, fileRepo, tagRepo, categoryRepo, poolRepo, transactor)
	auditSvc := service.NewAuditService(auditRepo)
	tagSvc := service.NewTagService(tagRepo, tagRuleRepo, aclSvc, auditSvc, transactor)
	categorySvc := service.NewCategoryService(categoryRepo, tagRepo, aclSvc, auditSvc)
	poolSvc := service.NewPoolService(poolRepo, aclSvc, auditSvc)
	duplicateSvc := service.NewDuplicateService(
		fileRepo, duplicatePairRepo, dismissalRepo, aclSvc, auditSvc, transactor, cfg.DuplicateHashThreshold,
	)
	fileSvc := service.NewFileService(
		fileRepo,
		mimeRepo,
		diskStorage,
		aclSvc,
		auditSvc,
		tagSvc,
		transactor,
		cfg.ImportPath,
	)
	userSvc := service.NewUserService(userRepo, sessionRepo, auditSvc)

	// Bootstrap the initial administrator (idempotent).
	if err := userSvc.EnsureAdmin(context.Background(), cfg.AdminUsername, cfg.AdminPassword); err != nil {
		slog.Error("failed to bootstrap admin user", "err", err)
		os.Exit(1)
	}

	// Handlers
	authMiddleware := handler.NewAuthMiddleware(authSvc)
	authHandler := handler.NewAuthHandler(authSvc)
	fileHandler := handler.NewFileHandler(fileSvc, tagSvc, authSvc, cfg.MaxUploadBytes)
	duplicateHandler := handler.NewDuplicateHandler(duplicateSvc)
	tagHandler := handler.NewTagHandler(tagSvc, fileSvc)
	categoryHandler := handler.NewCategoryHandler(categorySvc)
	poolHandler := handler.NewPoolHandler(poolSvc)
	userHandler := handler.NewUserHandler(userSvc)
	aclHandler := handler.NewACLHandler(aclSvc)
	auditHandler := handler.NewAuditHandler(auditSvc)

	r, err := handler.NewRouter(
		authMiddleware, authHandler,
		fileHandler, duplicateHandler, tagHandler, categoryHandler, poolHandler,
		userHandler, aclHandler, auditHandler,
		cfg.StaticDir,
		cfg.TrustedProxies,
	)
	if err != nil {
		slog.Error("building router", "err", err)
		os.Exit(1)
	}

	// ReadHeaderTimeout bounds slow-header (Slowloris) attacks; body read/write
	// are left unbounded so large file uploads and downloads can stream.
	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Trigger a graceful shutdown on SIGINT/SIGTERM (the latter is what Docker
	// sends when the container is stopped or recreated on deploy).
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("starting server", "addr", cfg.ListenAddr)
		// ListenAndServe returns ErrServerClosed after a graceful Shutdown; that
		// is the expected exit, not a failure.
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	// Restore default signal handling so a second Ctrl+C / SIGTERM force-quits
	// instead of waiting on the drain.
	stop()
	slog.Info("shutting down", "timeout", cfg.ShutdownTimeout)

	// Stop accepting new connections and let in-flight requests finish, up to the
	// timeout. Docker's stop grace period reads the same SHUTDOWN_TIMEOUT, so it
	// won't SIGKILL before this returns.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	slog.Info("shutdown complete")
}
