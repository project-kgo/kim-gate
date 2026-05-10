package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/project-kgo/kim-gate/internal/config"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		logger.Error("failed to load config", slog.Any("error", err))
		os.Exit(1)
	}

	application, err := Initialize(cfg, logger)
	if err != nil {
		logger.Error("failed to initialize application", slog.Any("error", err))
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := application.Start(); err != nil {
		logger.Error("failed to start application", slog.Any("error", err))
		os.Exit(1)
	}

	var runErr error
	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case runErr = <-application.Done():
		if runErr != nil {
			logger.Error("application stopped with error", slog.Any("error", runErr))
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := application.Shutdown(shutdownCtx); err != nil {
		logger.Error("failed to shutdown application", slog.Any("error", err))
		os.Exit(1)
	}
	if runErr != nil {
		os.Exit(1)
	}

	time.Sleep(50 * time.Millisecond)
}
