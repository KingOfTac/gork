package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/apparentlymart/go-userdirs/userdirs"
	"github.com/kingoftac/gork/internal/db"
	"github.com/kingoftac/gork/internal/scheduler"
	"github.com/kingoftac/gork/internal/version"
)

var (
	dbName = "gork.db"
	dirs   = userdirs.ForApp("gork", "com.github.kingoftac.gork", "com.github.kingoftac.gork")
	dbPath = dirs.DataHome() + string(os.PathSeparator) + dbName
)

func main() {
	// Use a writer that flushes immediately for real-time log output
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Disable output buffering for real-time logs
	os.Stdout.Sync()

	db, err := db.NewDB(dbPath)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	sched := scheduler.NewScheduler(db)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		slog.Info("Received shutdown signal, exiting...")
		cancel()
	}()

	slog.Info("Starting gork daemon...", "version", version.Version)
	sched.Start(ctx)
	slog.Info("Gork daemon stopped")
}
