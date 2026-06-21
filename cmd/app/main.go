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

	"file-storage-server/internal/access"
	"file-storage-server/internal/admin"
	"file-storage-server/internal/api"
	"file-storage-server/internal/audit"
	"file-storage-server/internal/auth"
	"file-storage-server/internal/config"
	"file-storage-server/internal/file"
	"file-storage-server/internal/filesystem"
	"file-storage-server/internal/folder"
	"file-storage-server/internal/jobs"
	"file-storage-server/internal/metrics"
	"file-storage-server/internal/middleware"
	"file-storage-server/internal/postgres"
	"file-storage-server/internal/profile"
	"file-storage-server/internal/share"
	"file-storage-server/internal/storage"
	"file-storage-server/internal/user"
	"file-storage-server/internal/web"

	"github.com/go-chi/chi/v5"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	if err := config.LoadDotEnv(".env"); err != nil {
		logger.Error("load .env", "error", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.OpenPool(ctx, cfg.Postgres)
	if err != nil {
		logger.Error("open postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	userRepository := user.NewPostgresRepository(pool)
	storageRepository := storage.NewPostgresRepository(pool)
	fileRepository := file.NewPostgresRepository(pool)
	folderRepository := folder.NewPostgresRepository(pool)
	accessRepository := access.NewPostgresRepository(pool)
	adminRepository := admin.NewRepository(pool)
	adminService := admin.NewService(adminRepository)
	profileRepository := profile.NewRepository(pool)
	profileService := profile.NewService(profileRepository)
	shareRepository := share.NewPostgresRepository(pool)
	refreshTokenRepository := auth.NewPostgresRefreshTokenRepository(pool)
	auditRepository := audit.NewPostgresRepository(pool)
	fileStorage := filesystem.NewLocalStorage(cfg.Files.RootDir)
	unitOfWork := postgres.NewUnitOfWork(pool)

	auditService := audit.NewService(auditRepository, logger)
	authService := auth.NewService(userRepository, refreshTokenRepository, cfg.Auth)
	permissionService := access.NewPermissionService(userRepository, storageRepository, fileRepository, accessRepository)
	storageService := storage.NewService(storageRepository, permissionService)
	storageAccessService := access.NewStorageAccessService(accessRepository, userRepository, folderRepository, permissionService)
	folderService := folder.NewService(folderRepository, storageRepository, unitOfWork, permissionService)
	fileService := file.NewService(fileRepository, storageRepository, folderRepository, fileStorage, unitOfWork, permissionService)
	shareService := share.NewService(shareRepository, fileRepository, fileService, fileStorage, permissionService)
	metricsRegistry := metrics.NewRegistry()

	authHandler := api.NewAuthHandler(authService, auditService)
	storageHandler := api.NewStorageHandler(storageService, auditService)
	accessHandler := api.NewAccessHandler(storageAccessService, auditService)
	fileHandler := api.NewFileHandler(fileService, auditService)
	shareHandler := api.NewShareHandler(shareService, auditService)
	webHandler, err := web.NewHandler(authService, storageService, fileService, folderService, storageAccessService, adminService, metricsRegistry, profileService, shareService, auditService, cfg.Files.RootDir, "templates/*.html")
	if err != nil {
		logger.Error("load templates", "error", err)
		os.Exit(1)
	}

	jobs.NewCleanupJob(pool, cfg.Files.RootDir, cfg.Jobs, logger).Start(ctx)
	authRateLimit := middleware.NewRateLimiter(20, time.Minute).Middleware
	publicRateLimit := middleware.NewRateLimiter(120, time.Minute).Middleware

	router := chi.NewRouter()
	router.Use(middleware.Metrics(metricsRegistry))
	router.Use(middleware.RequestLogger(logger))
	router.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	router.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "postgres is not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	router.Handle("/metrics/raw", metricsRegistry.Handler())
	api.RegisterRoutes(router, api.RouterDeps{
		AuthMiddleware:  middleware.Auth(authService),
		AuthRateLimit:   authRateLimit,
		PublicRateLimit: publicRateLimit,
		Auth:            authHandler,
		Storages:        storageHandler,
		Accesses:        accessHandler,
		Files:           fileHandler,
		Shares:          shareHandler,
	})
	webHandler.Routes(router, middleware.WebAuth(authService, "/login"), authRateLimit)

	server := &http.Server{
		Addr:         cfg.HTTP.Addr,
		Handler:      router,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	}

	go func() {
		logger.Info("server started", "addr", cfg.HTTP.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("serve http", "error", err)
			stop()
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown server", "error", err)
	}
}
