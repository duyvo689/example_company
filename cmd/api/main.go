package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example-company/internal/config"
	"example-company/internal/database"
	"example-company/internal/handler"
	"example-company/internal/middleware"
	"example-company/internal/outbox"
	platformjwt "example-company/internal/platform/jwt"
	"example-company/internal/router"
	"example-company/internal/service"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel)}))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(ctx, cfg.DatabaseURL, cfg.Environment == "development")
	if err != nil {
		log.Error("database_connection_failed", "error", err)
		os.Exit(1)
	}
	if cfg.AutoMigrate {
		if err := database.AutoMigrate(db); err != nil {
			log.Error("auto_migrate_failed", "error", err)
			os.Exit(1)
		}
	}
	if err := database.Seed(db); err != nil {
		log.Error("seed_failed", "error", err)
		os.Exit(1)
	}

	tokens := platformjwt.New(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	services := service.New(db, tokens)
	handlers := handler.New(services)
	engine := router.New(db, handlers, tokens,
		middleware.RequestID(), middleware.CORS(cfg.CORSAllowedOrigins), middleware.Logger(log), middleware.Recover(log), middleware.Audit(db, log),
	)
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: engine, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}

	dispatcher := outbox.New(db, cfg.IntegrationWebhookURL, cfg.OutboxInterval, log)
	go dispatcher.Run(ctx)
	go func() {
		log.Info("http_server_started", "addr", cfg.HTTPAddr, "environment", cfg.Environment)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http_server_failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("http_shutdown_failed", "error", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	log.Info("application_stopped")
}

func parseLevel(value string) slog.Level {
	switch value {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
