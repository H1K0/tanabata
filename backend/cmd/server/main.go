package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
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
	)
	if err != nil {
		slog.Error("failed to initialise storage", "err", err)
		os.Exit(1)
	}

	// Repositories
	userRepo     := postgres.NewUserRepo(pool)
	sessionRepo  := postgres.NewSessionRepo(pool)
	fileRepo     := postgres.NewFileRepo(pool)
	mimeRepo     := postgres.NewMimeRepo(pool)
	aclRepo      := postgres.NewACLRepo(pool)
	auditRepo    := postgres.NewAuditRepo(pool)
	tagRepo      := postgres.NewTagRepo(pool)
	tagRuleRepo  := postgres.NewTagRuleRepo(pool)
	categoryRepo := postgres.NewCategoryRepo(pool)
	poolRepo     := postgres.NewPoolRepo(pool)
	transactor   := postgres.NewTransactor(pool)

	// Services
	authSvc := service.NewAuthService(
		userRepo,
		sessionRepo,
		cfg.JWTSecret,
		cfg.JWTAccessTTL,
		cfg.JWTRefreshTTL,
	)
	aclSvc      := service.NewACLService(aclRepo, fileRepo, tagRepo, categoryRepo, poolRepo, transactor)
	auditSvc    := service.NewAuditService(auditRepo)
	tagSvc      := service.NewTagService(tagRepo, tagRuleRepo, aclSvc, auditSvc, transactor)
	categorySvc := service.NewCategoryService(categoryRepo, tagRepo, aclSvc, auditSvc)
	poolSvc     := service.NewPoolService(poolRepo, aclSvc, auditSvc)
	fileSvc     := service.NewFileService(
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
	authMiddleware  := handler.NewAuthMiddleware(authSvc)
	authHandler     := handler.NewAuthHandler(authSvc)
	fileHandler     := handler.NewFileHandler(fileSvc, tagSvc, cfg.MaxUploadBytes)
	tagHandler      := handler.NewTagHandler(tagSvc, fileSvc)
	categoryHandler := handler.NewCategoryHandler(categorySvc)
	poolHandler     := handler.NewPoolHandler(poolSvc)
	userHandler     := handler.NewUserHandler(userSvc)
	aclHandler      := handler.NewACLHandler(aclSvc)
	auditHandler    := handler.NewAuditHandler(auditSvc)

	r := handler.NewRouter(
		authMiddleware, authHandler,
		fileHandler, tagHandler, categoryHandler, poolHandler,
		userHandler, aclHandler, auditHandler,
		cfg.StaticDir,
	)

	// ReadHeaderTimeout bounds slow-header (Slowloris) attacks; body read/write
	// are left unbounded so large file uploads and downloads can stream.
	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	slog.Info("starting server", "addr", cfg.ListenAddr)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
