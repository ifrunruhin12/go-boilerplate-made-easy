package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"toolbox/internal/cache"
	"toolbox/internal/config"
	"toolbox/internal/database"
	"toolbox/internal/server"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--healthcheck" {
		if err := healthcheck(); err != nil {
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	deps := server.Dependencies{Logger: logger}
	startup, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	db, err := database.Open(startup, cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	deps.DB = db
	redisClient, err := cache.Open(startup, cfg.RedisURL)
	if err != nil {
		return err
	}
	defer redisClient.Close()
	deps.Cache = redisClient
	srv := server.New(cfg, deps)
	errs := make(chan error, 1)
	go func() { errs <- srv.Listen(cfg.Address) }()
	logger.Info("server listening", "address", cfg.Address)
	select {
	case err := <-errs:
		if err != nil {
			return fmt.Errorf("listen: %w", err)
		}
		return nil
	case <-ctx.Done():
	}
	shutdown, cancelShutdown := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancelShutdown()
	// Drain HTTP before deferred database/cache closure.
	if err := srv.Shutdown(shutdown); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	logger.Info("server drained")
	return nil
}

func healthcheck() error {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	client := http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://" + net.JoinHostPort("127.0.0.1", port) + "/ready")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return fmt.Errorf("not ready")
	}
	return nil
}
