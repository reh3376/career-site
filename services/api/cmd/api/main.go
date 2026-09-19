// Command api runs the career-site HTTP API.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/reh3376/career-site/services/api/internal/config"
	"github.com/reh3376/career-site/services/api/internal/server"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		log.Error("config load failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	srv := server.New(cfg, log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case err := <-errCh:
		if err != nil {
			log.Error("server exited with error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}

	if err := srv.Shutdown(context.Background()); err != nil {
		log.Error("shutdown failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	log.Info("shutdown complete")
}
